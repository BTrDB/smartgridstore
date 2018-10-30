package gen2ingress

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/pborman/uuid"

	btrdb "gopkg.in/BTrDB/btrdb.v4"
)

type Inserter struct {
	cachemu          sync.Mutex
	streamcache      map[streamkey]*btrdb.Stream
	db               *btrdb.BTrDB
	workq            chan InsertRecord
	coalesceInterval time.Duration
	maxSize          int64
	curSize          int64
	dropped          int64
}
type streamkey struct {
	Collection string
	Name       string
	Unit       string
}

//Default queue size is 1GB
const DefaultWorkQueueLength = 1000 * 1000 * 1000

func NewInserter(db *btrdb.BTrDB) *Inserter {
	wql := DefaultWorkQueueLength
	paramQueueLength := os.Getenv("WORK_QUEUE_LENGTH")
	if paramQueueLength != "" {
		parsed, err := strconv.ParseInt(paramQueueLength, 10, 64)
		if err != nil {
			panic(err)
		}
		wql = int(parsed)
	}
	rv := Inserter{
		streamcache: make(map[streamkey]*btrdb.Stream),
		db:          db,
		//We assume a worst case where every record is just one raw point
		//which makes Size() return 116
		workq:            make(chan InsertRecord, wql/116),
		coalesceInterval: 2 * time.Second,
		maxSize:          int64(wql),
	}
	for i := 0; i < 4; i++ {
		go rv.worker()
	}
	go func() {
		var lastDropped int64
		for {
			deltaDropped := atomic.LoadInt64(&rv.dropped) - lastDropped
			if deltaDropped > 0 {
				fmt.Printf("CRITICAL: IN THE PAST 5 SECONDS, %d BATCHES HAVE BEEN DROPPED\n", deltaDropped)
			}
			curSize := atomic.LoadInt64(&rv.curSize)
			fullPercentage := float64(curSize) / float64(rv.maxSize) * 100
			if fullPercentage > 5 {
				fmt.Printf("WARNING: QUEUE IS %.2f%% FULL.\n", fullPercentage)
			}
			time.Sleep(5 * time.Second)
		}
	}()
	return &rv
}

func (ins *Inserter) SetCoalesceInterval(d time.Duration) {
	ins.coalesceInterval = d
}
func (ins *Inserter) ProcessBatch(ir []InsertRecord) {
	irSize := 0
	for _, r := range ir {
		irSize += r.Size()
	}
	cursize := atomic.LoadInt64(&ins.curSize)
	ok := cursize+int64(irSize) < ins.maxSize

	if !ok {
		atomic.AddInt64(&ins.dropped, int64(len(ir)))
		return
	}
	atomic.AddInt64(&ins.curSize, int64(irSize))
	for _, v := range ir {
		ins.workq <- v
	}
}

func (ins *Inserter) worker() {
	debugDelay := os.Getenv("DEBUG_DELAY_INSERTS") == "YES"
	buf := make(map[streamkey][]btrdb.RawPoint)
	anns := make(map[streamkey]map[string]string)
	coalesceTime := time.Now()
	flushBuf := func() {
		then := time.Now()
		total := 0
		if debugDelay {
			time.Sleep(10 * time.Second)
		}
		for sk, dat := range buf {
			ins.cachemu.Lock()
			stream, ok := ins.streamcache[sk]
			if !ok {
				sz, err := ins.db.LookupStreams(context.Background(), sk.Collection,
					false, btrdb.OptKV("name", sk.Name), nil)
				if err != nil {
					panic(err)
				}
				if len(sz) == 0 {
					//create stream and assign to stream
					uu := uuid.NewRandom()
					st, err := ins.db.Create(context.Background(),
						uu, sk.Collection, btrdb.M{"name": sk.Name, "unit": sk.Unit}, nil)
					if err != nil {
						panic(err)
					}
					stream = st
					ins.streamcache[sk] = stream
				} else {
					stream = sz[0]
					ins.streamcache[sk] = stream
				}
			}
			ins.cachemu.Unlock()

			ann, ok := anns[sk]
			if ok {
				_, aver, err := stream.Annotations(context.Background())
				if err != nil {
					panic(err)
				}
				changes := make(map[string]*string)
				for k, v := range ann {
					vc := v
					changes[k] = &vc
				}
				err = stream.CompareAndSetAnnotation(context.Background(), aver, changes)
				if err != nil {
					fmt.Printf("failed to set annotations: %v (%d)\n", err, aver)
				} else {
					delete(anns, sk)
				}
			}

			total += len(dat)
			err := stream.Insert(context.Background(), dat)
			if err != nil {
				fmt.Printf("Got insert error (ignoring): %v\n", err)
			}
		}
		buf = make(map[streamkey][]btrdb.RawPoint)
		coalesceTime = time.Now()
		fmt.Printf("Batch of %d readings processed in %.2f ms\n", total, float64(coalesceTime.Sub(then)/time.Microsecond)/1000.0)
	}
	for {
		var ir InsertRecord
		//Opportunistically flush the buffer if there is no work to be done
		select {
		case ir = <-ins.workq:
		default:
			flushBuf()
			ir = <-ins.workq
		}
		atomic.AddInt64(&ins.curSize, -int64(ir.Size()))
		if len(ir.Data) == 0 {
			continue
		}
		if ir.Name == "" || ir.Collection == "" {
			spew.Dump(ir)
			panic("missing name or collection")
		}
		sk := streamkey{Name: ir.Name,
			Collection: ir.Collection,
			Unit:       ir.Unit}
		if ir.AnnotationChanges != nil {
			anns[sk] = ir.AnnotationChanges
		}
		buf[sk] = append(buf[sk], ir.Data...)
		if time.Since(coalesceTime) > ins.coalesceInterval || len(buf) > 4000 {
			flushBuf()
		}
	}
}
