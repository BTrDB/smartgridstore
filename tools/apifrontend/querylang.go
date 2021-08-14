// Copyright (c) 2021 Michael Andersen
// Copyright (c) 2021 Regents of the University Of California
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bcampbell/fuzzytime"
	"gopkg.in/yaml.v2"
)

//Nothing in this file is ready to use yet.

const SELECT = "SELECT"
const FROM = "FROM"

type Query struct {
	Window *QueryWindow `yaml:"window"`
}

type QueryWindow struct {
	Fields          []string `yaml:"fields"`
	Collection      string   `yaml:"collection"`
	Properties      []string `yaml:"properties"`
	From            string   `yaml:"from"`
	To              string   `yaml:"to"`
	Window          string   `yaml:"window"`
	WindowPrecision string   `yaml:"windowPrecision"`
}

type Field struct {
	Operation string
	Name      string
}

//ParseDuration is a little like the existing time.ParseDuration
//but adds days and years because its really annoying not having that
func ParseDuration(s string) (*time.Duration, error) {
	if s == "" {
		return nil, nil
	}
	pat := regexp.MustCompile(`^(\d+y)?(\d+d)?(\d+h)?(\d+m)?(\d+s)?$`)
	res := pat.FindStringSubmatch(s)
	if res == nil {
		return nil, fmt.Errorf("Invalid duration")
	}
	res = res[1:]
	sec := int64(0)
	for idx, mul := range []int64{365 * 24 * 60 * 60, 24 * 60 * 60, 60 * 60, 60, 1} {
		if res[idx] != "" {
			key := res[idx][:len(res[idx])-1]
			v, e := strconv.ParseInt(key, 10, 64)
			if e != nil { //unlikely
				return nil, e
			}
			sec += v * mul
		}
	}
	rv := time.Duration(sec) * time.Second
	return &rv, nil
}

func OpAllowed(s string) bool {
	return s == "MEAN" || s == "MIN" || s == "MAX" || s == "COUNT"
}
func (qw *QueryWindow) ParseFields() ([]Field, error) {
	rv := []Field{}
	for _, e := range qw.Fields {
		pstart := strings.Index(e, "(")
		pend := strings.LastIndex(e, ")")
		fmt.Printf("pstart=%d, pend=%d e=%q", pstart, pend, e)
		if (pstart + 1) >= (pend - 1) {
			return nil, fmt.Errorf("invalid field name")
		}
		name := e[pstart+1 : pend-1]
		op := e[0:pstart]
		if !OpAllowed(op) {
			return nil, fmt.Errorf("invalid operation %q", op)
		}
		rv = append(rv, Field{Operation: op, Name: name})
	}
	return rv, nil
}
func (qw *QueryWindow) ParseProperties() (anns map[string]*string, tags map[string]*string, err error) {
	anns = make(map[string]*string)
	tags = make(map[string]*string)
	for _, p := range qw.Properties {
		kvz := strings.Split(p, "=")
		pname := kvz[0]
		var pval *string
		if len(kvz) == 2 && len(kvz[1]) > 0 {
			pval = &kvz[1]
		}
		if strings.HasPrefix(pname, "a.") {
			anns[pname[2:]] = pval
		} else if strings.HasPrefix(pname, "t.") {
			tags[pname[2:]] = pval
		} else {
			return nil, nil, fmt.Errorf("properties must start with p. or t.")
		}
	}
	return anns, tags, nil
}
func (qw *QueryWindow) ParseCollection() (string, bool, error) {
	if qw.Collection == "" {
		return "", false, fmt.Errorf("no collection specified")
	}
	col := qw.Collection
	isprefix := false
	if strings.HasSuffix(col, "*") {
		col = strings.TrimSuffix(col, "*")
		isprefix = true
	}
	return col, isprefix, nil
}
func ParseFuzzyTime(s string) (time.Time, error) {
	dt, _, err := fuzzytime.Extract(s)
	if err != nil {
		return time.Time{}, err
	}
	if !dt.HasFullDate() {
		return time.Time{}, fmt.Errorf("insufficient precision")
	}
	if !dt.HasHour() {
		dt.SetHour(0)
	}
	if !dt.HasMinute() {
		dt.SetMinute(0)
	}
	if !dt.HasSecond() {
		dt.SetSecond(0)
	}
	isoformat := dt.ISOFormat()
	return time.Parse(time.RFC3339, isoformat)
}
func (qw *QueryWindow) ParseFrom() (time.Time, error) {
	return ParseFuzzyTime(qw.From)
}
func (qw *QueryWindow) ParseTo() (time.Time, error) {
	return ParseFuzzyTime(qw.To)
}
func (qw *QueryWindow) ParseWindow() (time.Duration, error) {
	d, err := ParseDuration(qw.Window)
	if err != nil {
		return 0, fmt.Errorf("malformed duration expression")
	}
	return *d, nil
}

const DefaultWindowPrecision = 20 //~1ms

func (qw *QueryWindow) ParseWindowPrecision() (int, error) {
	if qw.WindowPrecision == "" {
		return DefaultWindowPrecision, nil
	}
	//A power user can specify the exact PW
	asInt, err := strconv.ParseInt(qw.WindowPrecision, 10, 64)
	if err == nil {
		if asInt >= 0 && asInt <= 48 {
			return int(asInt), nil
		} else {
			return 0, fmt.Errorf("window precision exponent must be >=0 and <= 48")
		}
	}
	//Normal users can specify a duration and we pick the first PW
	//below that duration
	d, err := ParseDuration(qw.Window)
	if err != nil {
		return 0, fmt.Errorf("malformed duration expression")
	}
	rv := uint(0)
	for time.Duration(1<<rv) <= *d {
		rv++
	}
	rv -= 1
	if rv >= 48 {
		return 48, nil
	}
	if rv < 0 {
		return 0, nil
	}
	return int(rv), nil
}
func tokenize(q string) []string {
	rv := []string{}
	parts := strings.Split(q, " ")
	for _, p := range parts {
		rv = append(rv, strings.TrimSpace(p))
	}
	return rv
}
func queryhandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		w.WriteHeader(405)
		w.Write([]byte("queries must be POSTed"))
		return
	}
	user, pass, ok := req.BasicAuth()
	fmt.Printf("user: %q, pass: %q, ok: %v\n", user, pass, ok)
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		fmt.Printf("Query error: %v\n", err)
		return
	}
	fmt.Printf("body was:\n%v\n", string(body))
	req.Body.Close()
	doselect(body, w)
}

func doselect(query []byte, w http.ResponseWriter) error {
	q := Query{}
	err := yaml.Unmarshal(query, &q)
	abort := func(fs string, args ...interface{}) error {
		w.WriteHeader(400)
		w.Write([]byte(fmt.Sprintf(fs, args...)))
		return nil
	}
	if err != nil {
		return abort("invalid query format: %v", err)
	}
	if q.Window == nil {
		return abort("empty query")
	}
	qw := q.Window
	fields, err := qw.ParseFields()
	if err != nil {
		return abort("invalid fields: %v", err)
	}
	collection, isprefix, err := qw.ParseCollection()
	if err != nil {
		return abort("invalid collections: %v", err)
	}
	from, err := qw.ParseFrom()
	if err != nil {
		return abort("invalid 'from' time: %v", err)
	}
	to, err := qw.ParseTo()
	if err != nil {
		return abort("invalid 'to' time: %v", err)
	}
	windowDuration, err := qw.ParseWindow()
	if err != nil {
		return abort("invalid 'window' duration: %v", err)
	}
	windowPrecision, err := qw.ParseWindowPrecision()
	if err != nil {
		return abort("invalid 'windowPrecision' duration: %v", err)
	}
	anns, tags, err := qw.ParseProperties()
	if err != nil {
		return abort("invalid properties: %v", err)
	}
	fmt.Printf("\nparsed:\nfields=%v\ncollection=%v\nisprefix=%v\nfrom=%v\nto=%v\nwindowDuration=%v\nwindowPrecision=%v\nanns=%v\ntags=%v\n", fields, collection, isprefix, from, to, windowDuration, windowPrecision, anns, tags)
	return nil

}
