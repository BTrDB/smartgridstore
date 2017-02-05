package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	etcd "github.com/coreos/etcd/clientv3"
	"github.com/immesys/smartgridstore/tools/manifest"
	"github.com/immesys/smartgridstore/tools/upmuparser"
	"github.com/pborman/uuid"

	btrdb "gopkg.in/btrdb.v4"
)

const UpmuSpaceString = "c9bbebff-ff40-4dbe-987e-f9e96afb7a57"

var UpmuSpace = uuid.Parse(UpmuSpaceString)
var bc *btrdb.BTrDB
var ec *etcd.Client

func descriptorFromSerial(serial string) string {
	return strings.ToLower(fmt.Sprintf("psl.pqube3.%s", serial))
}

func getUUID(serial string, streamname string) uuid.UUID {
	streamid := fmt.Sprintf("%v.%v", descriptorFromSerial(serial), streamname)
	return uuid.NewSHA1(UpmuSpace, []byte(streamid))
}

func processMessage(ctx context.Context, sernum string, data []byte) bool {
	parsed, err := upmuparser.ParseSyncOutArray(data)
	if err != nil {
		log.Printf("Could not parse data from %v: %v", sernum, err)
		return false
	}

	// Get the data that we have to insert and put it in this format
	toinsert := make([][]btrdb.RawPoint, len(upmuparser.STREAMS))
	for _, synco := range parsed {
		igs := synco.GetInsertGetters()
		timeArr := synco.Times()
		timestamp := time.Date(int(timeArr[0]), time.Month(timeArr[1]), int(timeArr[2]), int(timeArr[3]), int(timeArr[4]), int(timeArr[5]), 0, time.UTC).UnixNano()
		for sid, ig := range igs {
			t := timestamp
			for i := 0; i != upmuparser.ReadingsPerStruct; i++ {
				t += int64(1000000000) / upmuparser.ReadingsPerStruct
				v := ig(i, synco)
				toinsert[sid] = append(toinsert[sid], btrdb.RawPoint{Time: t, Value: v})
			}
		}
	}

	var dev *manifest.ManifestDevice

	// Now let's go ahead and insert the data
	for sid, dataset := range toinsert {
		// Some uPMUs don't have all streams
		if len(dataset) == 0 {
			continue
		}

		uu := getUUID(sernum, upmuparser.STREAMS[sid])
		s := bc.StreamFromUUID(uu)
		ex, err := s.Exists(ctx)
		if err != nil {
			log.Fatalf("Could not check if stream exists in BTrDB: %v", err)
		}
		if !ex {
			// We need to actually create the stream. First, we query the path from etcd
			desc := descriptorFromSerial(sernum)

			if dev == nil {
				dev, err = manifest.RetrieveManifestDevice(ctx, ec, desc)
				if err != nil {
					log.Fatalf("Could not check for device path in etcd: %v", err)
				}
			}

			var path string
			if dev != nil {
				var ok bool
				path, ok = dev.Metadata["path"]
				if !ok {
					log.Printf("Device %v is missing the path metadata; falling back to descriptor", desc)
					path = desc
				}
			} else {
				log.Printf("No manifest device info for %v was found; falling back to descriptor", desc)
				path = desc
			}
			s, err = bc.Create(ctx, uu, strings.ToLower(path), map[string]string{"name": upmuparser.STREAMS[sid]}, []byte{})
			if err != nil {
				log.Fatalf("Could not create stream in BTrDB: %v", err)
			}
		}

		err = s.Insert(ctx, dataset)
		if err != nil {
			log.Fatalf("Could not insert stream %v of %v into BTrDB: %v", upmuparser.STREAMS[sid], descriptorFromSerial(sernum), err)
		}
	}

	return true
}
