package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/BTrDB/smartgridstore/tools/gen2ingress"
	btrdb "gopkg.in/BTrDB/btrdb.v4"
)

//Driver definition
type FNETAscii struct {
	inserter *gen2ingress.Inserter
	prefix   string
}

func (fn *FNETAscii) DIDPrefix() string {
	return "fnet.ascii.client"
}
func (fn *FNETAscii) InitiatesConnections() bool {
	return true
}
func (fn *FNETAscii) SetConn(in *gen2ingress.Inserter) {
	fn.inserter = in
}

func (fn *FNETAscii) HandleDevice(descriptor string) {
	//Format: fnet.ascii.client/collection/prefix@ip:port
Note this wont work for multiple devices. fn cannot have state as a multi-device driver
	suffix := strings.TrimPrefix(descriptor, fn.DIDPrefix())
	parts := strings.Split(suffix, "@")
	//Remove starting slash
	prefix := strings.TrimPrefix(parts[0], "/")
	fn.prefix = prefix
	target := parts[1]
	gen2ingress.DialLoop(target, fn.processDevice)
}

func (fn *FNETAscii) processDevice(conn *net.TCPConn, r *bufio.Reader) error {
	sync := func() error {
		for {
			pa, err := r.Peek(1)
			if err != nil {
				return err
			}
			if pa[0] == 0x01 {
				return nil
			}
			_, err = r.ReadByte()
			if err != nil {
				return err
			}
		}
	}

	for {
		sync()
		frame, err := r.ReadString(0)
		if err != nil {
			return err
		}
		if frame[54] != 0x00 {
			fmt.Printf("skipping bad frame\n")
			continue
		}

		var fUnitID int
		var fDateMonth int
		var fDateDay int
		var fDateYear int
		var fTimeHour int
		var fTimeMinute int
		var fTimeSecond int
		var fConvNum int
		var fFirstFreq float64
		var fFinalFreq float64
		var fVoltage float64
		var fAngle float64

		//Extract the fields
		nscanned, err := fmt.Sscanf(string(frame[1:54]),
			"%3d %2d%2d%2d %2d%2d%2d %2d %f %f %f %f",
			&fUnitID, &fDateMonth, &fDateDay, &fDateYear,
			&fTimeHour, &fTimeMinute, &fTimeSecond,
			&fConvNum, &fFirstFreq, &fFinalFreq,
			&fVoltage, &fAngle)
		if nscanned != 12 || err != nil {
			fmt.Printf("skipping bad frame\n")
			continue
		}
		//Mon Jan 2 15:04:05 MST 2006
		//RFC3339     = "2006-01-02T15:04:05Z07:00"
		ts, err := time.Parse("06-01-02 15-04-05", fmt.Sprintf("%02d-%02d-%02d %02d-%02d-%02d",
			fDateYear, fDateMonth, fDateDay, fTimeHour, fTimeMinute, fTimeSecond))
		if err != nil {
			fmt.Printf("skipping bad frame: bad timestamp\n")
			continue
		}
		//TODO add subsecond part to timestamp
		tsnano := ts.UnixNano()
		dat := []gen2ingress.InsertRecord{}
		dat = append(dat, &gen2ingress.StructInsertRecord{
			FData:       []btrdb.RawPoint{{Time: tsnano, Value: fVoltage}},
			FName:       "Voltage",
			FCollection: fn.prefix,
			FUnit:       "Volts",
		})
		dat = append(dat, &gen2ingress.StructInsertRecord{
			FData:       []btrdb.RawPoint{{Time: tsnano, Value: fAngle}},
			FName:       "Angle",
			FCollection: fn.prefix,
			FUnit:       "Radians",
		})
		dat = append(dat, &gen2ingress.StructInsertRecord{
			FData:       []btrdb.RawPoint{{Time: tsnano, Value: fFinalFreq}},
			FName:       "FinalFreq",
			FCollection: fn.prefix,
			FUnit:       "Hz",
		})
		//First freq is an overloaded field
		doffreq := !(fTimeSecond == 0 && fConvNum <= 2)
		if fTimeSecond == 0 {
			if fConvNum == 0 {
				//Latitutde
				dat = append(dat, &gen2ingress.StructInsertRecord{
					FData:       []btrdb.RawPoint{{Time: tsnano, Value: fFirstFreq}},
					FName:       "Latitude",
					FCollection: fn.prefix,
					FUnit:       "Degrees",
				})
			}
			if fConvNum == 1 {
				//Longitude
				dat = append(dat, &gen2ingress.StructInsertRecord{
					FData:       []btrdb.RawPoint{{Time: tsnano, Value: fFirstFreq}},
					FName:       "Longitude",
					FCollection: fn.prefix,
					FUnit:       "Degrees",
				})
			}
			if fConvNum == 2 {
				//Satellite count
				dat = append(dat, &gen2ingress.StructInsertRecord{
					FData:       []btrdb.RawPoint{{Time: tsnano, Value: fFirstFreq}},
					FName:       "Satellites",
					FCollection: fn.prefix,
					FUnit:       "Count",
				})
			}
		}
		if doffreq {
			dat = append(dat, &gen2ingress.StructInsertRecord{
				FData:       []btrdb.RawPoint{{Time: tsnano, Value: fFirstFreq}},
				FName:       "FirstFreq",
				FCollection: fn.prefix,
				FUnit:       "Hz",
			})
		}
		fn.inserter.ProcessBatch(dat)
	}
}
