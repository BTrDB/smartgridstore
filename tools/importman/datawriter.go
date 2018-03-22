package importman

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BTrDB/smartgridstore/tools/importman/plugins"
	"github.com/pborman/uuid"
	btrdb "gopkg.in/btrdb.v4"
	pb "gopkg.in/cheggaaa/pb.v2"
)

type streamKey struct {
	collection string
	sertags    string
}
type streamVal struct {
	ready  chan struct{}
	stream *btrdb.Stream
}

type dataWriter struct {
	collectionPrefix string
	db               *btrdb.BTrDB
	input            chan plugins.Stream
	done             chan struct{}
	bardone          chan struct{}
	streams          map[streamKey]*streamVal
	totalQueued      int64
	totalWritten     int64
	totalInserts     int64
	totalFailedFinds int64
	unknown          bool
	bar              *pb.ProgressBar
	checkExisting    bool
	mu               sync.Mutex
	barmu            sync.Mutex
	wg               sync.WaitGroup
}

func NewDataWriter(collectionPrefix string, checkExisting bool, total int64) *dataWriter {
	if collectionPrefix == "" {
		fmt.Printf("no collection specified\n")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := btrdb.Connect(ctx, btrdb.EndpointsFromEnv()...)
	if err != nil {
		fmt.Printf("could not connect to BTrDB: %v\n", err)
		os.Exit(1)
	}
	rv := &dataWriter{
		db:               db,
		collectionPrefix: collectionPrefix,
		input:            make(chan plugins.Stream, 5000),
		done:             make(chan struct{}),
		bardone:          make(chan struct{}),
		checkExisting:    checkExisting,
		bar:              pb.Full.Start(int(total)),
		streams:          make(map[streamKey]*streamVal),
	}
	go rv.startWorkers()
	return rv
}

func (dw *dataWriter) Wait() {
	<-dw.done
	<-dw.bardone
}

func (dw *dataWriter) startWorkers() {
	const numworkers = 500
	dw.wg.Add(numworkers)
	for i := 0; i < numworkers; i++ {
		go dw.startSingleWorkerLoop()
	}
	dw.wg.Wait()
	close(dw.done)
	dw.bar.SetTotal(dw.totalQueued)
	dw.bar.SetCurrent(dw.totalWritten)
	dw.bar.Finish()
	close(dw.bardone)
}
func (dw *dataWriter) Enqueue(sz []plugins.Stream) {
	for _, s := range sz {
		ttl, known := s.Total()
		if known {
			atomic.AddInt64(&dw.totalQueued, ttl)
		} else {
			dw.unknown = true
		}
		dw.input <- s
	}
}
func (dw *dataWriter) NoMoreStreams() {
	fmt.Printf("total queued points is %d\n", dw.totalQueued)
	close(dw.input)
}
func (dw *dataWriter) getHandleFor(s plugins.Stream) *btrdb.Stream {
	if s == nil {
		panic("nil stream")
	}
	sk := streamKey{
		collection: dw.collectionPrefix + s.CollectionSuffix(),
	}
	sertags := []string{}
	opttags := make(map[string]*string)
	for k, v := range s.Tags() {
		sertags = append(sertags, fmt.Sprintf("%s=%s;", k, v))
		lk := k
		lv := v
		opttags[lk] = &lv
	}
	sort.Strings(sertags)
	sk.sertags = strings.Join(sertags, "")
	dw.mu.Lock()
	str, ok := dw.streams[sk]
	if !ok {
		str = &streamVal{
			ready: make(chan struct{}),
		}
		dw.streams[sk] = str
		dw.mu.Unlock()
		mustcreate := true
		if dw.checkExisting {
			//Try lookup the stream
			rv, err := dw.db.LookupStreams(context.Background(), sk.collection, false, opttags, nil)
			if err != nil {
				fmt.Printf("could not lookup streams: %v\n", err)
				os.Exit(1)
			}
			if len(rv) > 1 {
				fmt.Printf("stream is ambiguous, there are multiple matches\n")
				os.Exit(1)
			}
			if len(rv) == 1 {
				mustcreate = false
				str.stream = rv[0]
				close(str.ready)
			}
		}
		if mustcreate {
			var err error
			uu := uuid.NewRandom()
			stream, err := dw.db.Create(context.Background(), uu, sk.collection, s.Tags(), s.Annotations())
			if err != nil {
				fmt.Printf("could not create stream: %v\n", err)
				os.Exit(1)
			}
			str.stream = stream
			close(str.ready)
		}
	} else {
		dw.mu.Unlock()
	}
	<-str.ready
	return str.stream
}
func (dw *dataWriter) startSingleWorkerLoop() {
	for {
		stream, ok := <-dw.input
		if !ok {
			dw.wg.Done()
			return
		}
		dbstream := dw.getHandleFor(stream)
		pts := stream.Next()
		for len(pts) > 0 {
			err := dbstream.InsertF(context.Background(), len(pts), func(i int) int64 {
				return pts[i].Time
			}, func(i int) float64 {
				return pts[i].Value
			})

			dw.barmu.Lock()
			dw.totalWritten += int64(len(pts))
			dw.bar.SetCurrent(dw.totalWritten)
			dw.barmu.Unlock()
			if err != nil {
				fmt.Printf("error writing to BTrDB: %v\n", err)
				os.Exit(1)
			}
			pts = stream.Next()
		}
	}
}
