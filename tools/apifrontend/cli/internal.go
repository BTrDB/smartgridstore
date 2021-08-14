// Copyright (c) 2021 Michael Andersen
// Copyright (c) 2021 Regents of the University Of California
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package cli

import (
	"context"
	"crypto/tls"

	etcd "github.com/coreos/etcd/clientv3"
	"golang.org/x/crypto/acme/autocert"
)

func GetAPIFrontendAutocertInfo(c *etcd.Client) (hostname string, email string, err error) {
	hostname, err = getkey(c, "host")
	if err != nil {
		return "", "", err
	}
	email, err = getkey(c, "email")
	if err != nil {
		return "", "", err
	}
	return
}

func GetAPIFrontendCertSrc(c *etcd.Client) (source string, err error) {
	rv, err := getkey(c, "certsrc")
	if rv == "" {
		rv = "disabled"
	}
	return rv, err
}

func GetAPIFrontendHardcoded(c *etcd.Client) (cert []byte, key []byte, err error) {
	privrv, err := getkey(c, "hardcoded_priv")
	if err != nil {
		return nil, nil, err
	}
	pubrv, err := getkey(c, "hardcoded_pub")
	if err != nil {
		return nil, nil, err
	}
	return []byte(pubrv), []byte(privrv), nil
}

func GetAPIFrontendAutocertCache(c *etcd.Client) autocert.Cache {
	return &EtcdCache{
		etcdClient: c,
	}
}

func GetAPIFrontendAutocertTLS(c *etcd.Client) (*tls.Config, error) {
	cache := GetAPIFrontendAutocertCache(c)
	host, email, err := GetAPIFrontendAutocertInfo(c)
	if err != nil {
		return nil, err
	}
	manager := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Cache:      cache,
		HostPolicy: autocert.HostWhitelist(host),
		Email:      email,
	}
	return &tls.Config{GetCertificate: manager.GetCertificate}, nil
}

type EtcdCache struct {
	etcdClient *etcd.Client
}

func (ec *EtcdCache) Get(ctx context.Context, key string) ([]byte, error) {
	strval, err := getkey(ec.etcdClient, "autocert")
	if strval == "" && err == nil {
		return nil, autocert.ErrCacheMiss
	}

	return []byte(strval), err
}

func (ec *EtcdCache) Put(ctx context.Context, key string, val []byte) error {
	strval := string(val)
	return setkey(ec.etcdClient, "autocert", strval)
}

func (ec *EtcdCache) Delete(ctx context.Context, key string) error {
	return setkey(ec.etcdClient, "autocert", "")
}
