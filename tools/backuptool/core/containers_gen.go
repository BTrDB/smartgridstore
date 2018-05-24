package core

// NOTE: THIS FILE WAS PRODUCED BY THE
// MSGP CODE GENERATION TOOL (github.com/tinylib/msgp)
// DO NOT EDIT

import (
	"github.com/tinylib/msgp/msgp"
)

// MarshalMsg implements msgp.Marshaler
func (z *BackupMetadata) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 2
	// string "NumberOfFiles"
	o = append(o, 0x82, 0xad, 0x4e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x4f, 0x66, 0x46, 0x69, 0x6c, 0x65, 0x73)
	o = msgp.AppendInt(o, z.NumberOfFiles)
	// string "Timestamps"
	o = append(o, 0xaa, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x73)
	o = msgp.AppendArrayHeader(o, uint32(len(z.Timestamps)))
	for zxvk := range z.Timestamps {
		o, err = z.Timestamps[zxvk].MarshalMsg(o)
		if err != nil {
			return
		}
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *BackupMetadata) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zbzg uint32
	zbzg, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zbzg > 0 {
		zbzg--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "NumberOfFiles":
			z.NumberOfFiles, bts, err = msgp.ReadIntBytes(bts)
			if err != nil {
				return
			}
		case "Timestamps":
			var zbai uint32
			zbai, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				return
			}
			if cap(z.Timestamps) >= int(zbai) {
				z.Timestamps = (z.Timestamps)[:zbai]
			} else {
				z.Timestamps = make([]TimestampEntry, zbai)
			}
			for zxvk := range z.Timestamps {
				bts, err = z.Timestamps[zxvk].UnmarshalMsg(bts)
				if err != nil {
					return
				}
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *BackupMetadata) Msgsize() (s int) {
	s = 1 + 14 + msgp.IntSize + 11 + msgp.ArrayHeaderSize
	for zxvk := range z.Timestamps {
		s += z.Timestamps[zxvk].Msgsize()
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *CephObject) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 6
	// string "OMAPData"
	o = append(o, 0x86, 0xa8, 0x4f, 0x4d, 0x41, 0x50, 0x44, 0x61, 0x74, 0x61)
	o = msgp.AppendArrayHeader(o, uint32(len(z.OMAPData)))
	for zcmr := range z.OMAPData {
		// map header, size 2
		// string "Key"
		o = append(o, 0x82, 0xa3, 0x4b, 0x65, 0x79)
		o = msgp.AppendString(o, z.OMAPData[zcmr].Key)
		// string "Value"
		o = append(o, 0xa5, 0x56, 0x61, 0x6c, 0x75, 0x65)
		o = msgp.AppendBytes(o, z.OMAPData[zcmr].Value)
	}
	// string "XATTRData"
	o = append(o, 0xa9, 0x58, 0x41, 0x54, 0x54, 0x52, 0x44, 0x61, 0x74, 0x61)
	o = msgp.AppendArrayHeader(o, uint32(len(z.XATTRData)))
	for zajw := range z.XATTRData {
		// map header, size 2
		// string "Key"
		o = append(o, 0x82, 0xa3, 0x4b, 0x65, 0x79)
		o = msgp.AppendString(o, z.XATTRData[zajw].Key)
		// string "Value"
		o = append(o, 0xa5, 0x56, 0x61, 0x6c, 0x75, 0x65)
		o = msgp.AppendBytes(o, z.XATTRData[zajw].Value)
	}
	// string "Content"
	o = append(o, 0xa7, 0x43, 0x6f, 0x6e, 0x74, 0x65, 0x6e, 0x74)
	o = msgp.AppendBytes(o, z.Content)
	// string "Name"
	o = append(o, 0xa4, 0x4e, 0x61, 0x6d, 0x65)
	o = msgp.AppendString(o, z.Name)
	// string "Namespace"
	o = append(o, 0xa9, 0x4e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65)
	o = msgp.AppendString(o, z.Namespace)
	// string "Pool"
	o = append(o, 0xa4, 0x50, 0x6f, 0x6f, 0x6c)
	o = msgp.AppendString(o, z.Pool)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *CephObject) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zwht uint32
	zwht, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zwht > 0 {
		zwht--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "OMAPData":
			var zhct uint32
			zhct, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				return
			}
			if cap(z.OMAPData) >= int(zhct) {
				z.OMAPData = (z.OMAPData)[:zhct]
			} else {
				z.OMAPData = make([]KeyValue, zhct)
			}
			for zcmr := range z.OMAPData {
				var zcua uint32
				zcua, bts, err = msgp.ReadMapHeaderBytes(bts)
				if err != nil {
					return
				}
				for zcua > 0 {
					zcua--
					field, bts, err = msgp.ReadMapKeyZC(bts)
					if err != nil {
						return
					}
					switch msgp.UnsafeString(field) {
					case "Key":
						z.OMAPData[zcmr].Key, bts, err = msgp.ReadStringBytes(bts)
						if err != nil {
							return
						}
					case "Value":
						z.OMAPData[zcmr].Value, bts, err = msgp.ReadBytesBytes(bts, z.OMAPData[zcmr].Value)
						if err != nil {
							return
						}
					default:
						bts, err = msgp.Skip(bts)
						if err != nil {
							return
						}
					}
				}
			}
		case "XATTRData":
			var zxhx uint32
			zxhx, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				return
			}
			if cap(z.XATTRData) >= int(zxhx) {
				z.XATTRData = (z.XATTRData)[:zxhx]
			} else {
				z.XATTRData = make([]KeyValue, zxhx)
			}
			for zajw := range z.XATTRData {
				var zlqf uint32
				zlqf, bts, err = msgp.ReadMapHeaderBytes(bts)
				if err != nil {
					return
				}
				for zlqf > 0 {
					zlqf--
					field, bts, err = msgp.ReadMapKeyZC(bts)
					if err != nil {
						return
					}
					switch msgp.UnsafeString(field) {
					case "Key":
						z.XATTRData[zajw].Key, bts, err = msgp.ReadStringBytes(bts)
						if err != nil {
							return
						}
					case "Value":
						z.XATTRData[zajw].Value, bts, err = msgp.ReadBytesBytes(bts, z.XATTRData[zajw].Value)
						if err != nil {
							return
						}
					default:
						bts, err = msgp.Skip(bts)
						if err != nil {
							return
						}
					}
				}
			}
		case "Content":
			z.Content, bts, err = msgp.ReadBytesBytes(bts, z.Content)
			if err != nil {
				return
			}
		case "Name":
			z.Name, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "Namespace":
			z.Namespace, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "Pool":
			z.Pool, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *CephObject) Msgsize() (s int) {
	s = 1 + 9 + msgp.ArrayHeaderSize
	for zcmr := range z.OMAPData {
		s += 1 + 4 + msgp.StringPrefixSize + len(z.OMAPData[zcmr].Key) + 6 + msgp.BytesPrefixSize + len(z.OMAPData[zcmr].Value)
	}
	s += 10 + msgp.ArrayHeaderSize
	for zajw := range z.XATTRData {
		s += 1 + 4 + msgp.StringPrefixSize + len(z.XATTRData[zajw].Key) + 6 + msgp.BytesPrefixSize + len(z.XATTRData[zajw].Value)
	}
	s += 8 + msgp.BytesPrefixSize + len(z.Content) + 5 + msgp.StringPrefixSize + len(z.Name) + 10 + msgp.StringPrefixSize + len(z.Namespace) + 5 + msgp.StringPrefixSize + len(z.Pool)
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *EtcdRecords) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 1
	// string "KVz"
	o = append(o, 0x81, 0xa3, 0x4b, 0x56, 0x7a)
	o = msgp.AppendArrayHeader(o, uint32(len(z.KVz)))
	for zdaf := range z.KVz {
		// map header, size 2
		// string "Key"
		o = append(o, 0x82, 0xa3, 0x4b, 0x65, 0x79)
		o = msgp.AppendString(o, z.KVz[zdaf].Key)
		// string "Value"
		o = append(o, 0xa5, 0x56, 0x61, 0x6c, 0x75, 0x65)
		o = msgp.AppendBytes(o, z.KVz[zdaf].Value)
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *EtcdRecords) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zpks uint32
	zpks, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zpks > 0 {
		zpks--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "KVz":
			var zjfb uint32
			zjfb, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				return
			}
			if cap(z.KVz) >= int(zjfb) {
				z.KVz = (z.KVz)[:zjfb]
			} else {
				z.KVz = make([]KeyValue, zjfb)
			}
			for zdaf := range z.KVz {
				var zcxo uint32
				zcxo, bts, err = msgp.ReadMapHeaderBytes(bts)
				if err != nil {
					return
				}
				for zcxo > 0 {
					zcxo--
					field, bts, err = msgp.ReadMapKeyZC(bts)
					if err != nil {
						return
					}
					switch msgp.UnsafeString(field) {
					case "Key":
						z.KVz[zdaf].Key, bts, err = msgp.ReadStringBytes(bts)
						if err != nil {
							return
						}
					case "Value":
						z.KVz[zdaf].Value, bts, err = msgp.ReadBytesBytes(bts, z.KVz[zdaf].Value)
						if err != nil {
							return
						}
					default:
						bts, err = msgp.Skip(bts)
						if err != nil {
							return
						}
					}
				}
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *EtcdRecords) Msgsize() (s int) {
	s = 1 + 4 + msgp.ArrayHeaderSize
	for zdaf := range z.KVz {
		s += 1 + 4 + msgp.StringPrefixSize + len(z.KVz[zdaf].Key) + 6 + msgp.BytesPrefixSize + len(z.KVz[zdaf].Value)
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *KeyValue) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 2
	// string "Key"
	o = append(o, 0x82, 0xa3, 0x4b, 0x65, 0x79)
	o = msgp.AppendString(o, z.Key)
	// string "Value"
	o = append(o, 0xa5, 0x56, 0x61, 0x6c, 0x75, 0x65)
	o = msgp.AppendBytes(o, z.Value)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *KeyValue) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zeff uint32
	zeff, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zeff > 0 {
		zeff--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Key":
			z.Key, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "Value":
			z.Value, bts, err = msgp.ReadBytesBytes(bts, z.Value)
			if err != nil {
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *KeyValue) Msgsize() (s int) {
	s = 1 + 4 + msgp.StringPrefixSize + len(z.Key) + 6 + msgp.BytesPrefixSize + len(z.Value)
	return
}

// MarshalMsg implements msgp.Marshaler
func (z MK) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 3
	// string "Pool"
	o = append(o, 0x83, 0xa4, 0x50, 0x6f, 0x6f, 0x6c)
	o = msgp.AppendString(o, z.Pool)
	// string "Namespace"
	o = append(o, 0xa9, 0x4e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65)
	o = msgp.AppendString(o, z.Namespace)
	// string "OID"
	o = append(o, 0xa3, 0x4f, 0x49, 0x44)
	o = msgp.AppendString(o, z.OID)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *MK) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zrsw uint32
	zrsw, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zrsw > 0 {
		zrsw--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Pool":
			z.Pool, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "Namespace":
			z.Namespace, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "OID":
			z.OID, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z MK) Msgsize() (s int) {
	s = 1 + 5 + msgp.StringPrefixSize + len(z.Pool) + 10 + msgp.StringPrefixSize + len(z.Namespace) + 4 + msgp.StringPrefixSize + len(z.OID)
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *TimestampEntry) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 2
	// string "Key"
	// map header, size 3
	// string "Pool"
	o = append(o, 0x82, 0xa3, 0x4b, 0x65, 0x79, 0x83, 0xa4, 0x50, 0x6f, 0x6f, 0x6c)
	o = msgp.AppendString(o, z.Key.Pool)
	// string "Namespace"
	o = append(o, 0xa9, 0x4e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65)
	o = msgp.AppendString(o, z.Key.Namespace)
	// string "OID"
	o = append(o, 0xa3, 0x4f, 0x49, 0x44)
	o = msgp.AppendString(o, z.Key.OID)
	// string "Val"
	o = append(o, 0xa3, 0x56, 0x61, 0x6c)
	o = msgp.AppendInt64(o, z.Val)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *TimestampEntry) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zxpk uint32
	zxpk, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zxpk > 0 {
		zxpk--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Key":
			var zdnj uint32
			zdnj, bts, err = msgp.ReadMapHeaderBytes(bts)
			if err != nil {
				return
			}
			for zdnj > 0 {
				zdnj--
				field, bts, err = msgp.ReadMapKeyZC(bts)
				if err != nil {
					return
				}
				switch msgp.UnsafeString(field) {
				case "Pool":
					z.Key.Pool, bts, err = msgp.ReadStringBytes(bts)
					if err != nil {
						return
					}
				case "Namespace":
					z.Key.Namespace, bts, err = msgp.ReadStringBytes(bts)
					if err != nil {
						return
					}
				case "OID":
					z.Key.OID, bts, err = msgp.ReadStringBytes(bts)
					if err != nil {
						return
					}
				default:
					bts, err = msgp.Skip(bts)
					if err != nil {
						return
					}
				}
			}
		case "Val":
			z.Val, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *TimestampEntry) Msgsize() (s int) {
	s = 1 + 4 + 1 + 5 + msgp.StringPrefixSize + len(z.Key.Pool) + 10 + msgp.StringPrefixSize + len(z.Key.Namespace) + 4 + msgp.StringPrefixSize + len(z.Key.OID) + 4 + msgp.Int64Size
	return
}
