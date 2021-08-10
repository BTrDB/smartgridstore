// Copyright (c) 2021 Michael Andersen
// Copyright (c) 2021 Regents of the University Of California
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/pborman/uuid"
)

var streams []string = []string{"L1MAG", "L2MAG", "L3MAG", "L1ANG", "L2ANG", "L3ANG", "C1MAG", "C2MAG", "C3MAG", "C1ANG", "C2ANG", "C3ANG", "LSTATE", "FUND_DPF", "FUND_VA", "FUND_VAR", "FUND_W", "FREQ_L1_1S", "FREQ_L1_C37"}

const UpmuSpaceString = "c9bbebff-ff40-4dbe-987e-f9e96afb7a57"

var UpmuSpace = uuid.Parse(UpmuSpaceString)

func descriptorFromSerial(serial string) string {
	return strings.ToLower(fmt.Sprintf("psl.pqube3.%s", serial))
}

func getUUID(serial string, streamname string) uuid.UUID {
	streamid := fmt.Sprintf("%v.%v", descriptorFromSerial(serial), streamname)
	return uuid.NewSHA1(UpmuSpace, []byte(streamid))
}

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("usage: manifest2uuid <serial>\n")
		os.Exit(1)
	}
	serial := os.Args[1]
	serial = strings.ToUpper(serial)
	fmt.Printf("Streams for device %s\n", descriptorFromSerial(serial))
	for _, s := range streams {
		fmt.Printf("  %-12s %s\n", s, getUUID(serial, s))
	}
}
