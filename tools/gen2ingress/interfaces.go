// Copyright (c) 2021 Michael Andersen
// Copyright (c) 2021 Regents of the University Of California
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package gen2ingress

import (
	"bufio"
	"context"
	"net"

	"gopkg.in/BTrDB/btrdb.v4"
)

type InsertRecord struct {
	Data []btrdb.RawPoint
	//TODO this is not used
	Flags             []uint64
	Name              string
	Collection        string
	Unit              string
	AnnotationChanges map[string]string
}

func (ir *InsertRecord) Size() int {
	return len(ir.Data)*16 + 100
}

type DialProcessFunction func(ctx context.Context, conn *net.TCPConn, r *bufio.Reader) error
type CustomProcessFunction func(ctx context.Context) error

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
	HandleDevice(ctx context.Context, descriptor string) error
}
