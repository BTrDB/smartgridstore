// Copyright (c) 2021 Michael Andersen
// Copyright (c) 2021 Regents of the University Of California
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package main

import (
	"bytes"
	"fmt"
	"sort"
)

//go:generate msgp

type FullRecord struct {
	Collection string            `msg:"c"`
	Tags       map[string]string `msg:"t"`
	Anns       map[string]string `msg:"a"`
}

func (fr *FullRecord) setAnnotation(key string, value string) {
	fr.Anns[key] = value
}
func (fr *FullRecord) deleteAnnotation(key string) {
	delete(fr.Anns, key)
}
func (fr *FullRecord) serialize() []byte {
	rv, err := fr.MarshalMsg(nil)
	if err != nil {
		panic(err)
	}
	return rv
}

func tagString(tags map[string]string) string {
	strs := []string{}
	sz := 1 //one extra for fun
	for k, v := range tags {
		sz += 2 + len(k) + len(v)
		strs = append(strs, fmt.Sprintf("%s\x00%s\x00", k, v))
	}
	sort.StringSlice(strs).Sort()
	ts := bytes.NewBuffer(make([]byte, 0, sz))
	for _, s := range strs {
		ts.WriteString(s)
	}
	return ts.String()
}
