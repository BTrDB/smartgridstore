package main

import (
	"context"
	"fmt"
	"os"

	"github.com/BTrDB/mr-plotter/accounts"
	etcd "github.com/coreos/etcd/clientv3"
)

func main() {
	etcdEndpoint := os.Getenv("ETCD_ENDPOINT")
	if len(etcdEndpoint) == 0 {
		etcdEndpoint = "localhost:2379"
	}
	etcdKeyPrefix := os.Getenv("ETCD_KEY_PREFIX")
	if len(etcdKeyPrefix) != 0 {
		accounts.SetEtcdKeyPrefix(etcdKeyPrefix)
		fmt.Printf("Using Mr. Plotter configuration '%s'\n", etcdKeyPrefix)
	}
	etcdClient, err := etcd.New(etcd.Config{Endpoints: []string{etcdEndpoint}})
	if err != nil {
		fmt.Printf("Could not connect to etcd: %v\n", err)
		os.Exit(1)
	}

	tdef := &accounts.MrPlotterTagDef{
		Tag: "public",
		PathPrefix: map[string]struct{}{
			"": struct{}{},
		},
	}

	err = accounts.UpsertTagDef(context.Background(), etcdClient, tdef)
	if err != nil {
		fmt.Printf("Could not update tag 'public': %v\n", err)
		os.Exit(2)
	}

	fmt.Println("Success")
}
