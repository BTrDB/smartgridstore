package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/ceph/go-ceph/rados"

	logging "github.com/op/go-logging"
)

var logger *logging.Logger

func init() {
	logger = logging.MustGetLogger("log")
}

const ReLookupInterval = 30 * time.Minute

type cacheEntry struct {
	alias string
	found bool
	time  time.Time
}

var aliasCacheMu sync.Mutex
var aliasCache map[string]cacheEntry

var FAILUREMSG = make([]byte, 4, 4)

func lookupAlias(serial string) (string, bool) {
	rh := <-rhPool
	defer func() { rhPool <- rh }()
	rv, err := rh.GetOmapValues("meta.aliases", "", serial, 1)
	if err != nil {
		if err == rados.RadosErrorNotFound {
			return "", false
		}
		logger.Panicf("Got alias lookup error: %v", err)
	}
	real_rv, ok := rv[serial]
	if ok {
		return string(real_rv), true
	}
	return "", false
}

func lookupAliasCache(serial string) string {
	aliasCacheMu.Lock()
	defer aliasCacheMu.Unlock()
	ce, ok := aliasCache[serial]
	if ok {
		if ce.found {
			return ce.alias
		}
		if time.Now().Sub(ce.time) > ReLookupInterval {
			al, ok := lookupAlias(serial)
			aliasCache[serial] = cacheEntry{alias: al, found: ok, time: time.Now()}
			if ok {
				return al
			}
			return "? (" + serial + ")"
		}
	}
	al, ok := lookupAlias(serial)
	aliasCache[serial] = cacheEntry{alias: al, found: ok, time: time.Now()}
	if ok {
		return al
	}
	return "? (" + serial + ")"
}

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

func processMessage(sendid []byte, ip string, sernum string, filepath string, data []byte) []byte {

	rh := <-rhPool
	defer func() { rhPool <- rh }()

	objname := fmt.Sprintf("data.psl.pqube3.%s.%s", sernum, filepath)

	err := rh.SetOmap("meta.master", map[string][]byte{objname: []byte(objname)})
	if err != nil {
		logger.Panicf("Could not write master entry: %v", err)
	}

	err = rh.SetOmap(fmt.Sprintf("meta.gen.%d", Generation), map[string][]byte{objname: []byte(objname)})
	if err != nil {
		logger.Panicf("Could not write gen %d entry: %v", Generation, err)
	}

	lastheardrec := fmt.Sprintf("%d,%d", time.Now().UnixNano(), ip)
	err = rh.SetOmap("meta.lastheard", map[string][]byte{sernum: []byte(lastheardrec)})
	if err != nil {
		logger.Panicf("Could not write last heard entry:%v", err)
	}

	err = rh.WriteFull(objname, data)
	if err != nil {
		logger.Panicf("Could not write to ceph: %v", err)
	}

	// Database was successfully updated
	return sendid
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
	var resp []byte
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
					if sernum != "" && newsernum != sernum {
						fmt.Printf("WARNING: serial number changed from %s to %s\n", sernum, newsernum)
						fmt.Println("Updating serial number for next write")
					}
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
					alias := lookupAliasCache(sernum)
					recvdfull = true
					fmt.Printf("Received %s: serial number is %s (%s), length is %v\n", filepath, sernum, alias, lendt)
					resp = processMessage(sendid, conn.RemoteAddr().String(), sernum, filepath, dtbuffer[:lendt])
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
	//Load variables
	prt := os.Getenv("RECEIVER_PORT")
	if prt == "" {
		Port = 1883
	} else {
		p, err := strconv.ParseInt(prt, 10, 64)
		if err != nil {
			logger.Panicf("Could not parse port: %v", err)
		}
		Port = int(p)
	}

	Pool := os.Getenv("RECEIVER_POOL")
	if Pool == "" {
		Pool = "receiver"
	}

	gen := os.Getenv("RECEIVER_GENERATION")
	if gen == "" {
		Generation = 1
	} else {
		g, err := strconv.ParseInt(gen, 10, 64)
		if err != nil {
			logger.Panicf("Could not parse generation: %v", err)
		}
		Generation = int(g)
	}

	aliasCache = make(map[string]cacheEntry)

	conn, err := rados.NewConn()
	if err != nil {
		logger.Panicf("Could not initialize ceph storage: %v", err)
	}
	err = conn.ReadDefaultConfigFile()
	if err != nil {
		logger.Panicf("Could not read ceph config: %v", err)
	}
	err = conn.Connect()
	if err != nil {
		logger.Panicf("Could not initialize ceph storage: %v", err)
	}

	rhPool = make(chan *rados.IOContext, NUM_RHANDLES+1)

	for i := 0; i < NUM_RHANDLES; i++ {
		h, err := conn.OpenIOContext(Pool)
		if err != nil {
			logger.Panicf("Could not open ceph handle", err)
		}
		rhPool <- h
	}

	var bindaddr *net.TCPAddr
	var listener *net.TCPListener

	bindaddr, err = net.ResolveTCPAddr("tcp", fmt.Sprintf("0.0.0.0:%v", Port))
	if err != nil {
		logger.Panicf("Could not resolve address to bind TCP server socket: %v\n", err)

	}
	listener, err = net.ListenTCP("tcp", bindaddr)
	if err != nil {
		logger.Panicf("Could not create bound TCP server socket: %v\n", err)
	}

	logger.Infof("Waiting for incoming connections...")

	var upmuconn *net.TCPConn
	for {
		upmuconn, err = listener.AcceptTCP()
		if err == nil {
			go handlePMUConn(upmuconn)
		} else {
			logger.Warningf("Could not accept incoming TCP connection: %v\n", err)
		}
	}
}
