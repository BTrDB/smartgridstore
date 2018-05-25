package main

import (
	"fmt"
	"os"

	btrdbcli "github.com/BTrDB/btrdb-server/cliplugin"
	"github.com/BTrDB/mr-plotter/accounts"
	"github.com/BTrDB/smartgridstore/acl"
	"github.com/BTrDB/smartgridstore/admincli"
	api "github.com/BTrDB/smartgridstore/tools/apifrontend/cli"
	mfst "github.com/BTrDB/smartgridstore/tools/manifest/cli"
	mrplotterconf "github.com/BTrDB/smartgridstore/tools/mr-plotter-conf/cli"
	etcd "github.com/coreos/etcd/clientv3"
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
	api := api.NewFrontendModule(c)
	r := &admincli.GenericCLIModule{
		MChildren: []admincli.CLIModule{
			mrp,
			acl,
			manifest,
			btrdb,
			api,
		},
	}
	return r
}
