package core

import (
	"context"
	"fmt"
	"os"

	"github.com/immesys/go-ceph/rados"
	"github.com/coreos/etcd/clientv3"
	"github.com/urfave/cli"
)

func Clear(c *cli.Context) error {
	if c.Bool("skip-etcd") && c.Bool("skip-ceph") {
		fmt.Printf("you can't skip both ceph and etcd\n")
		os.Exit(1)
	}
	restorepools := []string{}
	for _, arg := range c.StringSlice("include-pool") {
		restorepools = append(restorepools, arg)
	}
	skipceph := true
	skipetcd := true
	if !c.Bool("skip-etcd") {
		connectEtcd(c)
		skipetcd = false
	}
	if !c.Bool("skip-ceph") {
		connectCeph(c)
		checkPoolsExist(nil, restorepools)
		skipceph = false
	}
	if !skipceph {
		for _, pool := range restorepools {
			ioctx, err := gcephconn.OpenIOContext(pool)
			ChkErr(err)
			ioctx2, err := gcephconn.OpenIOContext(pool)
			ChkErr(err)
			ioctx.SetNamespace(rados.RadosAllNamespaces)
			iter, err := ioctx.Iter()
			ChkErr(err)
			for iter.Next() {
				ChkErr(iter.Err())
				ioctx2.SetNamespace(iter.Namespace())
				err := ioctx2.Delete(iter.Value())
				fmt.Printf("deleted ceph object %q/%q\n", pool, iter.Value())
				ChkErr(err)
			}
			ChkErr(iter.Err())
			ioctx.Destroy()
			ioctx2.Destroy()
		}
	}
	if !skipetcd {
		_, err := getcd.Delete(context.Background(), "", clientv3.WithPrefix())
		ChkErr(err)
		fmt.Printf("deleted etcd keys\n")
	}
	fmt.Printf("done\n")
	return nil
}
