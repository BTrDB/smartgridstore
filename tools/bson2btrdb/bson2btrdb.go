package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/immesys/smartgridstore/tools/inserter"

	btrdb "gopkg.in/btrdb.v4"
	"gopkg.in/ini.v1"
	"gopkg.in/mgo.v2/bson"
)

var bufferPool = sync.Pool{}
var done chan struct{}

var lastProcessed int64

// StateFileName is the name of the state file storing the offset into the file
const StateFileName = ".bson2btrdb"

// BkupStateFileName is a backup file used if the program crashes while writing a file
const BkupStateFileName = ".bson2btrdb.bkup"

type dataInsertRequest struct {
	data   []byte
	offset int64
}

var serial2path *ini.Section

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s <BSON file>\n", os.Args[0])
		return
	}

	filename := os.Args[1]
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("Could not open %s: %v", filename, err)
	}

	cfg, err := ini.Load("serial2path.ini")
	if err != nil {
		log.Fatalf("Could not load serial2path.ini: %v", err)
	}
	serial2path = cfg.Section("")

	runtime.GOMAXPROCS(runtime.NumCPU())

	bc, err := btrdb.Connect(context.Background(), btrdb.EndpointsFromEnv()...)
	if err != nil {
		log.Fatalf("Could not connect to BTrDB: %v", err)
	}

	var numworkers int
	numworkersstr := os.Getenv("NUM_WORKERS")
	if numworkersstr != "" {
		numworkers, err = strconv.Atoi(numworkersstr)
		if err != nil {
			log.Fatalf("$NUM_WORKERS must be an integer (got %s)", numworkersstr)
		}
	} else {
		numworkers = runtime.NumCPU() << 3
		log.Printf("$NUM_WORKERS not set; using %d", numworkers)
	}

	reqchan := make(chan dataInsertRequest, numworkers<<2)

	var wg sync.WaitGroup
	wg.Add(numworkers)
	for i := 0; i != numworkers; i++ {
		go unmarshalAndProcess(reqchan, bc, &wg)
	}

	statefile, err := os.OpenFile(StateFileName, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		log.Fatalf("Could not open state file: %v", err)
	}
	var offset int64
	err = binary.Read(statefile, binary.LittleEndian, &offset)
	if err != io.EOF {
		if err != nil {
			log.Fatalf("Could not read state file: %v", err)
		}
		_, err = file.Seek(offset, io.SeekStart)
		if err != nil {
			log.Fatalf("Could not reach expected offset (%v) in file: %v", offset, err)
		}
	}
	err = statefile.Close()
	if err != nil {
		log.Fatalf("Could not close state file: %v", err)
	}

	go periodicallyUpdateStateFile()

	lastProcessed = offset

	signals := make(chan os.Signal, 1)
	done = make(chan struct{})

	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for sig := range signals {
			log.Printf("Got signal %v", sig)
			done <- struct{}{}
		}
	}()

	for {
		select {
		case <-done:
			log.Println("Terminating...")
			break
		}

		var buffer []byte
		bufferint := bufferPool.Get()
		if bufferint == nil {
			buffer = make([]byte, 65536)
		} else {
			buffer = bufferint.([]byte)
		}

		sizebuf := buffer[:4]
		c, err := io.ReadFull(file, sizebuf)
		if c == 0 && err == io.EOF {
			log.Println("Finished processing file. Terminating...")
			break
		} else if err != nil {
			log.Printf("Unexpected failure to read from file: %v", err)
			break
		}

		size := int32(binary.LittleEndian.Uint32(sizebuf))
		_, err = io.ReadFull(file, buffer[4:size])
		if err != nil {
			log.Printf("Unexpected failure to read from file: %v", err)
			break
		}

		off, err := file.Seek(0, io.SeekCurrent)
		if err != nil {
			log.Fatalf("Could not get current file position: %v", err)
		}

		reqchan <- dataInsertRequest{
			data:   buffer,
			offset: off,
		}
	}

	log.Println("Waiting for remaining files to be processed...")
	wg.Done()
}

func updateStateFile(filename string, off int64) {
	statefile, err := os.OpenFile(BkupStateFileName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		log.Fatalf("Could not open state file: %v", err)
	}
	err = binary.Write(statefile, binary.LittleEndian, off)
	if err != nil {
		log.Fatalf("Could not write to state file: %v", err)
	}
	err = statefile.Close()
	if err != nil {
		log.Fatalf("Could not close state file: %v", err)
	}
}

func periodicallyUpdateStateFile() {
	for {
		off := atomic.LoadInt64(&lastProcessed)

		/* Maintain two files, in case we crash while writing to one of them... */
		updateStateFile(StateFileName, off)
		updateStateFile(BkupStateFileName, off)

		time.Sleep(time.Second)
	}
}

// UpmuDocument represents a deserialized uPMU document.
type UpmuDocument struct {
	ID           bson.ObjectId `bson:"_id"`
	Data         bson.Binary   `bson:"data"`
	Published    bool          `bson:"published"`
	SerialNumber string        `bson:"serial_number"`
	TimeReceived string        `bson:"time_received"`
	Xtag         float64       `bson:"xtag"`
	Ytag         float64       `bson:"ytag"`
}

func unmarshalAndProcess(c chan dataInsertRequest, b *btrdb.BTrDB, wg *sync.WaitGroup) {
	var doc UpmuDocument
	for req := range c {
		doc.SerialNumber = ""
		bson.Unmarshal(req.data, &doc)
		if doc.SerialNumber != "" {
			inserter.ProcessMessage(context.Background(), doc.SerialNumber, doc.Data.Data, b, serialToPath)
		}
		bufferPool.Put(req.data)

		atomic.StoreInt64(&lastProcessed, req.offset)
	}
	wg.Done()
}

func serialToPath(ctx context.Context, sernum string) string {
	if serial2path.HasKey(sernum) {
		pathkey := serial2path.Key(sernum)
		return pathkey.Value()
	}
	return fmt.Sprintf("upmu/%s", sernum)
}
