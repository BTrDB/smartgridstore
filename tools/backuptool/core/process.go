package core

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"sync"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/howeyc/gopass"
	"github.com/immesys/go-ceph/rados"
	"github.com/urfave/cli"
)

var gframewriter *FrameWriter
var goutputdir string
var gincrementaldir string
var gdoincrement bool
var gcephconn *rados.Conn
var gpassphrase string
var gpoolstobackup []string
var gsharder *Sharder
var getcd *clientv3.Client
var gpoolsmap map[string]string

var gtimestampsmu sync.Mutex
var gtimestamps map[MK]int64
var ginctimestamps map[MK]int64

func Create(c *cli.Context) error {
	gtimestamps = make(map[MK]int64)
	if c.Bool("skip-etcd") && c.Bool("skip-ceph") {
		fmt.Printf("you can't skip both ceph and etcd\n")
		os.Exit(1)
	}
	if !c.Bool("skip-etcd") {
		connectEtcd(c)
	}
	getPassphrase()
	createFrameWriter()
	createSharder(c)
	if !c.Bool("skip-ceph") {
		connectCeph(c)
		checkPoolsExist(c.StringSlice("skip-pool"), c.StringSlice("include-pool"))
	}
	if !c.Bool("skip-ceph") {
		backupCeph()
	} else {
		fmt.Printf("skipping ceph backup\n")
	}
	if !c.Bool("skip-etcd") {
		backupEtcd()
	} else {
		fmt.Printf("skipping etcd backup\n")
	}
	writeManifest()
	return nil
}

func connectEtcd(c *cli.Context) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{c.String("etcd")},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		fmt.Printf("could not connect to etcd: %v\n", err)
		os.Exit(1)
	}
	getcd = cli
}
func backupEtcd() {
	count := 0
	batch := []KeyValue{}
	resp, err := getcd.Get(context.Background(), "", clientv3.WithPrefix())
	ChkErr(err)
	for _, kv := range resp.Kvs {
		count++
		batch = append(batch, KeyValue{Key: string(kv.Key), Value: kv.Value})
		if len(batch) > 100 {
			gsharder.Write(&EtcdRecords{KVz: batch})
			batch = []KeyValue{}
		}

	}
	if len(batch) > 0 {
		gsharder.Write(&EtcdRecords{KVz: batch})
	}
	if resp.More {
		ChkErr(fmt.Errorf("etcd did not give us all the records"))
	}
	fmt.Printf("ETCD DONE; included %d etcd records\n", count)
}
func createSharder(c *cli.Context) {
	goutputdir = c.String("outputdir")
	gincrementaldir = c.String("incremental-over")
	gdoincrement = gincrementaldir != ""
	ReadIncrementalMetadata(gframewriter, gincrementaldir)
	gsharder = NewWriteSharder(gframewriter)
}
func createReadSharder(c *cli.Context) {
	goutputdir = c.String("outputdir")
	gsharder = NewReadSharder(gframewriter, goutputdir)
}

type backupEntry struct {
	Namespace string
	OID       string
	Pool      string
}

func backupCeph() {
	count := 0
	incskip := 0
	fmt.Printf("this backup will include %d pools:\n", len(gpoolstobackup))
	incpf, err := os.Create(path.Join(goutputdir, "pools.txt"))
	ChkErr(err)
	for _, pool := range gpoolstobackup {
		fmt.Printf(" - %s\n", pool)
		incpf.WriteString(fmt.Sprintf("%s\n", pool))
	}
	ChkErr(incpf.Close())

	const ibufsz = 50 * 1024 * 1024
	todo := make(chan backupEntry, 50)
	towrite := make(chan Container, 50)
	go func() {
		for _, pool := range gpoolstobackup {
			ioctx, err := gcephconn.OpenIOContext(pool)
			ChkErr(err)
			ioctx.SetNamespace(rados.RadosAllNamespaces)
			iter, err := ioctx.Iter()
			ChkErr(err)
			for iter.Next() {
				ChkErr(iter.Err())
				todo <- backupEntry{
					Namespace: iter.Namespace(),
					OID:       iter.Value(),
					Pool:      pool,
				}
			}
			ioctx.Destroy()
		}
		close(todo)
	}()

	wg := sync.WaitGroup{}
	const numWorkers = 8
	wg.Add(numWorkers)
	for worker := 0; worker < numWorkers; worker++ {
		go func() {
			poolconns := make(map[string]*rados.IOContext)
			buf := make([]byte, ibufsz)
			for rec := range todo {
				buf = buf[0:ibufsz]
				oid := rec.OID
				ns := rec.Namespace
				ioctx2, ok := poolconns[rec.Pool]
				if !ok {
					var err error
					ioctx2, err = gcephconn.OpenIOContext(rec.Pool)
					ChkErr(err)
					poolconns[rec.Pool] = ioctx2
				}
				ioctx2.SetNamespace(ns)
				//stat it first
				so, err := ioctx2.Stat(oid)
				ChkErr(err)
				if gdoincrement {
					k := MK{rec.Pool, ns, oid}
					lastmtime, ok := ginctimestamps[k]
					if ok && lastmtime == so.ModTime.UnixNano() {
						incskip++
						continue
					}
				}
				gtimestampsmu.Lock()
				gtimestamps[MK{rec.Pool, ns, oid}] = so.ModTime.UnixNano()
				gtimestampsmu.Unlock()
				//get real content
				n, err := ioctx2.Read(oid, buf, 0)
				ChkErr(err)
				if n == ibufsz {
					panic("found object >50MB in rados")
				}
				buf = buf[:n]

				//get xattr
				xattrs, err := ioctx2.ListXattrs(oid)
				ChkErr(err)

				//get omaps
				omaps, err := ioctx2.GetAllOmapValues(oid, "", "", 1000)
				ChkErr(err)

				container := CephObject{}
				for k, v := range xattrs {
					container.XATTRData = append(container.XATTRData, KeyValue{Key: k, Value: v})
				}
				for k, v := range omaps {
					container.OMAPData = append(container.OMAPData, KeyValue{Key: k, Value: v})
				}
				container.Content = buf
				container.Namespace = ns
				container.Pool = rec.Pool
				container.Name = oid
				count++
				towrite <- &container
			}
			wg.Done()
		}()
	}
	go func() {
		wg.Wait()
		close(towrite)
	}()
	containers := 0
	for rec := range towrite {
		gsharder.Write(rec)
		containers++
		if containers%1000 == 0 {
			fmt.Printf(" processed %d K objects\n", containers/1000)
		}
	}
	fmt.Printf("CEPH DONE; included %d ceph objects (skipped %d unchanged)\n", containers, incskip)
}

func writeManifest() {
	gsharder.WriteMeta(gsharder.Metadata())
	gsharder.Close()
}

func ChkErr(err error) {
	if err != nil {
		fmt.Printf("unexpected error: %v\n", err)
		os.Exit(1)
	}
}

func checkPoolsExist(skip []string, include []string) {
	pools := listPools()
	gpoolstobackup = []string{}
	for _, include := range include {
		found := false
		for _, existing := range pools {
			if existing == include {
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("ceph pool %q does not exist, aborting\n", include)
			os.Exit(1)
		}
		gpoolstobackup = append(gpoolstobackup, include)
	}
	for _, existing := range pools {
		found := false
		for _, skip := range skip {
			if skip == existing {
				found = true
				break
			}
		}
		if !found && len(include) == 0 {
			gpoolstobackup = append(gpoolstobackup, existing)
		}
	}
}

func connectCeph(c *cli.Context) {
	conn, err := rados.NewConn()
	ChkErr(err)
	err = conn.ReadConfigFile(c.String("cephconf"))
	ChkErr(err)
	err = conn.Connect()
	ChkErr(err)
	gcephconn = conn
}

func createFrameWriter() {
	fw := NewFrameWriter(gpassphrase)
	gframewriter = fw
}

type ProcessArgs struct {
	CephConf      string
	PoolsToBackup []string
}

func getPassphrase() {
	fmt.Printf("please enter the encryption passphrase: ")
	rv, err := gopass.GetPasswdMasked()
	ChkErr(err)
	fmt.Printf("same again: ")
	rv2, err := gopass.GetPasswdMasked()
	ChkErr(err)
	if !bytes.Equal(rv, rv2) {
		ChkErr(fmt.Errorf("passphrases did not match"))
	}
	gpassphrase = string(rv)
}

func listPools() []string {
	names, err := gcephconn.ListPools()
	ChkErr(err)
	return names
}
