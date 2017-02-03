package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/SoftwareDefinedBuildings/mr-plotter/accounts"
	etcd "github.com/coreos/etcd/clientv3"
	"github.com/immesys/smartgridstore/admincli"
	mrplotterconf "github.com/samkumar/mr-plotter-conf/cli"
)

type rootCLIModule struct {
	children []admincli.CLIModule
}

var r *rootCLIModule

func InitRootModule(c *etcd.Client) {

	etcdKeyPrefix := os.Getenv("ETCD_KEY_PREFIX")
	if len(etcdKeyPrefix) != 0 {
		accounts.SetEtcdKeyPrefix(etcdKeyPrefix)
		fmt.Printf("Using etcd prefix '%s'\n", etcdKeyPrefix)
	}

	mrp := mrplotterconf.NewMrPlotterCLIModule(c)
	r = &rootCLIModule{children: []admincli.CLIModule{
		mrp,
	}}
}
func getRootCLIModule() admincli.CLIModule {
	return r
}

func (r *rootCLIModule) Children() []admincli.CLIModule {
	return r.children
}

func (r *rootCLIModule) Name() string {
	return ""
}

func (r *rootCLIModule) Hint() string {
	return ""
}

func (r *rootCLIModule) Usage() string {
	return "Smart Grid Store management console, root level.\nThe various subsystems can be listed with the 'ls' command"
}

func (r *rootCLIModule) Runnable() bool {
	return false
}

func (r *rootCLIModule) Run(ctx context.Context, output io.Writer, args ...string) bool {
	return false
}
