package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"net"
	"sync/atomic"
	"time"

	"github.com/SoftwareDefinedBuildings/sync2_quasar/upmuparser"
)

func roundUp4(x uint32) uint32 {
	return (x + 3) & 0xFFFFFFFC
}

//simulate a PMU waiting interval seconds between files
func simulatePmu(target string, serial int64, interval int64) {
	var sendid uint32 = 0

	//Add jitter to simulation
	time.Sleep(time.Duration(float64(interval)*rand.Float64()*1000.0) * time.Millisecond)

	//Later we can improve this so that multiple runs of the simulator
	//with interval < 120 do not overlap if restarted immediately.
	//for now I don't care. Round it to 1 second
	startTime := (time.Now().UnixNano() / 1000000000) * 1000000000

	//Outer for loop. Do reconnections
	for {
		//reconnect:
		//Reconnect to server
		serial := fmt.Sprintf("P%d", serial)
		fmt.Printf("Connecting virtual PMU %s to server %s\n", serial, target)

		conn, err := net.Dial("tcp", target)
		if err != nil {
			fmt.Printf("Could not connect to receiver: %v\n", err)
			time.Sleep(time.Duration(interval) * time.Second)
			continue
		}

		for {
			//Inner loop, for each file
			startTime += 120 * 1000 * 1000 * 1000
			var blob []byte = generateFile(startTime / 1000000000)

			// Filepath for this file...
			var filepath string = fmt.Sprintf("/simulation/file%v.dat", sendid)

			var lenfp uint32 = uint32(len(filepath))
			var lensn uint32 = uint32(len(serial))
			var lendt uint32 = uint32(len(blob))

			var lenpfp = roundUp4(lenfp)
			var lenpsn = roundUp4(lensn)

			//Send the blob to the receiver
			var header_buf []byte = make([]byte, 16+lenpfp+lenpsn)
			var sendid_buf []byte = header_buf[0:4]
			var lenfp_buf []byte = header_buf[4:8]
			var lensn_buf []byte = header_buf[8:12]
			var lendt_buf []byte = header_buf[12:16]

			binary.LittleEndian.PutUint32(sendid_buf, sendid)
			binary.LittleEndian.PutUint32(lenfp_buf, lenfp)
			binary.LittleEndian.PutUint32(lensn_buf, lensn)
			binary.LittleEndian.PutUint32(lendt_buf, lendt)

			copy(header_buf[16:16+lenpfp], filepath)
			copy(header_buf[16+lenpfp:], serial)

			n, err := conn.Write(header_buf)
			if n != len(header_buf) || err != nil {
				panic("TCP write failed on header")
			}
			n, err = conn.Write(blob)
			if n != len(blob) || err != nil {
				panic("TCP write failed on file")
			}

			var resp_buf []byte = make([]byte, 4)

			totalread := 0
			for totalread < 4 {
				n, err = conn.Read(resp_buf[totalread:])
				if err != nil {
					panic("Could not get confirmation of receipt")
				}
				totalread += n
			}

			var resp uint32 = binary.LittleEndian.Uint32(resp_buf)
			if resp != sendid {
				fmt.Printf("Received improper confirmation of receipt: got %v, expected %v\n", resp, sendid)
			}

			sendid++

			//Increment our stats
			atomic.AddInt64(&sent, 1)
			//Wait INTERVAL before doing next file
			time.Sleep(time.Duration(interval) * time.Second)
		}
	}
}

func generateFile(startTime int64) []byte {
	//generate a 120 second long file starting from startTime in SECONDS
	//and return it as a byte array
	var buffer bytes.Buffer
	for j := 0; j < 120; j++ {
		var data upmuparser.Upmu_one_second_output_standard = generateSecond(startTime + int64(j))
		binary.Write(&buffer, binary.LittleEndian, &data)
	}

	return buffer.Bytes()
}

func clampFloat32(x float64) float32 {
	var y float32 = float32(x)
	var z float64 = float64(y)
	if math.IsInf(z, 1) {
		return math.MaxFloat32
	} else if math.IsInf(z, -1) {
		return -math.MaxFloat32
	} else if math.IsNaN(z) {
		return 0.0
	} else {
		return y
	}
}

// This function is differentiable, but its derivative is not integrable.
func x2sinxinv(x float64) float64 {
	if x == 0.0 {
		return 0.0
	} else {
		return x * x * math.Sin(1/x)
	}
}

// This function satisfies the Intermediate Value Property, but is not integrable.
func xinvsinxinv(x float64) float64 {
	if x == 0.0 {
		return 0.0
	} else {
		xinv := 1 / x
		return xinv * math.Sin(xinv)
	}
}

const WEIERSTRASS_A float64 = 0.5
const WEIERSTRASS_B float64 = 15.0
const WEIERSTRASS_ITER int = 20

// This approximates a function that is continuous, but not differentiable anywhere.
func weierstrass(x float64) float64 {
	var fx float64 = 0
	var aton float64 = 1.0
	var bton float64 = 1.0
	for n := 0; n < WEIERSTRASS_ITER; n++ {
		aton *= WEIERSTRASS_A
		bton *= WEIERSTRASS_B
		fx += aton * math.Cos(bton*math.Pi*x)
	}
	return fx
}

func generateSecond(startTime int64) upmuparser.Upmu_one_second_output_standard {
	var time time.Time = time.Unix(startTime, 0)
	var data upmuparser.Upmu_one_second_output_standard
	data.Data.Sample_interval_in_milliseconds = 1000.0 / 120.0
	data.Data.Timestamp[0] = int32(time.Year())
	data.Data.Timestamp[1] = int32(time.Month())
	data.Data.Timestamp[2] = int32(time.Day())
	data.Data.Timestamp[3] = int32(time.Hour())
	data.Data.Timestamp[4] = int32(time.Minute())
	data.Data.Timestamp[5] = int32(time.Second())

	/* Fill in the actual data */
	for i := 0; i < 120; i++ {
		data.Data.L1_e_vector_space[i].Fundamental_magnitude_volts = clampFloat32(math.Sin(float64(i) * 4.0 * math.Pi / 120))
		data.Data.L2_e_vector_space[i].Fundamental_magnitude_volts = clampFloat32(math.Sin(float64(i) * 2.0 * math.Pi / 120))
		data.Data.L3_e_vector_space[i].Fundamental_magnitude_volts = clampFloat32(math.Sin(float64(i) * 1.0 * math.Pi / 120))

		data.Data.C1_e_vector_space[i].Fundamental_magnitude_volts = clampFloat32(math.Y0(float64(i) * 15.0 / 120))
		data.Data.C2_e_vector_space[i].Fundamental_magnitude_volts = clampFloat32(math.Y1(float64(i) * 15.0 / 120))
		data.Data.C3_e_vector_space[i].Fundamental_magnitude_volts = clampFloat32(math.Yn(2, float64(i)*15.0/120))

		data.Data.L1_e_vector_space[i].Phase_in_degrees = rand.Float32()
		data.Data.L2_e_vector_space[i].Phase_in_degrees = clampFloat32(rand.NormFloat64())
		data.Data.L3_e_vector_space[i].Phase_in_degrees = clampFloat32(rand.ExpFloat64())

		data.Data.C1_e_vector_space[i].Phase_in_degrees = clampFloat32(x2sinxinv(float64(i-60) / 240))
		data.Data.C2_e_vector_space[i].Phase_in_degrees = clampFloat32(xinvsinxinv(float64(i-60) / 120))
		data.Data.C3_e_vector_space[i].Phase_in_degrees = clampFloat32(weierstrass(float64(i-60) / 60))

		data.Data.Status[i] = int32(1)
	}

	return data
}
