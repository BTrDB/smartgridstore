package main

import (
	"context"
	"fmt"
	"math"
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
	workq            chan []*DataFrame
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
		workq:            make(chan []*DataFrame),
	}
	go rv.worker()
	return &rv
}

func (ins *Inserter) ProcessBatch(df map[uint16][]*DataFrame) {
	for _, v := range df {
		ins.workq <- v
	}
}

func (ins *Inserter) mkc(d *PMUData) string {
	return fmt.Sprintf("%s/%d_%s", ins.CollectionPrefix, d.IDCODE, d.STN)
}
func (ins *Inserter) worker() {
	for {
		buf := make(map[streamkey][]btrdb.RawPoint)
		data := <-ins.workq
		if len(data) == 0 {
			continue
		}
		then := time.Now()
		for _, d := range data {
			ts := d.UTCUnixNanos

			for _, pm := range d.Data {
				skStats := streamkey{Name: "STAT",
					Collection: ins.mkc(pm),
					Unit:       "STAT"}
				buf[skStats] = append(buf[skStats],
					btrdb.RawPoint{Time: ts, Value: float64(pm.STAT)})
				if pm.STAT&0x8000 != 0 {
					//STAT field says drop this data
					continue
				}
				skTQ := streamkey{Name: "TIMEQUAL",
					Collection: ins.mkc(pm),
					Unit:       "TQ"}
				buf[skTQ] = append(buf[skTQ],
					btrdb.RawPoint{Time: ts, Value: float64(d.TimeQual)})

				skFreq := streamkey{Name: "FREQ",
					Collection: ins.mkc(pm),
					Unit:       "Hz"}
				buf[skFreq] = append(buf[skFreq],
					btrdb.RawPoint{Time: ts, Value: pm.FREQ})

				skDFreq := streamkey{Name: "DFREQ",
					Collection: ins.mkc(pm),
					Unit:       "ROCOF"}
				buf[skDFreq] = append(buf[skDFreq],
					btrdb.RawPoint{Time: ts, Value: pm.DFREQ})

				for phi, ph := range pm.PHASOR_NAMES {
					unit := "Volt"
					u := "VOL"
					if !pm.PHASOR_ISVOLT[phi] {
						u = "CUR"
						unit = "Amp"
					}
					if ph == "" {
						ph = u
					}
					if math.IsNaN(pm.PHASOR_MAG[phi]) {
						fmt.Printf("WARN, device %d issues NaN magnitude\n", pm.IDCODE)
						continue
					}
					if math.IsNaN(pm.PHASOR_ANG[phi]) {
						fmt.Printf("WARN, device %d issues NaN angle\n", pm.IDCODE)
						continue
					}
					skphmag := streamkey{Name: fmt.Sprintf("PH%dMAG %s", phi, ph),
						Collection: ins.mkc(pm),
						Unit:       unit}
					buf[skphmag] = append(buf[skphmag],
						btrdb.RawPoint{Time: ts, Value: pm.PHASOR_MAG[phi]})
					skphang := streamkey{Name: fmt.Sprintf("PH%dANG %s", phi, ph),
						Collection: ins.mkc(pm),
						Unit:       "degrees"}
					buf[skphang] = append(buf[skphang],
						btrdb.RawPoint{Time: ts, Value: pm.PHASOR_ANG[phi]})
				}
				for ani, an := range pm.ANALOG_NAMES {
					nm := fmt.Sprintf("AN%d %s", ani, an)
					if an == "" {
						nm = fmt.Sprintf("AN%d", ani)
					}
					if math.IsNaN(pm.ANALOG[ani]) {
						fmt.Printf("WARN, device %d issues NaN analog channel\n", pm.IDCODE)
						continue
					}
					ska := streamkey{Name: nm,
						Collection: ins.mkc(pm),
						Unit:       "analog"}
					buf[ska] = append(buf[ska],
						btrdb.RawPoint{Time: ts, Value: pm.ANALOG[ani]})
				}
				for dgi, _ := range pm.DIGITAL_NAMES {
					skd := streamkey{Name: fmt.Sprintf("DG%d", dgi),
						Collection: ins.mkc(pm),
						Unit:       "digital"}
					buf[skd] = append(buf[skd],
						btrdb.RawPoint{Time: ts, Value: float64(pm.DIGITAL[dgi])})
				}
			}
		} //end for over data
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
			total += len(dat)
			err := stream.Insert(context.Background(), dat)
			if err != nil {
				fmt.Printf("Got insert error (ignoring): %v\n", err)
			}
		}
		now := time.Now()
		fmt.Printf("Batch of %d readings processed in %.2f ms\n", total, float64(now.Sub(then)/time.Microsecond)/1000.0)
	}
}
