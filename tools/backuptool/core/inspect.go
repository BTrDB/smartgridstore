// Copyright (c) 2021 Michael Andersen
// Copyright (c) 2021 Regents of the University Of California
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package core

import (
	"fmt"

	"github.com/urfave/cli"
)

func Inspect(c *cli.Context) error {
	getPassphrase()
	createFrameWriter()
	createReadSharder(c)
	cephcount := 0
	etcdcount := 0
	pools := make(map[string]bool)
	container, more := gsharder.Read()
	for more {
		co, ok := container.(*CephObject)
		if ok {
			cephcount++
			pools[co.Pool] = true
			//fmt.Printf("CO P=%s NS=%q OMAP=%d XATTR=%d OID=%s\n", co.Pool, co.Namespace, len(co.OMAPData), len(co.XATTRData), co.Name)
		} else {
			et, ok := container.(*EtcdRecords)
			if ok {
				etcdcount += len(et.KVz)
			} else {
				fmt.Printf("unknown container!\n")
			}
		}
		container, more = gsharder.Read()
	}
	fmt.Printf("archive contains %d ceph objects and %d etcd records\n", cephcount, etcdcount)
	fmt.Printf("the following %d pools are referenced:\n", len(pools))
	for pool, _ := range pools {
		fmt.Printf(" - %s\n", pool)
	}
	return nil
}
