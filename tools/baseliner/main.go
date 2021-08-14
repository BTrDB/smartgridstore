// Copyright (c) 2021 Michael Andersen
// Copyright (c) 2021 Regents of the University Of California
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	btrdb "github.com/BTrDB/btrdb"
	"github.com/montanaflynn/stats"
	"github.com/pborman/uuid"
)

//Constants
const InsertSize = 120
const InsertInterval = 1 * time.Second

const DelayBetweenWorkerSets = 500 * time.Millisecond
const WorkerSets = 10

var StreamsPerWorker int64 = 5

const DelayBetweenStreams = 15 * time.Millisecond

var activeStreams int64
var droppedRecords int64

var logSpanCh chan span
var exstart time.Time
var gsetcode int

var totalinserts int64
var totaltime int64
var contexts [][]context.CancelFunc
var windowtimes []float64
var windowmu sync.Mutex

func main() {
	exstart = time.Now()
	rand.Seed(time.Now().UnixNano())
	gsetcode = rand.Int() % 0xFFFFFF
	fmt.Printf("SET CODE IS %06x\n", gsetcode)
	logSpanCh = make(chan span, 1000)
	go logSpans()
	go printAverages()
	dorun()
}

func printAverages() {
	var lastinserts int64
	var lasttime int64
	for {
		time.Sleep(15 * time.Second)
		tinserts := atomic.LoadInt64(&totalinserts)
		ttime := atomic.LoadInt64(&totaltime)
		avg := (float64(ttime/1000) / float64(tinserts)) / 1000.0
		dinserts := tinserts - lastinserts
		dtime := ttime - lasttime
		davg := (float64(dtime/1000) / float64(dinserts)) / 1000.0
		windowmu.Lock()
		dat := windowtimes
		windowtimes = make([]float64, 0)
		windowmu.Unlock()
		p95, e := stats.Percentile(stats.Float64Data(dat), 95)
		if e != nil {
			fmt.Printf("percentile error: %v\n", e)
		}
		fmt.Printf("total_inserts=%d avg_time=%.3fms [last15s: +inserts=%d +avg_time=%.3fms p95=%.3fms]\n", tinserts, avg, dinserts, davg, p95)
		lastinserts = tinserts
		lasttime = ttime
	}
}

func dorun() {
	contexts = make([][]context.CancelFunc, WorkerSets)
	for i := 0; i < WorkerSets; i++ {
		contexts[i] = make([]context.CancelFunc, 0)
	}
	for i := 0; i < WorkerSets; i++ {
		go updateWorkerSet(i)
		fmt.Fprintf(os.Stderr, "started worker set %d\n", i)
		time.Sleep(DelayBetweenWorkerSets)
	}
	for {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Total streams: ")
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)
		v, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			fmt.Println(err.Error())
		} else {
			v /= WorkerSets
			atomic.StoreInt64(&StreamsPerWorker, v)
		}
	}
}

func updateWorkerSet(number int) {
	b, err := btrdb.Connect(context.Background(), btrdb.EndpointsFromEnv()...)
	if err != nil {
		panic(err)
	}
	for {
		SPW := atomic.LoadInt64(&StreamsPerWorker)
		for i := len(contexts[number]); i < int(SPW); i++ {
			atomic.AddInt64(&activeStreams, 1)
			fmt.Fprintf(os.Stderr, "started stream %d::%d (%d total)\n", number, i, activeStreams)
			ctx, ccancel := context.WithCancel(context.Background())
			contexts[number] = append(contexts[number], ccancel)
			go beginStream(ctx, b)
			time.Sleep(DelayBetweenStreams)
		}
		for i := int(SPW); i < len(contexts[number]); i++ {
			atomic.AddInt64(&activeStreams, -1)
			fmt.Fprintf(os.Stderr, "killed stream %d::%d (%d total)\n", number, i, activeStreams)
			contexts[number][i]()
			//time.Sleep(DelayBetweenStreams)
		}
		contexts[number] = contexts[number][:SPW]
		time.Sleep(500 * time.Millisecond)
	}
}

func beginStream(ctx context.Context, db *btrdb.BTrDB) {
	uu := uuid.NewRandom()
	collection := fmt.Sprintf("sim/%06x/%16x", gsetcode, uint64(rand.Int63()))
	stream, err := db.Create(context.Background(), uu, collection, btrdb.M{"name": "stream"}, nil)
	if err != nil {
		panic(err)
	}
	last := time.Now()
	for {
		if ctx.Err() != nil {
			return
		}
		data := make([]btrdb.RawPoint, InsertSize)

		for i := 0; i < InsertSize; i++ {
			t := last.Add(time.Duration(i) * (InsertInterval / InsertSize)).UnixNano()
			data[i] = btrdb.RawPoint{Time: t, Value: math.Sin(float64(t / 1e9))}
		}
		istart := time.Now()
		err = stream.Insert(context.Background(), data)
		if err != nil {
			panic(err)
		}
		iend := time.Now()
		atomic.AddInt64(&totalinserts, 1)
		atomic.AddInt64(&totaltime, int64(iend.Sub(istart)))
		windowmu.Lock()
		windowtimes = append(windowtimes, float64(int64(iend.Sub(istart))/1000)/1000.0)
		windowmu.Unlock()
		select {
		case logSpanCh <- span{Duration: float64(iend.Sub(istart)/1000) / 1000.0, Time: float64(int64((istart.Sub(exstart) / time.Millisecond)) / 1000.0), ActiveStreams: atomic.LoadInt64(&activeStreams)}:
		default:
			atomic.AddInt64(&droppedRecords, 1)
		}
		if ctx.Err() != nil {
			return
		}
		next := last.Add(InsertInterval)
		if next.Before(time.Now()) {
			next = time.Now()
		} else {
			time.Sleep(next.Sub(time.Now()))
		}
		last = next
	}
}

type span struct {
	Duration      float64
	ActiveStreams int64
	Time          float64 `json:"offset"`
}

var spanmu sync.Mutex
var spandata []span

func logSpans() {
	f, err := os.Create("results.json")
	if err != nil {
		panic(err)
	}
	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, os.Interrupt)
		<-c
		f.Close()
		os.Exit(1)
	}()
	enc := json.NewEncoder(f)
	for s := range logSpanCh {
		/*err := enc.Encode(s)
		if err != nil {
			panic(err)
		}*/
		_ = s
		_ = enc
	}
	// spanmu.Lock()
	// spandata = append(spandata)
	// spanmu.Unlock()
}

//Important baselines

//latency distribution of
// -insert vs insert concurrency
//  for size 1
//  size 10
// size 100
// size 1000
// size 10000
