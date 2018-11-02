package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"time"

	"github.com/BTrDB/btrdb-server/bte"
	"github.com/pborman/uuid"
	v4 "gopkg.in/BTrDB/btrdb.v4"
	pb "gopkg.in/cheggaaa/pb.v1"
	yaml "gopkg.in/yaml.v2"
)

type StreamConfig struct {
	SrcCollection string
	DstCollection string
	Tags          map[string]string
}
type Config struct {
	FromServer    string
	ToServer      string
	StartTime     string
	EndTime       string
	AbortIfExists bool
	Streams       []StreamConfig
}
type Stream struct {
	CC string
	NC string
	T  map[string]string
}

func Count(bc *v4.BTrDB, col string, tags map[string]string, start int64, end int64) (uint64, *v4.Stream, error) {
	tagopt := make(map[string]*string)
	tagstr := ""
	for k, v := range tags {
		vv := v
		tagopt[k] = &vv
		tagstr = tagstr + fmt.Sprintf("%q=%q,", k, v)
	}
	sz, err := bc.LookupStreams(context.Background(), col, false, tagopt, nil)
	if err != nil {
		panic(err)
	}
	if len(sz) != 1 {
		return 0, nil, fmt.Errorf("collection %q with tags %s is ambiguous or incorrect, it matches %d streams", col, tagstr, len(sz))
	}
	s := sz[0]
	csp, _, cerr := s.Windows(context.Background(), start, end, uint64(end-start), 0, 0)
	sv := <-csp
	err = <-cerr
	if err != nil {
		panic(err)
	}
	return sv.Count, s, nil
}
func Query(bc *v4.BTrDB, col string, tags map[string]string, start int64, end int64) chan v4.RawPoint {
	tagopt := make(map[string]*string)
	tagstr := ""
	for k, v := range tags {
		vv := v
		tagopt[k] = &vv
		tagstr = tagstr + fmt.Sprintf("%q=%q,", k, v)
	}
	sz, err := bc.LookupStreams(context.Background(), col, false, tagopt, nil)
	if err != nil {
		panic(err)
	}
	if len(sz) != 1 {
		//We expect this to be caught when doing the count
		panic("too many result streams")
	}
	s := sz[0]
	sv, _, cerr := s.RawValues(context.Background(), start, end, 0)
	go func() {
		err := <-cerr
		if err != nil {
			ee := v4.ToCodedError(err)
			fmt.Printf("ERROR %#v\n", ee)
		}
	}()
	return sv
}

func main() {
	streams := []Stream{}
	cfg := Config{}
	if len(os.Args) != 2 {
		fmt.Printf("Usage: btrdbcp <config>\n")
		os.Exit(1)
	}
	cfgdata, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		fmt.Printf("Could not read config file %q: %v\n", os.Args[1], err)
		os.Exit(1)
	}
	err = yaml.Unmarshal(cfgdata, &cfg)
	if err != nil {
		fmt.Printf("Could not parse config file: %v\n", err)
		os.Exit(1)
	}
	if len(cfg.Streams) > 20 {
		fmt.Printf("We don't support doing more than 20 streams concurrently\n")
		os.Exit(1)
	}
	for idx, s := range cfg.Streams {
		if s.SrcCollection == "" {
			fmt.Printf("Missing srccollection field on stream %d\n", idx)
			os.Exit(1)
		}
		if s.DstCollection == "" {
			fmt.Printf("Missing dstcollection field on stream %d\n", idx)
			os.Exit(1)
		}
		streams = append(streams, Stream{
			CC: s.SrcCollection,
			NC: s.DstCollection,
			T:  s.Tags,
		})
	}

	starttimet, err := time.Parse(time.RFC3339, cfg.StartTime)
	if err != nil {
		fmt.Printf("Could not parse start time %q: %v\n", cfg.StartTime, err)
		os.Exit(1)
	}
	endtimet, err := time.Parse(time.RFC3339, cfg.EndTime)
	if err != nil {
		fmt.Printf("Could not parse end time %q: %v\n", cfg.EndTime, err)
		os.Exit(1)
	}
	starttime := starttimet.UnixNano()
	endtime := endtimet.UnixNano()
	var from, to *v4.BTrDB
	if os.Getenv("FROM_API_KEY") != "" {
		from, err = v4.ConnectAuth(context.Background(), os.Getenv("FROM_API_KEY"), cfg.FromServer)
	} else {
		from, err = v4.Connect(context.Background(), cfg.FromServer)
	}
	if err != nil {
		fmt.Printf("Could not connect to FromServer: %v\n", err)
		os.Exit(1)
	}
	if os.Getenv("TO_API_KEY") != "" {
		to, err = v4.ConnectAuth(context.Background(), os.Getenv("TO_API_KEY"), cfg.ToServer)
	} else {
		to, err = v4.Connect(context.Background(), cfg.ToServer)
	}
	if err != nil {
		fmt.Printf("Could not connect to ToServer: %v\n", err)
		os.Exit(1)
	}

	streamcounts := []uint64{}
	streamshz := []*v4.Stream{}
	bars := []*pb.ProgressBar{}
	wg := sync.WaitGroup{}
	for _, s := range streams {
		tagstr := ""
		for k, v := range s.T {
			tagstr = tagstr + fmt.Sprintf("%q=%q,", k, v)
		}
		streamdesc := fmt.Sprintf("%s/%s", s.CC, tagstr)
		cnt, st, err := Count(from, s.CC, s.T, starttime, endtime)
		if err != nil {
			fmt.Printf("ABORT, when counting %s, error:\n %v\n", streamdesc, err)
			os.Exit(1)
		}
		fmt.Printf("%s has %d points\n", streamdesc, cnt)
		streamcounts = append(streamcounts, cnt)
		streamshz = append(streamshz, st)
		bars = append(bars, pb.New(int(cnt)).Prefix(streamdesc))
		wg.Add(1)
	}
	pool, err := pb.StartPool(bars...)
	if err != nil {
		panic(err)
	}
	deststreams := []*v4.Stream{}
	for idx, s := range streams {
		tagstr := ""
		for k, v := range s.T {
			tagstr = tagstr + fmt.Sprintf("%q=%q,", k, v)
		}
		streamdesc := fmt.Sprintf("%s/%s", s.CC, tagstr)
		hz := streamshz[idx]
		uu := uuid.NewRandom()
		tags, err := hz.Tags(context.Background())
		if err != nil {
			panic(err)
		}
		anns, _, err := hz.CachedAnnotations(context.Background())
		if err != nil {
			panic(err)
		}
		v4stream, err := to.Create(context.Background(), uu, s.NC, tags, anns)
		if err != nil {
			cerr := v4.ToCodedError(err)
			if cerr.Code == bte.StreamExists {
				if cfg.AbortIfExists {
					fmt.Printf("ABORT: stream %s already exists\n", streamdesc)
					os.Exit(2)
				} else {
					panic("Have not implemented erasing dest streams yet")
					/* if the stream already exists you can use logic like this to erase the destination before copying
					sz, err := iconn.LookupStreams(context.Background(), s.NC, false, v4.OptKV("name", s.N, "unit", s.U), nil)
								if err != nil {
									panic(err)
								}
								if len(sz) != 1 {
									panic("bad stream length")
								}

								v4stream := sz[0]
								_, err = v4stream.DeleteRange(context.Background(), AllowedStart, AllowedEnd)
								if err != nil {
									panic(err)
								}
					*/
				}
			} else {
				fmt.Printf("ABORT: unexpected error %v\n", cerr)
				os.Exit(1)
			}
		}
		deststreams = append(deststreams, v4stream)
	}

	for sidx, s := range streams {
		go func(idx int, s Stream) {
			v4stream := deststreams[idx]
			sv := Query(from, s.CC, s.T, starttime, endtime)
			buf := make([]v4.RawPoint, 0, 2000)
			for v := range sv {
				buf = append(buf, v)
				if len(buf) >= 2000 {
					err := v4stream.Insert(context.Background(), buf)
					if err != nil {
						fmt.Printf("got error %v\n", err)
					}
					buf = buf[:0]
					// err = v4stream.Flush(context.Background())
					// if err != nil {
					// 	fmt.Printf("got error %v\n", err)
					// }
				}
				bars[idx].Increment()
			}
			if len(buf) > 0 {
				err := v4stream.Insert(context.Background(), buf)
				if err != nil {
					fmt.Printf("got error %v\n", err)
				}
			}
			bars[idx].Finish()
			wg.Done()
		}(sidx, s)
	}
	wg.Wait()
	pool.Stop()
}
