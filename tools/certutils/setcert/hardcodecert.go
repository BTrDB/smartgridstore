package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/BTrDB/mr-plotter/keys"
	etcd "github.com/coreos/etcd/clientv3"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Printf("Usage: %s plotter/api cert.pem key.pem\n", os.Args[0])
		return
	}

	httpscert, err := ioutil.ReadFile(os.Args[2])
	if err != nil {
		log.Fatalf("Could not read HTTPS certificate file: %v", err)
	}
	httpskey, err := ioutil.ReadFile(os.Args[3])
	if err != nil {
		log.Fatalf("Could not read HTTPS certificate file: %v", err)
	}

	hardcoded := &keys.HardcodedTLSCertificate{
		Cert: httpscert,
		Key:  httpskey,
	}

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

	switch os.Args[1] {
	case "plotter":
		err := keys.UpsertHardcodedTLSCertificate(context.Background(), etcdConn, hardcoded)
		if err != nil {
			log.Fatalf("Could not update hardcoded TLS certificate: %v", err)
		}
	case "api":
		_, err = etcdConn.Put(context.Background(), "api/hardcoded_priv", string(httpskey))
		if err != nil {
			log.Fatalf("could not set API private key: %v", err)
		}
		_, err = etcdConn.Put(context.Background(), "api/hardcoded_pub", string(httpscert))
		if err != nil {
			log.Fatalf("could not set API public key: %v", err)
		}
	default:
		fmt.Printf("second argument must be 'plotter' or 'api'\n")
		os.Exit(1)
	}
	log.Println("DONE")
}
