// Copyright (c) 2021 Michael Andersen
// Copyright (c) 2021 Regents of the University Of California
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/ceph/go-ceph/rados"
	etcd "github.com/coreos/etcd/clientv3"
	"github.com/pborman/uuid"
)

var etcdClient *etcd.Client

const pfx = "btrdb"

var hotctx *rados.IOContext
var otherctx *rados.IOContext

//var coldctx *rados.IOContext
type SK struct {
	Collection string
	Stream     string
}

var names map[string]SK

var streams []string = []string{"L1MAG", "L2MAG", "L3MAG", "L1ANG", "L2ANG", "L3ANG", "C1MAG", "C2MAG", "C3MAG", "C1ANG", "C2ANG", "C3ANG", "LSTATE", "FUND_DPF", "FUND_VA", "FUND_VAR", "FUND_W", "FREQ_L1_1S", "FREQ_L1_C37"}

const UpmuSpaceString = "c9bbebff-ff40-4dbe-987e-f9e96afb7a57"

var UpmuSpace = uuid.Parse(UpmuSpaceString)

func descriptorFromSerial(serial string) string {
	return strings.ToLower(fmt.Sprintf("psl.pqube3.%s", serial))
}

func getUUID(serial string, streamname string) uuid.UUID {
	streamid := fmt.Sprintf("%v.%v", descriptorFromSerial(serial), streamname)
	return uuid.NewSHA1(UpmuSpace, []byte(streamid))
}

func main() {
	names = make(map[string]SK)
	data, err := ioutil.ReadFile("mapping.txt")
	if err != nil {
		fmt.Printf("you need to have a file called mapping.txt\n")
		os.Exit(1)
	}
	lines := bytes.Split(data, []byte("\n"))
	for _, l := range lines {
		if len(strings.TrimSpace(string(l))) == 0 {
			continue
		}
		kv := strings.Split(string(l), "=")
		collection := strings.TrimSpace(kv[1])
		for _, s := range streams {
			uu := getUUID(kv[0], s)
			names[uu.String()] = SK{
				Collection: collection,
				Stream:     s,
			}
		}
	}
	etcdEndpoint := os.Getenv("ETCD_ENDPOINT")
	if len(etcdEndpoint) == 0 {
		etcdEndpoint = "http://etcd:2379"
	}
	btrdbhot := os.Getenv("BTRDB_HOT_POOL")
	if len(btrdbhot) == 0 {
		fmt.Printf("$BTRDB_HOT_POOL unset, defaulting to `btrdbhot`\n")
		btrdbhot = "btrdbhot"
	}
	etcdClient, err = etcd.New(etcd.Config{
		Endpoints:   []string{etcdEndpoint},
		DialTimeout: 5 * time.Second})
	if err != nil {
		fmt.Printf("Could not connect to etcd: %v\n", err)
		os.Exit(1)
	}
	conn, _ := rados.NewConn()
	conn.ReadDefaultConfigFile()
	err = conn.Connect()
	if err != nil {
		fmt.Printf("Could not connect to ceph: %v\n", err)
		os.Exit(1)
	}
	defer conn.Shutdown()

	hotctx, err = conn.OpenIOContext(btrdbhot)
	if err != nil {
		fmt.Printf("Could not open pool %q: %v\n", btrdbhot, err)
		os.Exit(1)
	}
	otherctx, err = conn.OpenIOContext(btrdbhot)
	if err != nil {
		fmt.Printf("Could not open pool %q: %v\n", btrdbhot, err)
		os.Exit(1)
	}
	FindStreams()
}
func IsInEtcd(uuid []byte) bool {
	streamkey := fmt.Sprintf("%s/u/%s", pfx, string(uuid))
	resp, err := etcdClient.Get(context.Background(), streamkey)
	if err != nil {
		fmt.Printf("etcd error: %v\n", err)
		os.Exit(1)
	}
	return resp.Count > 0
}

type rec struct {
	uuid    uuid.UUID
	version uint64
}

func FindStreams() {
	found := 0
	skipped := 0
	torecover := make([]rec, 0, 100)
	hotctx.ListObjects(func(oid string) {
		if !strings.HasPrefix(oid, "meta") {
			return
		}
		uuidhex := oid[4:]
		uu, err := hex.DecodeString(uuidhex)
		if err != nil {
			fmt.Printf("weird object name %q\n", oid)
			return
		}
		data := make([]byte, 8)
		bc, err := otherctx.GetXattr(oid, "version", data)
		if err == rados.RadosErrorNotFound {
			fmt.Printf("object missing xattr %q\n, oid")
			return
		}
		if err != nil || bc != 8 {
			fmt.Printf("weird ceph error getting xattrs: %v", err)
			return
		}
		ver := binary.LittleEndian.Uint64(data)
		uuproper := uuid.UUID(uu)
		if IsInEtcd(uu) {
			skipped++
			return
		}

		if ver <= 10 {
			fmt.Printf("Found unrecoverable orphan (journal only)\n")
			return
		} else {
			fmt.Printf("Found orphaned stream %s version %d\n", uuproper.String(), ver)
		}
		found++

		torecover = append(torecover, rec{
			uuid:    uuproper,
			version: ver,
		})
	})
	fmt.Printf("found %d orphans and %d okay streams\n", found, skipped)
	mapped := 0
	for _, s := range torecover {
		x, ok := names[s.uuid.String()]
		if ok {
			fmt.Printf("found uuid %q for mapping (%s)\n", s.uuid.String(), x.Collection)
			mapped++
		}
	}
	fmt.Printf("of the orphans, %d have a mapping, and %d do not\n", mapped, len(torecover)-mapped)
	if os.Getenv("TRY_RECOVER") == "" {
		fmt.Printf("$TRY_RECOVER is not set, aborting\n")
		os.Exit(1)
	}
	fmt.Printf("ok, we are going to create streams in the collection \"recovery/\"\n")
	fmt.Printf("these have no metadata, so you will have to copy them to appropriately named streams\n")

	for _, r := range torecover {
		var collection, name string
		sk, ok := names[r.uuid.String()]
		if ok {
			collection = sk.Collection
			name = sk.Stream
		} else {
			collection = fmt.Sprintf("recovery/%s_v%d\n", r.uuid, r.version)
			name = "recovered"
		}
		tags := make(map[string]string)
		tags["name"] = name
		annotations := make(map[string]string)

		fr := &FullRecord{
			Tags:       tags,
			Anns:       annotations,
			Collection: collection,
		}
		streamkey := fmt.Sprintf("%s/u/%s", pfx, string(r.uuid))
		tombstonekey := fmt.Sprintf("%s/z/%s", pfx, string(r.uuid))
		opz := []etcd.Op{}
		opz = append(opz, etcd.OpPut(streamkey, string(fr.serialize())))
		for k, v := range tags {
			path := fmt.Sprintf("%s/t/%s/%s/%s", pfx, k, collection, string(r.uuid))
			opz = append(opz, etcd.OpPut(path, v))
		}
		for k, v := range annotations {
			path := fmt.Sprintf("%s/a/%s/%s/%s", pfx, k, collection, string(r.uuid))
			opz = append(opz, etcd.OpPut(path, v))
		}
		//Although this may exist, it is important to write to it again
		//because the delete code will transact on the version of this
		colpath := fmt.Sprintf("%s/c/%s/", pfx, collection)
		opz = append(opz, etcd.OpPut(colpath, "NA"))
		tagstring := tagString(tags)
		tagstringpath := fmt.Sprintf("%s/s/%s/%s", pfx, collection, tagstring)
		opz = append(opz, etcd.OpPut(tagstringpath, string(r.uuid)))
		txr, err := etcdClient.Txn(context.Background()).
			If(etcd.Compare(etcd.Version(tombstonekey), "=", 0),
				etcd.Compare(etcd.Version(streamkey), "=", 0),
				etcd.Compare(etcd.Version(tagstringpath), "=", 0)).
			Then(opz...).
			Commit()
		if err != nil {
			fmt.Printf("could not create stream: %v\n", err)
			os.Exit(1)
		}
		if !txr.Succeeded {
			fmt.Printf("could not create transaction. call michael\n")
			os.Exit(1)
		}
	}
}
