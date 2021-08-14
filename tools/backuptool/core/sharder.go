// Copyright (c) 2021 Michael Andersen
// Copyright (c) 2021 Regents of the University Of California
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package core

import (
	"fmt"
	"io"
	"os"
	"path"
)

const MaxFileSize = 2 * 1024 * 1024 * 1024

type Sharder struct {
	readmode      bool
	fw            *FrameWriter
	directory     string
	filenumber    int
	maxfilenumber int
	curfile       *os.File
}

func (s *Sharder) Metadata() *BackupMetadata {
	rv := &BackupMetadata{
		NumberOfFiles: s.filenumber,
	}
	for k, v := range gtimestamps {
		rv.Timestamps = append(rv.Timestamps, TimestampEntry{
			Key: k,
			Val: v,
		})

	}
	return rv
}
func IsEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1) // Or f.Readdir(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err // Either not empty or error, suits both cases
}
func NewReadSharder(fw *FrameWriter, directory string) *Sharder {
	rv := &Sharder{
		readmode:   true,
		fw:         fw,
		filenumber: 0,
		directory:  directory,
	}
	fname := fmt.Sprintf("ARCHIVE%05d.bin", rv.filenumber)
	f, err := os.Open(path.Join(directory, fname))
	ChkErr(err)
	rv.curfile = f
	return rv
}
func ReadIncrementalMetadata(fw *FrameWriter, olddir string) {
	if olddir == "" {
		fmt.Printf("skipping incremental metadata\n")
		return
	}
	f, err := os.Open(path.Join(olddir, "METADATA.bin"))
	ChkErr(err)
	content, err := fw.Read(f)
	ChkErr(err)
	if content.Type != BackupMetadataTypeCode {
		panic("unexpected metadata frame type")
	}
	bm := BackupMetadata{}
	_, err = bm.UnmarshalMsg(content.Content)
	ChkErr(err)
	ginctimestamps = make(map[MK]int64)
	for _, kv := range bm.Timestamps {
		ginctimestamps[kv.Key] = kv.Val
	}
	gdoincrement = true
	return
}
func NewWriteSharder(fw *FrameWriter) *Sharder {
	d := goutputdir
	err := os.MkdirAll(d, 0700)
	ChkErr(err)
	isempty, err := IsEmpty(d)
	ChkErr(err)
	if !isempty {
		fmt.Printf("the output directory contains files already, aborting\n")
		os.Exit(1)
	}
	return &Sharder{
		fw:         fw,
		filenumber: 0,
		directory:  goutputdir,
	}
}

func (s *Sharder) Read() (Container, bool) {
	frame, err := s.fw.Read(s.curfile)
	if err == io.EOF {
		s.curfile.Close()
		s.filenumber++
		fname := fmt.Sprintf("ARCHIVE%05d.bin", s.filenumber)
		f, err := os.Open(path.Join(s.directory, fname))
		if err == nil {
			s.curfile = f
		} else if os.IsNotExist(err) {
			return nil, false
		} else {
			ChkErr(err)
		}
		frame, err = s.fw.Read(s.curfile)
		ChkErr(err)
	} else {
		ChkErr(err)
	}
	switch frame.Type {
	case CephObjectTypeCode:
		obj := CephObject{}
		_, err := obj.UnmarshalMsg(frame.Content)
		ChkErr(err)
		return &obj, true
	case EtcdRecordsTypeCode:
		obj := EtcdRecords{}
		_, err := obj.UnmarshalMsg(frame.Content)
		ChkErr(err)
		return &obj, true
	}
	panic(fmt.Sprintf("unexpected object type code %d\n", frame.Type))
}
func (s *Sharder) Write(c Container) {
	if s.readmode {
		panic("in read mode")
	}
	content, err := c.MarshalMsg(nil)
	ChkErr(err)
	create := false
	if s.curfile == nil {
		create = true
	} else {
		sk, err := s.curfile.Seek(0, os.SEEK_CUR)
		ChkErr(err)
		if sk > MaxFileSize {
			create = true
		}
	}

	if create {
		fname := fmt.Sprintf("ARCHIVE%05d.bin", s.filenumber)
		fmt.Printf("creating new file %s\n", fname)
		if s.curfile != nil {
			s.curfile.Close()
			s.curfile = nil
		}
		f, err := os.Create(path.Join(s.directory, fname))
		s.filenumber++
		ChkErr(err)
		s.curfile = f
	}
	err = s.fw.Write(s.curfile, c.GetType(), content)
	ChkErr(err)
}
func (s *Sharder) WriteMeta(c Container) {
	if s.readmode {
		panic("in read mode")
	}
	f, err := os.Create(path.Join(s.directory, "METADATA.bin"))
	ChkErr(err)
	content, err := c.MarshalMsg(nil)
	ChkErr(err)
	err = s.fw.Write(f, c.GetType(), content)
	ChkErr(err)
	f.Close()
}
func (s *Sharder) Close() {
	if s.curfile != nil {
		s.curfile.Close()
		s.curfile = nil
	}
}
