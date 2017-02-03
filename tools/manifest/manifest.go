package manifest

import (
	"context"
	"fmt"

	"github.com/samkumar/etcdstruct"
	"github.com/ugorji/go/codec"

	etcd "github.com/coreos/etcd/clientv3"
)

const manifestpath = "manifest/"

var etcdprefix = ""

var mp codec.Handle = &codec.MsgpackHandle{}

type ManifestDeviceStream struct {
	CanonicalName string            `codec:"-"`
	Metadata      map[string]string `codec:"metadata,omitempty"`
}

type ManifestDevice struct {
	Descriptor string                           `codec:"-"`
	Metadata   map[string]string                `codec:"metadata,empty"`
	Streams    map[string]*ManifestDeviceStream `codec:",omitempty"`

	retrievedRevision int64
}

func (md *ManifestDevice) SetRetrievedRevision(rev int64) {
	md.retrievedRevision = rev
}

func (md *ManifestDevice) GetRetrievedRevision() int64 {
	return md.retrievedRevision
}

func getEtcdKey(name string) string {
	return fmt.Sprintf("%s%s%s", etcdprefix, manifestpath, name)
}

func getNameFromEtcdKey(etcdKey string) string {
	return etcdKey[len(etcdprefix)+len(manifestpath):]
}

func RetrieveManifestDevice(ctx context.Context, etcdClient *etcd.Client, descriptor string) (md *ManifestDevice, err error) {
	md = &ManifestDevice{Descriptor: descriptor}
	err = etcdstruct.RetrieveEtcdStruct(ctx, etcdClient, getEtcdKey(descriptor), md)
	return
}

func UpsertManifestDevice(ctx context.Context, etcdClient *etcd.Client, md *ManifestDevice) error {
	return etcdstruct.UpsertEtcdStruct(ctx, etcdClient, getEtcdKey(md.Descriptor), md)
}

func UpsertManifestDeviceAtomically(ctx context.Context, etcdClient *etcd.Client, md *ManifestDevice) (bool, error) {
	return etcdstruct.UpsertEtcdStructAtomic(ctx, etcdClient, getEtcdKey(md.Descriptor), md)
}

func RetrieveMultipleManifestDevices(ctx context.Context, etcdClient *etcd.Client, descprefix string) ([]*ManifestDevice, error) {
	etcdKeyPrefix := getEtcdKey(descprefix)
	devs := make([]*ManifestDevice, 0, 1024)
	err := etcdstruct.RetrieveEtcdStructs(ctx, etcdClient, func(key []byte) etcdstruct.EtcdStruct {
		dev := &ManifestDevice{Descriptor: string(key)}
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

func DeleteManifestDevice(ctx context.Context, etcdClient *etcd.Client, md *ManifestDevice) error {
	_, err := etcdstruct.DeleteEtcdStructs(ctx, etcdClient, getEtcdKey(md.Descriptor))
	return err
}

func DeleteMultipleManifestDevices(ctx context.Context, etcdClient *etcd.Client, descprefix string) (int64, error) {
	return etcdstruct.DeleteEtcdStructs(ctx, etcdClient, getEtcdKey(descprefix), etcd.WithPrefix())
}
