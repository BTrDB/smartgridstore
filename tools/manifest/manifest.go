package manifest

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/samkumar/etcdstruct"
	"github.com/ugorji/go/codec"
	"golang.org/x/crypto/sha3"

	etcd "github.com/coreos/etcd/clientv3"
)

const manifestpath = "manifest/"
const manifestlockpath = "manifestlocks/"

var etcdprefix = ""

var mp codec.Handle = &codec.MsgpackHandle{}

type ManifestDeviceStream struct {
	CanonicalName string            `codec:"-" yaml:"-"`
	Metadata      map[string]string `codec:"metadata,omitempty" yaml:"metadata"`
}

type ManifestDevice struct {
	Descriptor string                           `codec:"-" yaml:"-"`
	Metadata   map[string]string                `codec:"metadata,omitempty" yaml:"metadata"`
	Streams    map[string]*ManifestDeviceStream `codec:"streams" yaml:"streams"`

	retrievedRevision int64
}

func (md *ManifestDevice) SetRetrievedRevision(rev int64) {
	md.retrievedRevision = rev
}

func (md *ManifestDevice) GetRetrievedRevision() int64 {
	return md.retrievedRevision
}

func SetEtcdKeyPrefix(prefix string) {
	etcdprefix = prefix
}

func getEtcdKey(name string) string {
	return fmt.Sprintf("%s%s%s", etcdprefix, manifestpath, name)
}

func getEtcdLockKey(name string, prefix string) string {
	return fmt.Sprintf("%s%s%s/%s", etcdprefix, manifestlockpath, prefix, name)
}

func getNameFromEtcdKey(etcdKey string) string {
	return etcdKey[len(etcdprefix)+len(manifestpath):]
}

func RetrieveManifestDevice(ctx context.Context, etcdClient *etcd.Client, descriptor string) (md *ManifestDevice, err error) {
	md = &ManifestDevice{Descriptor: descriptor}
	exists, err := etcdstruct.RetrieveEtcdStruct(ctx, etcdClient, getEtcdKey(descriptor), md)
	if !exists {
		md = nil
	} else {
		if md.Metadata == nil {
			md.Metadata = make(map[string]string)
		}
		if md.Streams == nil {
			md.Streams = make(map[string]*ManifestDeviceStream)
		}
	}
	return
}

func UpsertManifestDevice(ctx context.Context, etcdClient *etcd.Client, md *ManifestDevice) error {
	return etcdstruct.UpsertEtcdStruct(ctx, etcdClient, getEtcdKey(md.Descriptor), md)
}

func UpsertManifestDeviceAtomically(ctx context.Context, etcdClient *etcd.Client, md *ManifestDevice) (bool, error) {
	return etcdstruct.UpsertEtcdStructAtomic(ctx, etcdClient, getEtcdKey(md.Descriptor), md)
}

func GetDescriptorShortForm(descriptor string) string {
	hash := sha3.Sum256([]byte(descriptor))
	b64 := base64.URLEncoding.EncodeToString(hash[:])
	return b64[:10]
}

func GetLockTable(ctx context.Context, etcdClient *etcd.Client, prefix string) (map[string][]string, error) {
	locktableprefix := getEtcdLockKey("", prefix)
	resp, err := etcdClient.Get(ctx, locktableprefix, etcd.WithPrefix())
	if err != nil {
		return nil, err
	}
	ltable := make(map[string][]string)
	for _, r := range resp.Kvs {
		did := strings.TrimPrefix(string(r.Key), locktableprefix)
		ltable[string(r.Value)] = append(ltable[string(r.Value)], did)
	}
	return ltable, nil
}
func ObtainDeviceLock(ctx context.Context, etcdClient *etcd.Client, md *ManifestDevice, myid string, prefix string) (bool, error) {
	lockkey := getEtcdLockKey(md.Descriptor, prefix)
	lockval := myid
	resp, err := etcdClient.Grant(ctx, 5)
	if err != nil {
		return false, err
	}

	txr, err := etcdClient.Txn(ctx).
		If(etcd.Compare(etcd.CreateRevision(lockkey), "=", 0)).
		Then(etcd.OpPut(lockkey, lockval, etcd.WithLease(resp.ID))).
		Commit()
	if err != nil {
		return false, err
	}
	if !txr.Succeeded {
		return false, nil
	}
	ch, err := etcdClient.KeepAlive(ctx, resp.ID)
	if err != nil {
		return false, err
	}
	go func() {
		for _ = range ch {

		}
	}()
	return true, nil
}
func RetrieveMultipleManifestDevices(ctx context.Context, etcdClient *etcd.Client, descprefix string) ([]*ManifestDevice, error) {
	etcdKeyPrefix := getEtcdKey(descprefix)
	devs := make([]*ManifestDevice, 0, 1024)
	err := etcdstruct.RetrieveEtcdStructs(ctx, etcdClient, func(key []byte) etcdstruct.EtcdStruct {
		dev := &ManifestDevice{Descriptor: getNameFromEtcdKey(string(key))}
		devs = append(devs, dev)
		return dev
	}, func(es etcdstruct.EtcdStruct, key []byte) {
		dev := es.(*ManifestDevice)
		dev.Descriptor = getNameFromEtcdKey(string(key))
		dev.Streams = nil
	}, etcdKeyPrefix, etcd.WithPrefix())
	if err != nil {
		return nil, err
	}

	return devs, err
}

func DeleteManifestDevice(ctx context.Context, etcdClient *etcd.Client, descriptor string) error {
	_, err := etcdstruct.DeleteEtcdStructs(ctx, etcdClient, getEtcdKey(descriptor))
	return err
}

func DeleteMultipleManifestDevices(ctx context.Context, etcdClient *etcd.Client, descprefix string) (int64, error) {
	return etcdstruct.DeleteEtcdStructs(ctx, etcdClient, getEtcdKey(descprefix), etcd.WithPrefix())
}
