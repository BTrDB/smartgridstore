// Copyright (c) 2021 Michael Andersen
// Copyright (c) 2021 Regents of the University Of California
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/BTrDB/btrdb"
	"github.com/immesys/go-ceph/rados"
	"github.com/pborman/uuid"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "btrdbdu"
	app.Usage = "storage utilisation checker for BTrDB"
	app.Action = run
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "cephconf",
			Value: "/etc/ceph/ceph.conf",
			Usage: "location of ceph config",
		},
		cli.StringFlag{
			Name:  "btrdb",
			Value: "127.0.0.1:4410",
			Usage: "btrdb API port",
		},
		cli.StringFlag{
			Name:  "hot",
			Value: "btrdb_hot",
			Usage: "hot pool",
		},
		cli.StringFlag{
			Name:  "data",
			Value: "btrdb_data",
			Usage: "data pool",
		},
		cli.StringFlag{
			Name:  "csvout",
			Value: "",
			Usage: "raw details csv file",
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		panic(err)
	}
}

type usageType struct {
	totalHotSize  uint64
	totalColdSize uint64
	totalPoints   uint64
}

func run(c *cli.Context) error {
	db, err := btrdb.Connect(context.Background(), c.GlobalString("btrdb"))
	if err != nil {
		fmt.Printf("could not connect to btrdb: %v\n", err)
		os.Exit(1)
	}
	conn, err := rados.NewConn()
	if err != nil {
		panic(err)
	}
	err = conn.ReadConfigFile(c.GlobalString("cephconf"))
	if err != nil {
		fmt.Printf("could not read ceph config: %v\n", err)
		os.Exit(1)
	}
	err = conn.Connect()
	if err != nil {
		fmt.Printf("could not connect to ceph: %v\n", err)
		os.Exit(1)
	}
	defer conn.Shutdown()

	hotctx, err := conn.OpenIOContext(c.GlobalString("hot"))
	if err != nil {
		fmt.Printf("could not open hot pool %q: %v\n", c.GlobalString("hot"), err)
		os.Exit(1)
	}
	hotctx2, err := conn.OpenIOContext(c.GlobalString("hot"))
	if err != nil {
		fmt.Printf("could not open hot pool %q: %v\n", c.GlobalString("hot"), err)
		os.Exit(1)
	}
	coldctx, err := conn.OpenIOContext(c.GlobalString("data"))
	if err != nil {
		fmt.Printf("could not open cold pool %q: %v\n", c.GlobalString("data"), err)
		os.Exit(1)
	}
	coldctx2, err := conn.OpenIOContext(c.GlobalString("data"))
	if err != nil {
		fmt.Printf("could not open cold pool %q: %v\n", c.GlobalString("data"), err)
		os.Exit(1)
	}

	//-----------
	usage := make(map[[16]byte]*usageType)
	clusage := make(map[string]*usageType)
	clmap := make(map[string][][]byte)
	cnt := 0
	totalhot := uint64(0)
	totalcold := uint64(0)
	updateclusage := func(uuid []byte, coldsize uint64, hotsize uint64) {
		str := db.StreamFromUUID(uuid)
		col, err := str.Collection(context.Background())
		if err != nil {
			if btrdb.ToCodedError(err).Code == 404 {
				return
			}
			fmt.Printf("btrdb error: %v\n", err)
			os.Exit(1)
		}
		csp, _, cerr := str.AlignedWindows(context.Background(), 0, (1<<61)-1, 61, 0)
		sv := <-csp
		err = <-cerr
		if err != nil {
			fmt.Printf("could not count stream: %v\n", err)
			os.Exit(1)
		}

		parts := strings.Split(col, "/")
		for i := 1; i <= len(parts); i++ {
			subcol := strings.Join(parts[:i], "/")
			if clusage[subcol] == nil {
				clusage[subcol] = &usageType{}
			}
			clusage[subcol].totalColdSize += coldsize
			clusage[subcol].totalHotSize += hotsize
			clusage[subcol].totalPoints += sv.Count
		}
		clmap[col] = append(clmap[col], uuid)
		arr := [16]byte{}
		copy(arr[:], uuid[:])
		usage[arr].totalPoints = sv.Count
	}
	hotctx.ListObjects(func(oid string) {
		cnt++
		if cnt%3000 == 0 {
			fmt.Printf("\rscanned %d k objects  ", cnt)
		}
		uu, ok := extractUuid(oid)
		if !ok {
			return // non stream object
		}
		stats, err := hotctx2.Stat(oid)
		if err != nil {
			fmt.Printf("ceph error: %v\n", err)
			os.Exit(1)
		}
		if usage[uu] == nil {
			usage[uu] = &usageType{}
		}
		usage[uu].totalHotSize += stats.Size
		updateclusage(uu[:], 0, stats.Size)
		totalhot += stats.Size
	})
	coldctx.ListObjects(func(oid string) {
		cnt++
		if cnt%1000 == 0 {
			fmt.Printf("\rscanned %d k objects  ", cnt)
		}
		uu, ok := extractUuid(oid)
		if !ok {
			fmt.Printf("skipping %q\n", oid)
			return // non stream object
		}
		stats, err := coldctx2.Stat(oid)
		if err != nil {
			fmt.Printf("ceph error: %v\n", err)
			os.Exit(1)
		}
		if usage[uu] == nil {
			usage[uu] = &usageType{}
		}
		usage[uu].totalColdSize += stats.Size
		updateclusage(uu[:], stats.Size, 0)
		totalcold += stats.Size
	})

	fmt.Printf("\ncomplete\n")
	// for uu, rec := range usage {
	// 	struu := uuid.UUID(uu[:]).String()
	// 	fmt.Printf("%s %6.2f %6.2f\n", struu, float64(rec.totalColdSize)/(1024*1024), float64(rec.totalHotSize)/(1024*1024))
	// }
	cols := []string{}
	for col, _ := range clusage {
		cols = append(cols, col)
	}
	sort.Strings(cols)
	fmt.Printf("Collection size summary:\n")
	fmt.Printf("      HOT      COLD    PTS    /1MP Collection prefix\n")
	for _, col := range cols {
		rec := clusage[col]
		permp := (float64(rec.totalHotSize+rec.totalColdSize) / (1024 * 1024)) / (float64(rec.totalPoints) / 1e6)
		fmt.Printf("%8.2fM %8.2fM %5dM %6.2fM %s\n", float64(rec.totalHotSize)/(1024*1024), float64(rec.totalColdSize)/(1024*1024), int(math.Ceil(float64(rec.totalPoints)/1e6)), permp, col)
	}
	fmt.Printf("=========\n")
	fmt.Printf("Total hot object size : %.2f MB\n", float64(totalhot)/(1024*1024))
	fmt.Printf("Total cold object size: %.2f MB\n", float64(totalcold)/(1024*1024))

	//------
	csvoutfile := c.GlobalString("csvout")
	if csvoutfile == "" {
		return nil
	}
	f, err := os.Create(csvoutfile)
	if err != nil {
		fmt.Printf("could not create CSV output file: %v\n", err)
		os.Exit(1)
	}
	f.Write([]byte("uuid, hotsize, coldsize, points\n"))

	for uu, rec := range usage {
		f.Write([]byte(fmt.Sprintf("%s,%d,%d,%d\n", uuid.UUID(uu[:]).String(), rec.totalHotSize, rec.totalColdSize, rec.totalPoints)))
	}
	f.Close()
	return nil
}

func extractUuid(oid string) ([16]byte, bool) {
	if len(oid) < 32 {
		return [16]byte{}, false
	}
	uu, err := hex.DecodeString(oid[:32])
	if err != nil {
		return [16]byte{}, false
	}
	rv := [16]byte{}
	copy(rv[:], uu[:])
	return rv, true
}
