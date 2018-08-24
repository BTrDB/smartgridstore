package gen2ingress

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pborman/uuid"

	btrdb "gopkg.in/BTrDB/btrdb.v4"
)

type Inserter struct {
	CollectionPrefix string
	cachemu          sync.Mutex
	streamcache      map[streamkey]*btrdb.Stream
	db               *btrdb.BTrDB
	workq            chan InsertRecord
	coalesceInterval time.Duration
}
type streamkey struct {
	Collection string
	Name       string
	Unit       string
}

func NewInserter(db *btrdb.BTrDB, prefix string) *Inserter {
	prefix = strings.TrimSuffix(prefix, "/")
	rv := Inserter{
		CollectionPrefix: prefix,
		streamcache:      make(map[streamkey]*btrdb.Stream),
		db:               db,
		workq:            make(chan InsertRecord, 100),
		coalesceInterval: time.Second,
	}
	go rv.worker()
	go rv.worker()
	return &rv
}

func (ins *Inserter) SetCoalesceInterval(d time.Duration) {
	ins.coalesceInterval = d
}
func (ins *Inserter) ProcessBatch(ir []InsertRecord) {
	for _, v := range ir {
		ins.workq <- v
	}
}

func (ins *Inserter) worker() {
	buf := make(map[streamkey][]btrdb.RawPoint)
	coalesceTime := time.Now()
	flushBuf := func() {
		then := time.Now()
		total := 0
		for sk, dat := range buf {
			ins.cachemu.Lock()
			stream, ok := ins.streamcache[sk]
			ins.cachemu.Unlock()
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
					ins.cachemu.Lock()
					ins.streamcache[sk] = stream
					ins.cachemu.Unlock()
				} else {
					stream = sz[0]
					ins.cachemu.Lock()
					ins.streamcache[sk] = stream
					ins.cachemu.Unlock()
				}
			}
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
		if len(ir.Data()) == 0 {
			continue
		}
		sk := streamkey{Name: ir.Name(),
			Collection: ins.CollectionPrefix + ir.Collection(),
			Unit:       ir.Unit()}
		buf[sk] = append(buf[sk], ir.Data()...)
		if time.Since(coalesceTime) > ins.coalesceInterval {
			flushBuf()
		}
	}
}
