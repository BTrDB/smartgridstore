package core

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/immesys/go-ceph/rados"
	"github.com/urfave/cli"
)

func Restore(c *cli.Context) error {
	if c.Bool("skip-etcd") && c.Bool("skip-ceph") {
		fmt.Printf("you can't skip both ceph and etcd\n")
		os.Exit(1)
	}
	gpoolsmap = make(map[string]string)
	restorepools := []string{}
	for _, arg := range c.StringSlice("restore-pool") {
		parts := strings.SplitN(arg, ":", -1)
		if len(parts) != 2 {
			fmt.Printf("invalid restore-pool argument %q", arg)
			os.Exit(1)
		}
		gpoolsmap[parts[0]] = parts[1]
		restorepools = append(restorepools, parts[1])
	}
	getPassphrase()
	createFrameWriter()
	createReadSharder(c)
	skipceph := true
	skipetcd := true
	if !c.Bool("skip-etcd") {
		fmt.Printf("connecting etcd\n")
		connectEtcd(c)
		skipetcd = false
	}
	if !c.Bool("skip-ceph") {
		fmt.Printf("connecting ceph\n")
		connectCeph(c)
		checkPoolsExist(nil, restorepools)
		skipceph = false
	}

	poolconns := make(map[string]*rados.IOContext)
	cephcount := 0
	skipcephcount := 0
	etcdcount := 0
	container, more := gsharder.Read()
	for more {
		co, ok := container.(*CephObject)
		if ok {
			if skipceph {
				container, more = gsharder.Read()
				continue
			}
			restorepool, ok := gpoolsmap[co.Pool]
			if !ok {
				skipcephcount++
				container, more = gsharder.Read()
				continue
			}
			cephcount++
			if cephcount%200 == 0 {
				fmt.Printf("Restored %d objects, skipped %d objects\n", cephcount, skipcephcount)
			}
			ioctx, ok := poolconns[restorepool]
			if !ok {
				var err error
				ioctx, err = gcephconn.OpenIOContext(restorepool)
				ChkErr(err)
				poolconns[restorepool] = ioctx
			}
			ioctx.SetNamespace(co.Namespace)
			if len(co.Content) > 0 {
				err := ioctx.WriteFull(co.Name, co.Content)
				ChkErr(err)
			}
			for _, kv := range co.XATTRData {
				err := ioctx.SetXattr(co.Name, kv.Key, kv.Value)
				ChkErr(err)
			}
			for _, kv := range co.OMAPData {
				err := ioctx.SetOmap(co.Name, map[string][]byte{kv.Key: kv.Value})
				ChkErr(err)
			}
			//fmt.Printf("CO P=%s NS=%q OMAP=%d XATTR=%d OID=%s\n", co.Pool, co.Namespace, len(co.OMAPData), len(co.XATTRData), co.Name)
		} else {
			et, ok := container.(*EtcdRecords)
			if ok {
				if skipetcd {
					container, more = gsharder.Read()
					continue
				}
				etcdcount += len(et.KVz)
				for _, kv := range et.KVz {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					_, err := getcd.Put(ctx, kv.Key, string(kv.Value))
					ChkErr(err)
					cancel()
				}
			} else {
				fmt.Printf("unknown container!\n")
			}
		}
		container, more = gsharder.Read()
	}
	fmt.Printf("Restore complete: restored %d ceph objects, skipped %d ceph objects and restored  %d etcd records\n", cephcount, skipcephcount, etcdcount)
	return nil
}
