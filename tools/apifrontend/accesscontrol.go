package main

import (
	"context"

	"github.com/immesys/smartgridstore/acl"
	"github.com/pborman/uuid"
)

const PRawValues = acl.Permission("btrdb/RawValues")
const PAlignedWindows = acl.Permission("btrdb/AlignedWindows")
const PWindows = acl.Permission("btrdb/Windows")
const PStreamInfo = acl.Permission("btrdb/StreamInfo")
const PSetStreamAnnotations = acl.Permission("btrdb/SetStreamAnnotations")
const PChanges = acl.Permission("btrdb/Changes")
const PCreate = acl.Permission("btrdb/Create")
const PListCollections = acl.Permission("btrdb/ListCollections")
const PLookupStreams = acl.Permission("btrdb/LookupStreams")
const PNearest = acl.Permission("btrdb/Nearest")
const PInsert = acl.Permission("btrdb/Insert")
const PDelete = acl.Permission("btrdb/Delete")
const PFlush = acl.Permission("btrdb/Flush")
const PObliterate = acl.Permission("btrdb/Obliterate")
const PFaultInject = acl.Permission("btrdb/FaultInject")

type AccessControl struct {
}

func (ac *AccessControl) CheckPermissionsByUUID(ctx context.Context, uu uuid.UUID, op acl.Permission) error {
	//TODO
	return nil
}
