// Copyright (c) 2021 Michael Andersen
// Copyright (c) 2021 Regents of the University Of California
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package main

// #cgo CPPFLAGS: -I${SRCDIR}/thirdparty/Include
// #cgo LDFLAGS: -L${SRCDIR}/thirdparty/Libraries -L${SRCDIR}/thirdparty/Libraries/boost -lgsf -lboost_iostreams -lboost_thread -lboost_system -lpthread
// #include "adapter.h"
// #include <stdlib.h>
import "C"
import (
	"fmt"
	"reflect"
	"unsafe"
)

//void cbMeasurement(uint64_t id, const measurement_t *measurements, size_t count)

//export AdapterMeasurement
func AdapterMeasurement(id C.uint64_t, mz C.measurement_arg, cnt C.size_t) {
	globalmu.RLock()
	d, ok := globalmap[uint64(id)]
	globalmu.RUnlock()
	if !ok {
		fmt.Printf("Critical: unmapped device %d\n", id)
		return
	}
	for i := 0; i < int(cnt); i++ {
		m := C.measurement_at(mz, C.uint64_t(i))
		gom := &Measurement{
			ID:        uint32(m.ID),
			Source:    C.GoString(m.Source),
			SignalID:  C.GoString(m.SignalID),
			Tag:       C.GoString(m.Tag),
			Value:     float64(m.Value),
			Timestamp: int64(m.Timestamp),
			Flags:     uint32(m.Flags),
		}
		d.Measurement(gom)
	}
}

//export AdapterMetadata
func AdapterMetadata(id C.uint64_t, dat *C.uint8_t, count C.size_t) {
	var data []byte
	sliceHeader := (*reflect.SliceHeader)((unsafe.Pointer(&data)))
	sliceHeader.Cap = int(count)
	sliceHeader.Len = int(count)
	sliceHeader.Data = uintptr(unsafe.Pointer(dat))
	globalmu.RLock()
	d, ok := globalmap[uint64(id)]
	globalmu.RUnlock()
	if ok {
		d.Metadata(data)
	} else {
		fmt.Printf("Critical: unmapped device %d\n", id)
	}
}

//export AdapterMessage
func AdapterMessage(id C.uint64_t, isError bool, message *C.char) {
	gomessage := C.GoString(message)
	globalmu.RLock()
	d, ok := globalmap[uint64(id)]
	globalmu.RUnlock()
	if ok {
		d.Message(isError, gomessage)
	} else {
		fmt.Printf("Critical: unmapped device %d\n", id)
	}
}

//export AdapterFailed
func AdapterFailed(id C.uint64_t) {
	globalmu.RLock()
	d, ok := globalmap[uint64(id)]
	globalmu.RUnlock()
	if ok {
		d.Failed()
	} else {
		fmt.Printf("Critical: unmapped device %d\n", id)
	}
}

func Abort(id uint64) {
	C.abort_driver(C.uint64_t(id))
}

func RequestMetadata(id uint64) {
	C.request_metadata(C.uint64_t(id))
}

func NewDriver(id uint64, host string, port uint16, expression string) bool {
	chost := C.CString(host)
	cexpression := C.CString(expression)
	okay := C.new_driver(C.uint64_t(id), chost, C.uint16_t(port), cexpression)
	C.free(unsafe.Pointer(chost))
	C.free(unsafe.Pointer(cexpression))
	return okay != 0
}

// func main() {
// 	host := C.CString("52.10.177.108")
// 	expression := C.CString("FILTER ActiveMeasurements WHERE SignalID LIKE '%'")
//
// 	//bool new_driver(uint64_t id, const char* host, uint16_t port, const char* expression)
// 	C.new_driver(1, host, 6165, expression)
//
// 	C.free(unsafe.Pointer(host))
// 	C.free(unsafe.Pointer(expression))
// 	for {
// 		time.Sleep(5 * time.Second)
// 	}
// }
