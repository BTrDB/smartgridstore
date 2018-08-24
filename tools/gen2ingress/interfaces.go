package gen2ingress

import (
	"bufio"
	"net"

	"gopkg.in/BTrDB/btrdb.v4"
)

type InsertRecord interface {
	Data() []btrdb.RawPoint
	Name() string
	Collection() string
	Unit() string
}

type StructInsertRecord struct {
	FData       []btrdb.RawPoint
	FName       string
	FCollection string
	FUnit       string
}

func (s *StructInsertRecord) Data() []btrdb.RawPoint {
	return s.FData
}
func (s *StructInsertRecord) Name() string {
	return s.FName
}
func (s *StructInsertRecord) Collection() string {
	return s.FCollection
}
func (s *StructInsertRecord) Unit() string {
	return s.FUnit
}

type ProcessFunction func(conn *net.TCPConn, r *bufio.Reader) error

type Driver interface {
	//Which devices should this driver be assigned to
	///e.g "psl.pqube3"
	//It is a prefix of the descriptor
	DIDPrefix() string
	//Return false if we don't need to check the manifest
	// (no DIDPrefix)
	InitiatesConnections() bool
	//This is called early on
	SetConn(in *Inserter)
	//For devices that are connected TO, descriptors will be handed to the driver
	//from the manifest table
	HandleDevice(descriptor string)
}
