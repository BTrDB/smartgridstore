package main

import (
	"fmt"
	"os"

	btrdbcli "github.com/BTrDB/btrdb-server/cliplugin"
	"github.com/BTrDB/mr-plotter/accounts"
	etcd "github.com/coreos/etcd/clientv3"
	"github.com/BTrDB/smartgridstore/acl"
	"github.com/BTrDB/smartgridstore/admincli"
	mfst "github.com/BTrDB/smartgridstore/tools/manifest/cli"
	mrplotterconf "github.com/samkumar/mr-plotter-conf/cli"
)

func GetRootModule(c *etcd.Client, user string) admincli.CLIModule {

	etcdKeyPrefix := os.Getenv("ETCD_KEY_PREFIX")
	if len(etcdKeyPrefix) != 0 {
		accounts.SetEtcdKeyPrefix(etcdKeyPrefix)
		fmt.Printf("Using etcd prefix '%s'\n", etcdKeyPrefix)
	}

	mrp := mrplotterconf.NewMrPlotterCLIModule(c)
	acl := acl.NewACLModule(c, user)
	manifest := mfst.NewManifestCLIModule(c)
	btrdb := btrdbcli.NewBTrDBCLI(c)
	r := &admincli.GenericCLIModule{
		MChildren: []admincli.CLIModule{
			mrp,
			acl,
			manifest,
			btrdb,
		},
	}
	return r
}
