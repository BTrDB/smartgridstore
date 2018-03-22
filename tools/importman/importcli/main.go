package main

import (
	"fmt"
	"log"
	"os"
	"time"

	//_ "net/http/pprof"

	"github.com/BTrDB/smartgridstore/tools"
	"github.com/BTrDB/smartgridstore/tools/importman"
	"github.com/BTrDB/smartgridstore/tools/importman/plugins/openhistorian"
	"github.com/urfave/cli"
)

func main() {

	// go func() {
	// 	fmt.Println("==== PROFILING ENABLED ==========")
	// 	runtime.SetBlockProfileRate(5000)
	// 	err := http.ListenAndServe("0.0.0.0:6060", nil)
	// 	panic(err)
	// }()

	app := cli.NewApp()
	app.EnableBashCompletion = true
	app.Usage = "Import data into BTrDB"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "collection",
			Usage: "a prefix to add to the collections created by the import operation",
		},
		cli.BoolFlag{
			Name:  "continue",
			Usage: "ensure data is merged into existing streams if they already exist",
		},
	}
	app.Version = fmt.Sprintf("%d.%d.%d", tools.VersionMajor, tools.VersionMinor, tools.VersionPatch)
	app.Commands = []cli.Command{
		{
			Name:      "importfiles",
			Usage:     "load data from files",
			ArgsUsage: "[input files]",
			Action:    importFiles,
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "openhist_v1",
					Usage: "treat files as OpenHistorian v1 files",
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func importFiles(c *cli.Context) error {
	if !c.Bool("openhist_v1") {
		fmt.Printf("please specify the format of the input files (e.g --openhist_v1)\n")
		os.Exit(1)
	}
	//We only support openhistorian files for now, so this is not hard
	driver, err := openhist.NewOpenHistorian(c.Args())
	if err != nil {
		fmt.Printf("failed to load files: %v\n", err)
		os.Exit(1)
	}
	ttl, _ := driver.Total()
	dw := importman.NewDataWriter(c.GlobalString("collection"), c.GlobalBool("continue"), ttl)

	then := time.Now()

	streams := driver.Next()
	for len(streams) > 0 {
		dw.Enqueue(streams)
		streams = driver.Next()
	}
	dw.NoMoreStreams()
	dw.Wait()

	fmt.Printf("import complete: %s\n", time.Now().Sub(then))
	return nil
}
