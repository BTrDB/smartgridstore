package main

import (
	"context"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/BTrDB/mr-plotter/keys"
	etcd "github.com/coreos/etcd/clientv3"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Printf("Usage: %s cert.pem key.pem\n", os.Args[0])
		return
	}

	certname := os.Args[1]
	keyname := os.Args[2]

	var etcdEndpoint = os.Getenv("ETCD_ENDPOINT")
	if len(etcdEndpoint) == 0 {
		etcdEndpoint = "localhost:2379"
		log.Printf("ETCD_ENDPOINT is not set; using %s", etcdEndpoint)
	}
	var etcdConfig = etcd.Config{Endpoints: []string{etcdEndpoint}}
	log.Println("Connecting to etcd...")
	etcdConn, err := etcd.New(etcdConfig)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	defer etcdConn.Close()

	source, err := keys.GetCertificateSource(context.Background(), etcdConn)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	if source == "autocert" {
		hn, err := etcdConn.Get(context.Background(), "mrplotter/keys/hostname")
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		if len(hn.Kvs) == 0 {
			log.Fatalf("Error: %v", err)
		}
		hostname := string(hn.Kvs[0].Value)
		ci, err := etcdConn.Get(context.Background(), "mrplotter/keys/autocert_cache/"+hostname)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		if len(ci.Kvs) == 0 {
			log.Fatalf("Error: %v", err)
		}

		priv, rest := pem.Decode(ci.Kvs[0].Value)

		err = ioutil.WriteFile(keyname, pem.EncodeToMemory(priv), 0600)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		err = ioutil.WriteFile(certname, rest, 0600)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("DONE")
		os.Exit(0)
		//keys.GetAutocertCache(ctx, etcdClient, key)
	}
	if source == "hardcoded" {
		h, err := keys.RetrieveHardcodedTLSCertificate(context.Background(), etcdConn)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		err = ioutil.WriteFile(keyname, h.Key, 0600)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		err = ioutil.WriteFile(certname, h.Cert, 0600)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
	}

	log.Println("DONE")
}
