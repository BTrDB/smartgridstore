package main

import (
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

const VersionMajor = 4
const VersionMinor = 0
const VersionPatch = 0

var sent int64

func main() {
	if len(os.Args) == 2 && os.Args[1] == "-version" {
		fmt.Printf("%d.%d.%d\n", VersionMajor, VersionMinor, VersionPatch)
		os.Exit(0)
	}
	fmt.Printf("Booting simulator version %d.%d.%d\n", VersionMajor, VersionMinor, VersionPatch)
	//The receiver to send to (IP:PORT)
	target := os.Getenv("SIMULATOR_TARGET")
	if target == "" {
		fmt.Println("Missing $SIMULATOR_TARGET")
		os.Exit(1)
	}

	//The offset in serial numbers to use for this simulator
	offset := os.Getenv("SIMULATOR_SERIAL_OFFSET")
	if offset == "" {
		fmt.Println("Missing $SIMULATOR_SERIAL_OFFSET, assuming 0")
		offset = "1"
	}

	//How many PMUs to simulate
	num_pmus := os.Getenv("SIMULATOR_NUMBER")
	if num_pmus == "" {
		fmt.Println("Missing $SIMULATOR_NUMBER, assuming 1")
		num_pmus = "1"
	}

	//How long to wait between sending files. The files are always 2 minutes
	//long, but we may send them every 1 minute. This is to simulate backfill
	interval := os.Getenv("SIMULATOR_INTERVAL")
	if interval == "" {
		fmt.Println("Missing $SIMULATOR_INTERVAL, assuming 120")
		interval = "120"
	}

	i_num_pmus, err := strconv.ParseInt(num_pmus, 10, 64)
	if err != nil {
		fmt.Println("Could not parse SIMULATOR_NUMBER")
		os.Exit(2)
	}

	i_offset, err := strconv.ParseInt(offset, 10, 64)
	if err != nil {
		fmt.Println("Could not parse SIMULATOR_SERIAL_OFFSET")
		os.Exit(2)
	}

	i_interval, err := strconv.ParseInt(interval, 10, 64)
	if err != nil {
		fmt.Println("Could not parse SIMULATOR_INTERVAL")
		os.Exit(2)
	}

	for i := int64(0); i < i_num_pmus; i++ {
		go simulatePmu(target, 3500000+i+i_offset, i_interval)
	}

	for {
		l_sent := atomic.LoadInt64(&sent)
		fmt.Printf("Sent %d files\n", l_sent)
		time.Sleep(5 * time.Second)
	}
}
