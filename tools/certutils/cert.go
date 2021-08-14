// Copyright (c) 2021 Michael Andersen
// Copyright (c) 2021 Regents of the University Of California
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

//This package allows tools to use the external TLS certificate
package certutils

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	acli "github.com/BTrDB/smartgridstore/tools/apifrontend/cli"

	etcd "github.com/coreos/etcd/clientv3"
)

func GetAPIConfig(ec *etcd.Client) (*tls.Config, error) {
	src, err := acli.GetAPIFrontendCertSrc(ec)
	if err != nil {
		return nil, err
	}
	switch src {
	case "hardcoded":
		cert, key, err := acli.GetAPIFrontendHardcoded(ec)
		if err != nil {
			return nil, fmt.Errorf("could not load hardcoded certificate: %v\n", err)
		}
		if len(cert) == 0 || len(key) == 0 {
			return nil, fmt.Errorf("CRITICAL: certsrc set to hardcoded but no certificate set\n")
		}
		var tlsCertificate tls.Certificate
		tlsCertificate, err = tls.X509KeyPair(cert, key)
		cfg := &tls.Config{
			GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
				return &tlsCertificate, nil
			},
		}
		return cfg, nil
	case "autocert":
		cfg, err := MRPlottersAutocertTLSConfig(ec)
		if err != nil {
			return nil, err
		}
		if cfg == nil {
			return nil, nil
		}
		return cfg, nil
	case "disabled":
		return nil, nil
	}
	return nil, nil
}

// This was taken from https://golang.org/src/crypto/tls/generate_cert.go.
// All credit to the Go Authors.
func pemBlockForKey(priv interface{}) (*pem.Block, error) {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}, nil
	case *ecdsa.PrivateKey:
		b, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			return nil, err
		}
		return &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}, nil
	default:
		return nil, nil
	}
}

// SerializeCertificate serializes a TLS certificate into the cert and key PEM
// files.
func SerializeCertificate(certificate *tls.Certificate) (*pem.Block, *pem.Block, error) {
	certpem := &pem.Block{Type: "CERTIFICATE", Bytes: certificate.Certificate[0]}
	keypem, err := pemBlockForKey(certificate.PrivateKey)
	return certpem, keypem, err
}

// SelfSignedCertificate generates a self-signed certificate.
// Much of this is from https://golang.org/src/crypto/tls/generate_cert.go.
// All credit to the Go Authors.
func SelfSignedCertificate(dnsNames []string) (*pem.Block, *pem.Block, error) {
	privkey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, err
	}

	now := time.Now()

	template := &x509.Certificate{
		IsCA:         true,
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:         "default.autocert.smartgrid.store",
			Country:            []string{"United States of America"},
			Organization:       []string{"University of California, Berkeley"},
			OrganizationalUnit: []string{"Software Defined Buildings"},
			Locality:           []string{"Berkeley"},
			Province:           []string{"California"},
			StreetAddress:      []string{"410 Soda Hall"},
		},
		NotBefore: now.Add(-time.Hour),
		NotAfter:  now.Add(time.Hour * 24 * 365),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,

		DNSNames: dnsNames,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &privkey.PublicKey, privkey)
	if err != nil {
		return nil, nil, err
	}

	cert := &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}
	key, err := pemBlockForKey(privkey)
	if err != nil {
		return nil, nil, err
	}
	return cert, key, nil
}

func MRPlottersAutocertTLSConfig(c *etcd.Client) (*tls.Config, error) {
	hn, err := c.Get(context.Background(), "mrplotter/keys/hostname")
	if err != nil {
		return nil, err
	}
	if len(hn.Kvs) == 0 {
		return nil, nil
	}
	hostname := string(hn.Kvs[0].Value)
	ci, err := c.Get(context.Background(), "mrplotter/keys/autocert_cache/"+hostname)
	if err != nil {
		return nil, err
	}
	if len(ci.Kvs) == 0 {
		return nil, nil
	}
	cert, err := certificateFromAutocertCache(hostname, ci.Kvs[0].Value)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			return cert, nil
		},
	}, nil
}

// cacheGet always returns a valid certificate, or an error otherwise.
// If a cached certficate exists but is not valid, ErrCacheMiss is returned.
func certificateFromAutocertCache(domain string, data []byte) (*tls.Certificate, error) {

	// private
	priv, pub := pem.Decode(data)
	if priv == nil || !strings.Contains(priv.Type, "PRIVATE") {
		return nil, fmt.Errorf("certificate is corrupt")
	}
	privKey, err := parsePrivateKey(priv.Bytes)
	if err != nil {
		return nil, err
	}

	// public
	var pubDER [][]byte
	for len(pub) > 0 {
		var b *pem.Block
		b, pub = pem.Decode(pub)
		if b == nil {
			break
		}
		pubDER = append(pubDER, b.Bytes)
	}
	if len(pub) > 0 {
		// Leftover content not consumed by pem.Decode. Corrupt. Ignore.
		return nil, fmt.Errorf("certificate is corrupt")
	}

	// verify and create TLS cert
	leaf, err := validCert(domain, pubDER, privKey)
	if err != nil {
		return nil, fmt.Errorf("certificate is invalid")
	}
	tlscert := &tls.Certificate{
		Certificate: pubDER,
		PrivateKey:  privKey,
		Leaf:        leaf,
	}
	return tlscert, nil
}

// validCert parses a cert chain provided as der argument and verifies the leaf, der[0],
// corresponds to the private key, as well as the domain match and expiration dates.
// It doesn't do any revocation checking.
//
// The returned value is the verified leaf cert.
func validCert(domain string, der [][]byte, key crypto.Signer) (leaf *x509.Certificate, err error) {
	// parse public part(s)
	var n int
	for _, b := range der {
		n += len(b)
	}
	pub := make([]byte, n)
	n = 0
	for _, b := range der {
		n += copy(pub[n:], b)
	}
	x509Cert, err := x509.ParseCertificates(pub)
	if len(x509Cert) == 0 {
		return nil, errors.New("acme/autocert: no public key found")
	}
	// verify the leaf is not expired and matches the domain name
	leaf = x509Cert[0]
	now := time.Now()
	if now.Before(leaf.NotBefore) {
		return nil, errors.New("acme/autocert: certificate is not valid yet")
	}
	if now.After(leaf.NotAfter) {
		return nil, errors.New("acme/autocert: expired certificate")
	}
	if err := leaf.VerifyHostname(domain); err != nil {
		return nil, err
	}
	// ensure the leaf corresponds to the private key
	switch pub := leaf.PublicKey.(type) {
	case *rsa.PublicKey:
		prv, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("acme/autocert: private key type does not match public key type")
		}
		if pub.N.Cmp(prv.N) != 0 {
			return nil, errors.New("acme/autocert: private key does not match public key")
		}
	case *ecdsa.PublicKey:
		prv, ok := key.(*ecdsa.PrivateKey)
		if !ok {
			return nil, errors.New("acme/autocert: private key type does not match public key type")
		}
		if pub.X.Cmp(prv.X) != 0 || pub.Y.Cmp(prv.Y) != 0 {
			return nil, errors.New("acme/autocert: private key does not match public key")
		}
	default:
		return nil, errors.New("acme/autocert: unknown public key algorithm")
	}
	return leaf, nil
}

// Attempt to parse the given private key DER block. OpenSSL 0.9.8 generates
// PKCS#1 private keys by default, while OpenSSL 1.0.0 generates PKCS#8 keys.
// OpenSSL ecparam generates SEC1 EC private keys for ECDSA. We try all three.
//
// Inspired by parsePrivateKey in crypto/tls/tls.go.
func parsePrivateKey(der []byte) (crypto.Signer, error) {
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		switch key := key.(type) {
		case *rsa.PrivateKey:
			return key, nil
		case *ecdsa.PrivateKey:
			return key, nil
		default:
			return nil, errors.New("acme/autocert: unknown private key type in PKCS#8 wrapping")
		}
	}
	if key, err := x509.ParseECPrivateKey(der); err == nil {
		return key, nil
	}

	return nil, errors.New("acme/autocert: failed to parse private key")
}
