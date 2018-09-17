package gen2ingress

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/BTrDB/smartgridstore/tools"
	"github.com/BTrDB/smartgridstore/tools/manifest"
	etcd "github.com/coreos/etcd/clientv3"
	"github.com/pborman/uuid"
	btrdb "gopkg.in/BTrDB/btrdb.v4"
)

//This is called by the main method of the driver-specific executable
func Gen2Ingress(driver Driver) {
	if len(os.Args) == 2 && os.Args[1] == "-version" {
		fmt.Printf("%d.%d.%d\n", tools.VersionMajor, tools.VersionMinor, tools.VersionPatch)
		os.Exit(0)
	}
	fmt.Printf("Booting gen2 ingress version %d.%d.%d\n", tools.VersionMajor, tools.VersionMinor, tools.VersionPatch)

	manifest.SetEtcdKeyPrefix("")

	var etcdEndpoint string = os.Getenv("ETCD_ENDPOINT")
	if len(etcdEndpoint) == 0 {
		etcdEndpoint = "localhost:2379"
		log.Printf("ETCD_ENDPOINT is not set; using %s", etcdEndpoint)
	}

	var etcdConfig etcd.Config = etcd.Config{Endpoints: []string{etcdEndpoint}}

	log.Println("Connecting to etcd...")
	etcdConn, err := etcd.New(etcdConfig)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	defer etcdConn.Close()
	log.Println("Connecting to BTrDB...")
	btrdbconn, err := btrdb.Connect(context.Background(), btrdb.EndpointsFromEnv()...)
	if err != nil {
		fmt.Printf("Error connecting to the BTrDB: %v\n", err)
		return
	}

	defer func() {
		err := btrdbconn.Disconnect()
		if err == nil {
			fmt.Println("Finished closing connection")
		} else {
			fmt.Printf("Could not close connection: %v\n", err)
		}
	}()
	log.Println("Connected")

	ourid := uuid.NewRandom().String()

	insert := NewInserter(btrdbconn)
	driver.SetConn(insert)

	//Do we even need to lock anything in the manifest table
	if !driver.InitiatesConnections() {
		for {
			time.Sleep(5 * time.Second)
		}
	}

	ourActiveDevices := make(map[string]context.CancelFunc)

	//Get the lock table
devloop:
	for {
		time.Sleep(3 * time.Second)
		devs, err := manifest.RetrieveMultipleManifestDevices(context.Background(), etcdConn, driver.DIDPrefix())
		if err != nil {
			panic(err)
		}
		copyActiveDevices := make(map[string]context.CancelFunc)
		for k, v := range ourActiveDevices {
			copyActiveDevices[k] = v
		}
		for _, d := range devs {
			delete(copyActiveDevices, d.Descriptor)
		}

		//Any entries left in the copy are devices that have been removed from the
		//descriptor table
		for id, cancel := range copyActiveDevices {
			shortform := manifest.GetDescriptorShortForm(id)
			fmt.Printf("[%s] Device was removed from manifest. Stopping processing\n", shortform)
			cancel()
			delete(ourActiveDevices, id)
		}

		//Check which of the devices are locked
		ltable, err := manifest.GetLockTable(context.Background(), etcdConn, driver.DIDPrefix())
		us, min2, locked := parseLTable(ltable, ourid)
		//If we already have more than the two minimum nodes, then don't
		//take on any new nodes
		if us > min2 && min2 != -1 {
			//Do not start doing a new device if we are not doing a small number
			//of devices
			continue
		}
		fmt.Printf("We have a total of %d devices locked\n", us)
		// We need to start a device
		for _, d := range devs {
			if locked[d.Descriptor] {
				continue
			}
			identifier := d.Descriptor
			ctx, cancel := context.WithCancel(context.Background())
			gotlock, err := manifest.ObtainDeviceLock(ctx, etcdConn, d, ourid, driver.DIDPrefix())
			if err != nil {
				panic(err)
			}
			if gotlock {
				shortform := manifest.GetDescriptorShortForm(identifier)
				fmt.Printf("[%s] We locked device and are started processing\n", shortform)
				ourActiveDevices[identifier] = cancel
				err := driver.HandleDevice(ctx, identifier)
				if err != nil {
					fmt.Printf("[%s] device configuration error: %v\n", shortform, err)
				}
				continue devloop
			} else {
				cancel()
			}
		}
	}
}

func parseLTable(t map[string][]string, ourid string) (int, int, map[string]bool) {
	min := -1
	min2 := -1
	us := 0
	locked := make(map[string]bool)
	for k, v := range t {
		//k is a node, v is a list of dids
		if len(v) <= min || min == -1 {
			min2 = min
			min = len(v)
		}
		if k == ourid {
			us = len(v)
		}
		for _, vi := range v {
			locked[vi] = true
		}
	}
	return us, min2, locked
}

func DialLoop(ctx context.Context, target string, descriptor string, f DialProcessFunction) {
	shortform := manifest.GetDescriptorShortForm(descriptor)
	for {
		if ctx.Err() != nil {
			fmt.Printf("[%s] context cancelled\n", shortform)
			return
		}
		fmt.Printf("[%s] beginning dial of %s\n", shortform, target)
		sctx, cancel := context.WithCancel(ctx)
		err := dial(sctx, descriptor, target, f)
		cancel()
		fmt.Printf("[%s] fatal error: %v\n", shortform, err)
		if ctx.Err() != nil {
			return
		}
		time.Sleep(10 * time.Second)
		fmt.Printf("[%s] backoff over, reconnecting\n", shortform)
	}
}
func Loop(ctx context.Context, descriptor string, f CustomProcessFunction) {
	shortform := manifest.GetDescriptorShortForm(descriptor)
	for {
		if ctx.Err() != nil {
			fmt.Printf("[%s] context cancelled\n", shortform)
			return
		}
		fmt.Printf("[%s] beginning connect\n", shortform)
		sctx, cancel := context.WithCancel(ctx)
		err := custom(sctx, descriptor, f)
		cancel()
		fmt.Printf("[%s] fatal error: %v\n", shortform, err)
		time.Sleep(10 * time.Second)
		fmt.Printf("[%s] backoff over, reconnecting\n", shortform)
	}
}
func custom(ctx context.Context, descriptor string, f CustomProcessFunction) (err error) {
	shortform := manifest.GetDescriptorShortForm(descriptor)
	//TODO set back
	// defer func() {
	// 	r := recover()
	// 	if r != nil {
	// 		err = fmt.Errorf("[%s] panic: %v", shortform, r)
	// 	}
	// }()
	_ = shortform
	return f(ctx)
}
func dial(ctx context.Context, descriptor string, target string, f DialProcessFunction) (err error) {
	shortform := manifest.GetDescriptorShortForm(descriptor)
	var conn *net.TCPConn
	defer func() {
		r := recover()
		if r != nil {
			conn.Close()
			err = fmt.Errorf("[%s] panic: %v", shortform, r)
		}
	}()
	addr, err := net.ResolveTCPAddr("tcp", target)
	if err != nil {
		return err
	}
	conn, err = net.DialTCP("tcp", nil, addr)
	if err != nil {
		return err
	}
	fmt.Printf("[%s] dial succeeded\n", shortform)
	br := bufio.NewReader(conn)
	return f(ctx, conn, br)
}
