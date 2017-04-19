package main

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/immesys/smartgridstore/tools"
)

const VersionMajor = tools.VersionMajor
const VersionMinor = tools.VersionMinor
const VersionPatch = tools.VersionPatch

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

	num_tcps := os.Getenv("SIMULATOR_TCP_CONNECTIONS")
	if num_tcps == "" {
		fmt.Println("Missing $SIMULATOR_TCP_CONNECTIONS, assuming 1")
		num_tcps = "1"
	}

	//How many PMUs to simulate
	num_pmus_per := os.Getenv("SIMULATOR_NUM_PER_TCP")
	if num_pmus_per == "" {
		fmt.Println("Missing $SIMULATOR_NUM_PER_TCP, assuming 1")
		num_pmus_per = "1"
	}

	//How long to wait between sending files. The files are always 2 minutes
	//long, but we may send them every 1 minute. This is to simulate backfill
	interval := os.Getenv("SIMULATOR_INTERVAL")
	if interval == "" {
		fmt.Println("Missing $SIMULATOR_INTERVAL, assuming 120")
		interval = "120"
	}

	//The total time over which TCP connections are made. This is to avoid
	//all of the connections from being made at once
	startup := os.Getenv("SIMULATOR_STARTUP")
	if startup == "" {
		fmt.Printf("Missing $SIMULATOR_STARTUP, assuming %s (same as $SIMULATOR_INTERVAL)\n", interval)
		startup = interval
	}

	i_num_tcps, err := strconv.ParseInt(num_tcps, 10, 64)
	if err != nil {
		fmt.Println("Could not parse SIMULATOR_NUM_TCPS")
		os.Exit(2)
	}

	i_num_pmus_per, err := strconv.ParseInt(num_pmus_per, 10, 64)
	if err != nil {
		fmt.Println("Could not parse SIMULATOR_NUM_PER_TCP")
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

	i_startup, err := strconv.ParseInt(startup, 10, 64)
	if err != nil {
		fmt.Println("Could not parse SIMULATOR_STARTUP")
		os.Exit(2)
	}

	for i := int64(0); i < i_num_tcps; i++ {
		go func(index int64) {
			//Add jitter to simulation
			time.Sleep(time.Duration(float64(i_startup)*rand.Float64()*1000.0) * time.Millisecond)
			for {
				lock := &sync.Mutex{}

				fmt.Printf("Connecting to server %s\n", target)
				conn, err := net.Dial("tcp", target)
				if err != nil {
					fmt.Printf("Could not connect to receiver: %v\n", err)
					time.Sleep(time.Duration(i_interval) * time.Second)
					continue
				}

				wg := &sync.WaitGroup{}

				for j := int64(0); j < i_num_pmus_per; j++ {
					serial := int64(3500000) + ((index * i_num_pmus_per) + j) + i_offset
					wg.Add(1)
					go simulatePmu(conn, serial, i_interval, lock, wg)
				}

				wg.Wait()

				fmt.Printf("Connection to %s was dropped!\n", target)
				fmt.Println("Restarting...")
			}
		}(i)
	}

	for {
		l_sent := atomic.LoadInt64(&sent)
		fmt.Printf("Sent %d files\n", l_sent)
		time.Sleep(5 * time.Second)
	}
}
