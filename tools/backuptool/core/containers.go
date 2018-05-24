package core

import "github.com/tinylib/msgp/msgp"

//go:generate msgp -tests=false -io=false

//type code = 1
const EtcdRecordsTypeCode = 10

type Container interface {
	msgp.Marshaler
	GetType() int
}
type EtcdRecords struct {
	KVz []KeyValue
}

func (e *EtcdRecords) GetType() int {
	return EtcdRecordsTypeCode
}

type KeyValue struct {
	Key   string
	Value []byte
}

const CephObjectTypeCode = 20

type CephObject struct {
	OMAPData  []KeyValue
	XATTRData []KeyValue
	Content   []byte
	Name      string
	Namespace string
	Pool      string
}

func (c *CephObject) GetType() int {
	return CephObjectTypeCode
}

const BackupMetadataTypeCode = 30

type MK struct {
	Pool      string
	Namespace string
	OID       string
}

type TimestampEntry struct {
	Key MK
	Val int64
}
type BackupMetadata struct {
	NumberOfFiles int
	Timestamps    []TimestampEntry
}

func (bm *BackupMetadata) GetType() int {
	return BackupMetadataTypeCode
}
