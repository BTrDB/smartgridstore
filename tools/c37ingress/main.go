package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	etcd "github.com/coreos/etcd/clientv3"
	"github.com/immesys/smartgridstore/tools"
	"github.com/immesys/smartgridstore/tools/manifest"
	btrdb "gopkg.in/btrdb.v4"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "-version" {
		fmt.Printf("%d.%d.%d\n", tools.VersionMajor, tools.VersionMinor, tools.VersionPatch)
		os.Exit(0)
	}
	fmt.Printf("Booting c37 ingress version %d.%d.%d\n", tools.VersionMajor, tools.VersionMinor, tools.VersionPatch)

	manifest.SetEtcdKeyPrefix("")

	var etcdEndpoint string = os.Getenv("ETCD_ENDPOINT")
	if len(etcdEndpoint) == 0 {
		etcdEndpoint = "localhost:2379"
		log.Printf("ETCD_ENDPOINT is not set; using %s", etcdEndpoint)
	}

	var etcdConfig etcd.Config = etcd.Config{Endpoints: []string{etcdEndpoint}}

	log.Println("Connecting to etcd...")
	etcdConn, err := etcd.New(etcdConfig)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	defer etcdConn.Close()
	log.Println("Connecting to BTrDB...")
	btrdbconn, err := btrdb.Connect(context.Background(), btrdb.EndpointsFromEnv()...)
	if err != nil {
		fmt.Printf("Error connecting to the BTrDB: %v\n", err)
		return
	}

	defer func() {
		err := btrdbconn.Disconnect()
		if err == nil {
			fmt.Println("Finished closing connection")
		} else {
			fmt.Printf("Could not close connection: %v\n", err)
		}
	}()
	log.Println("Connected")
	devs, err := manifest.RetrieveMultipleManifestDevices(context.Background(), etcdConn, "c37-118.pdc.")
	if err != nil {
		panic(err)
	}

	for _, dev := range devs {
		identifier := dev.Descriptor
		connstring := strings.SplitN(identifier, ".", 3)[2]
		fmt.Printf("identified pdc %q\n", connstring)
		//connstring looks like PREFIX@IDCODE@HOST:PORT
		parts := strings.SplitN(connstring, "@", 3)
		if len(parts) != 3 {
			fmt.Printf("Invalid connection string %q\n", connstring)
			continue
		}
		prefix := parts[0]
		idcode, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			fmt.Printf("Invalid connection string %q\n", connstring)
			continue
		}
		go process(btrdbconn, int(idcode), prefix, parts[2])
	}
	for {
		time.Sleep(5 * time.Second)
	}

}

func process(db *btrdb.BTrDB, idcode int, prefix string, target string) {
	inserter := NewInserter(db, prefix)

	p := CreatePMU(target, uint16(idcode))

	for {
		then := time.Now()
		dat, drained := p.GetBatch()
		inserter.ProcessBatch(dat)

		if drained {
			delta := then.Add(30 * time.Second).Sub(time.Now())
			if delta > 0 {
				time.Sleep(delta)
			}
		}
	}
}
