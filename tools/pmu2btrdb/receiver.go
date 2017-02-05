package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	btrdb "gopkg.in/btrdb.v4"

	"github.com/ceph/go-ceph/rados"
	etcd "github.com/coreos/etcd/clientv3"
)

const VersionMajor = 4
const VersionMinor = 0
const VersionPatch = 0

const ReLookupInterval = 30 * time.Minute

type cacheEntry struct {
	alias string
	found bool
	time  time.Time
}

var aliasCacheMu sync.Mutex
var aliasCache map[string]cacheEntry

var FAILUREMSG = make([]byte, 4, 4)

var rhPool chan *rados.IOContext

var Port = 1883

var Generation = 1

const (
	CONNBUFLEN            = 1024 // number of bytes we read from the connection at a time
	MAXFILEPATHLEN        = 512
	MAXSERNUMLEN          = 32
	EXPDATALEN            = 757440
	MAXDATALEN            = 75744000
	MAXCONCURRENTSESSIONS = 16
	TIMEOUTSECS           = 30
	NUM_RHANDLES          = 16
)

func roundUp4(x uint32) uint32 {
	return (x + 3) & 0xFFFFFFFC
}

func handlePMUConn(conn *net.TCPConn) {
	fmt.Printf("Connected: %s\n", conn.RemoteAddr().String())

	defer conn.Close()

	/* Stores error on failed read from TCP connection. */
	var err error

	/* Stores error on failed write to TCP connection. */
	var erw error

	/* The id of a message is 4 bytes long. */
	var sendid []byte

	/* The length of the filepath. */
	var lenfp uint32
	/* The length of the filepath, including the padding added so it ends on a word boundary. */
	var lenpfp uint32

	/* The length of the serial number. */
	var lensn uint32
	/* The length of the serial number, including the padding added so it ends on a word boundary. */
	var lenpsn uint32

	/* The length of the data. */
	var lendt uint32

	/* Read from the Connection 1 KiB at a time. */
	var buf = make([]byte, CONNBUFLEN)
	var bpos int

	/* INFOBUFFER stores length data from the beginning of the message to get the length of the rest. */
	var infobuffer [16]byte
	var ibindex uint32

	/* FPBUFFER stores the filepath data received so far. */
	var fpbuffer = make([]byte, MAXFILEPATHLEN, MAXFILEPATHLEN)
	var fpindex uint32
	var filepath string

	/* SNBUFFER stores the part of the serial number received so far. */
	var snbuffer = make([]byte, MAXSERNUMLEN, MAXSERNUMLEN)
	var snindex uint32
	var sernum string
	var newsernum string

	/* DTBUFFER stores the part of the uPMU data received so far.
	   If a file is bigger than expected, we allocate a bigger buffer, specially for that file. */
	var dtbufferexp = make([]byte, EXPDATALEN, EXPDATALEN)
	var dtbuffer []byte = nil
	var dtindex uint32

	/* N stores the number of bytes read in the current read from the TCP connection. */
	var n int
	/* TOTRECV stores the total number of bytes read from the TCP connection for this message. */
	var totrecv uint32

	/* The response to send to the uPMU. */
	var recvdfull bool

	// Infinite loop to keep reading messages until connection is closed
	for {
		ibindex = 0
		fpindex = 0
		snindex = 0
		dtindex = 0
		totrecv = 0
		recvdfull = false
		// Read and process the message
		for !recvdfull {
			n, err = conn.Read(buf)
			bpos = 0
			if bpos < n && totrecv < 16 {
				for ibindex < 16 && bpos < n {
					infobuffer[ibindex] = buf[bpos]
					ibindex++
					bpos++
					totrecv++
				}
				if ibindex == 16 {
					sendid = infobuffer[:4]
					lenfp = binary.LittleEndian.Uint32(infobuffer[4:8])
					lensn = binary.LittleEndian.Uint32(infobuffer[8:12])
					lendt = binary.LittleEndian.Uint32(infobuffer[12:16])
					lenpfp = roundUp4(lenfp)
					lenpsn = roundUp4(lensn)
					if lenfp != 0 && lenfp > MAXFILEPATHLEN {
						fmt.Printf("Filepath length fails sanity check: %v\n", lenfp)
						return
					}
					if lensn != 0 && lensn > MAXSERNUMLEN {
						fmt.Printf("Serial number length fails sanity check: %v\n", lensn)
						return
					}
					if lendt != 0 && lendt > MAXDATALEN {
						fmt.Printf("Data length fails sanity check: %v\n", lendt)
						return
					}
					if lendt <= EXPDATALEN {
						dtbuffer = dtbufferexp
					} else {
						dtbuffer = make([]byte, lendt, lendt)
					}
				}
			}
			if bpos < n && totrecv < 16+lenpfp {
				for fpindex < lenpfp && bpos < n {
					fpbuffer[fpindex] = buf[bpos]
					fpindex++
					bpos++
					totrecv++
				}
				if fpindex == lenpfp {
					filepath = string(fpbuffer[:lenfp])
				}
			}
			if bpos < n && totrecv < 16+lenpfp+lenpsn {
				for snindex < lenpsn && bpos < n {
					snbuffer[snindex] = buf[bpos]
					snindex++
					bpos++
					totrecv++
				}
				if snindex == lenpsn {
					newsernum = string(snbuffer[:lensn])
					// we used to warn if the serial number changed from previous received frame
					sernum = newsernum
				}
			}
			if bpos < n && totrecv < 16+lenpfp+lenpsn+lendt {
				for dtindex < lendt && bpos < n {
					dtbuffer[dtindex] = buf[bpos]
					dtindex++
					bpos++
					totrecv++
				}
				if dtindex == lendt {
					if bpos < n {
						fmt.Printf("WARNING: got %d extra bytes\n", n-bpos)
					}
					// if we've reached this point, we have all the data
					recvdfull = true
					fmt.Printf("Received %s: serial number is %s, length is %v\n", filepath, sernum, lendt)
					success := processMessage(context.TODO(), sernum, dtbuffer[:lendt])

					resp := sendid
					if !success {
						resp = make([]byte, 4)
					}
					_, erw = conn.Write(resp)
					if erw != nil {
						fmt.Printf("Connection lost: %v (write failed: %v)\n", conn.RemoteAddr().String(), erw)
						return
					}
				}
			}
			if err != nil {
				fmt.Printf("Connection lost: %v (reason: %v)\n", conn.RemoteAddr().String(), err)
				return
			}
		}
	}
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "-version" {
		fmt.Printf("%d.%d.%d\n", VersionMajor, VersionMinor, VersionPatch)
		os.Exit(0)
	}
	fmt.Printf("Booting pmu2btrdb version %d.%d.%d\n", VersionMajor, VersionMinor, VersionPatch)
	//Load variables
	prt := os.Getenv("RECEIVER_PORT")
	if prt == "" {
		Port = 1883
	} else {
		p, err := strconv.ParseInt(prt, 10, 64)
		if err != nil {
			log.Fatalf("Could not parse port: %v", err)
		}
		Port = int(p)
	}

	var err error

	bc, err = btrdb.Connect(context.TODO(), btrdb.EndpointsFromEnv()...)
	if err != nil {
		log.Fatalf("Could not connect to BTrDB: %v", err)
	}

	etcdEndpoint := os.Getenv("ETCD_ENDPOINT")
	if len(etcdEndpoint) == 0 {
		etcdEndpoint = "localhost:2379"
		log.Printf("ETCD_ENDPOINT is not set; using %s", etcdEndpoint)
	}

	ec, err = etcd.New(etcd.Config{
		Endpoints:   []string{etcdEndpoint},
		DialTimeout: 10 * time.Second,
	})
	if err != nil {
		log.Fatalf("Could not connect to etcd: %v\n", err)
	}

	bindaddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("0.0.0.0:%v", Port))
	if err != nil {
		log.Fatalf("Could not resolve address to bind TCP server socket: %v\n", err)

	}
	listener, err := net.ListenTCP("tcp", bindaddr)
	if err != nil {
		log.Fatalf("Could not create bound TCP server socket: %v\n", err)
	}

	log.Println("Waiting for incoming connections...")

	var upmuconn *net.TCPConn
	for {
		upmuconn, err = listener.AcceptTCP()
		if err == nil {
			go handlePMUConn(upmuconn)
		} else {
			log.Printf("Could not accept incoming TCP connection: %v\n", err)
		}
	}
}
