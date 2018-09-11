package main

import (
	"fmt"
	"time"

	"github.com/ceph/go-ceph/rados"
	"github.com/davecgh/go-spew/spew"
)

func main() {
	pool := "btrdb"
	conn, err := rados.NewConn()
	fmt.Printf("NewConn exit: %v\n", err)
	if err != nil {
		panic(err)
	}
	err = conn.ReadConfigFile("/etc/ceph/ceph.conf")
	fmt.Printf("Read Config Exit: %v\n", err)
	if err != nil {
		panic(err)
	}
	go func() {
		for {
			fmt.Printf("alive\n")
			time.Sleep(5 * time.Second)
		}
	}()
	fmt.Printf("attempting connect\n")
	err = conn.Connect()
	fmt.Printf("connect returned:%v\n", err)
	if err != nil {
		panic(err)
	}
	fmt.Printf("opening IO context\n")
	coldh, err := conn.OpenIOContext(pool)
	fmt.Printf("io context returned: %v\n", err)
	if err != nil {
		panic(err)
	}
	_ = coldh
	fmt.Printf("attempting stat\n")
	statres, err := coldh.Stat("cold_allocator")
	fmt.Printf("stat returned: %v\n", err)
	spew.Dump(statres)
	fmt.Printf("done\n")
}
