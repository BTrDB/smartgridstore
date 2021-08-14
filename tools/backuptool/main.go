// Copyright (c) 2021 Michael Andersen
// Copyright (c) 2021 Regents of the University Of California
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package main

import (
	"fmt"
	"os"

	"github.com/BTrDB/smartgridstore/tools"
	"github.com/BTrDB/smartgridstore/tools/backuptool/core"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "btrdb-backup"
	app.Usage = "create a backup of a BTrDB cluster"
	app.Version = fmt.Sprintf("%d.%d.%d", tools.VersionMajor, tools.VersionMinor, tools.VersionPatch)
	app.Commands = []cli.Command{
		{
			Name:  "create",
			Usage: "create a new backup ",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "cephconf",
					Value: "/etc/ceph/ceph.conf",
					Usage: "the location of ceph.conf",
				},
				cli.StringFlag{
					Name:  "outputdir",
					Value: "backup.dir",
					Usage: "the output directory",
				},
				cli.StringSliceFlag{
					Name:  "skip-pool",
					Usage: "the ceph pools to skip",
				},
				cli.StringSliceFlag{
					Name:  "include-pool",
					Usage: "the ceph pools to include (default: all)",
				},
				cli.StringFlag{
					Name:  "etcd",
					Value: "127.0.0.1:2379",
					Usage: "the address of the etcd server",
				},
				cli.StringFlag{
					Name:  "incremental-over",
					Usage: "perform the backup as incremental over the given backup",
				},
				cli.BoolFlag{
					Name:  "skip-ceph",
					Usage: "just back up etcd",
				},
				cli.BoolFlag{
					Name:  "skip-etcd",
					Usage: "just back up ceph",
				},
			},
			Action: cli.ActionFunc(core.Create),
		},
		{
			Name:   "inspect",
			Usage:  "check the integrity of some backup files",
			Action: cli.ActionFunc(core.Inspect),
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "outputdir",
					Value: "backup.dir",
					Usage: "the output directory containing the backup",
				},
			},
		},
		{
			Name:   "clear",
			Usage:  "delete everything in the restore target",
			Action: cli.ActionFunc(core.Clear),
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "cephconf",
					Value: "/etc/ceph/ceph.conf",
					Usage: "the location of ceph.conf",
				},
				cli.StringSliceFlag{
					Name:  "include-pool",
					Usage: "the ceph pools to include (default: none)",
				},
				cli.StringFlag{
					Name:  "etcd",
					Value: "127.0.0.1:2379",
					Usage: "the address of the etcd server",
				},
				cli.BoolFlag{
					Name:  "skip-ceph",
					Usage: "just clear etcd",
				},
				cli.BoolFlag{
					Name:  "skip-etcd",
					Usage: "just clear ceph",
				},
			},
		},
		{
			Name:   "restore",
			Usage:  "restore a backup to a fresh cluster",
			Action: cli.ActionFunc(core.Restore),
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "cephconf",
					Value: "/etc/ceph/ceph.conf",
					Usage: "the location of ceph.conf",
				},
				cli.StringFlag{
					Name:  "outputdir",
					Value: "backup.dir",
					Usage: "the output directory containing the backup",
				},
				cli.StringSliceFlag{
					Name:  "restore-pool",
					Usage: "<backupname>:<restorename>",
				},
				cli.StringFlag{
					Name:  "etcd",
					Value: "127.0.0.1:2379",
					Usage: "the address of the etcd server",
				},
				cli.BoolFlag{
					Name:  "skip-ceph",
					Usage: "just restore etcd",
				},
				cli.BoolFlag{
					Name:  "skip-etcd",
					Usage: "just restore ceph",
				},
			},
		},
	}
	app.Run(os.Args)
}
