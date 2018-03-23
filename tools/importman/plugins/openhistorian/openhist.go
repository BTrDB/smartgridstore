//Package openhist provides a crude data-only import of openhistorian .d files.
//We do not handle metadata at this time
package openhist

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"time"

	"github.com/BTrDB/smartgridstore/tools/importman/plugins"
)

//The number of nanoseconds to add for each reading within a data block
const SpoofNanos = 33333333

type openhistfile struct {
	filename       string
	pointsArchived int32
	datablockSize  int32
	datablockCount int32
	blocks         []datablockDesc
	cursor         int
	reader         *bufio.Reader
}

type datablockDesc struct {
	typeID    int32
	timestamp float64
}
type openhist struct {
	files  []*openhistfile
	cursor int
	total  int64
}

func NewOpenHistorian(filenames []string) (plugins.DataSource, error) {
	if len(filenames) == 0 {
		return nil, fmt.Errorf("no files specified")
	}
	rv := &openhist{}
	for _, f := range filenames {
		ohf, err := openFile(f)
		if err != nil {
			return nil, fmt.Errorf("error processing file %s: %v", f, err)
		}
		rv.files = append(rv.files, ohf)
		rv.total += int64(ohf.pointsArchived)
	}
	return rv, nil
}

func openFile(filename string) (*openhistfile, error) {
	fl, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("could not open file: %v", err)
	}
	defer fl.Close()
	_, err = fl.Seek(-32, os.SEEK_END)
	if err != nil {
		return nil, fmt.Errorf("could not seek to FAT: %v", err)
	}
	FAT := make([]byte, 32)
	_, err = io.ReadFull(fl, FAT)
	if err != nil {
		return nil, fmt.Errorf("could not read FAT: %v", err)
	}

	rv := &openhistfile{
		filename: filename,
	}
	rv.pointsArchived = int32(binary.LittleEndian.Uint32(FAT[20:]))
	rv.datablockSize = int32(binary.LittleEndian.Uint32(FAT[24:])) * 1024
	rv.datablockCount = int32(binary.LittleEndian.Uint32(FAT[28:]))
	rv.blocks = make([]datablockDesc, rv.datablockCount)
	_, err = fl.Seek(-(32 + int64(rv.datablockCount)*12), os.SEEK_END)
	br := bufio.NewReader(fl)
	for i := 0; i < int(rv.datablockCount); i++ {
		rec := make([]byte, 12)
		_, err := io.ReadFull(br, rec)
		if err != nil {
			return nil, fmt.Errorf("could not read FAT body: %v", err)
		}
		rv.blocks[i].typeID = int32(binary.LittleEndian.Uint32(rec[:4]))
		rv.blocks[i].timestamp = math.Float64frombits(binary.LittleEndian.Uint64(rec[4:]))
	}
	//
	// _, err = fl.Seek(0, os.SEEK_SET)
	// if err != nil {
	// 	return nil, fmt.Errorf("could not seek to begin: %v", err)
	// }
	// rv.reader = bufio.NewReaderSize(fl, 16*1024*1024)
	// rv.cursor = 0
	rv.cursor = 0
	return rv, nil
}

func (oh *openhist) Next() []plugins.Stream {
	if oh.cursor == len(oh.files) {
		return nil
	}
	rv := oh.files[oh.cursor].Streams()
	oh.cursor++
	return rv
}

func (oh *openhist) Total() (int64, bool) {
	return oh.total, true
}
func (oh *openhistfile) Streams() []plugins.Stream {
	fl, err := os.Open(oh.filename)
	if err != nil {
		fmt.Printf("could not reopen file %v\n", err)
		os.Exit(1)
	}
	defer fl.Close()
	oh.reader = bufio.NewReaderSize(fl, 16*1024*1024)
	rvmap := make(map[int32]*ohstream)
	epoch, err := time.Parse(time.RFC3339, "1995-01-01T00:00:00+00:00")
	if err != nil {
		panic(err)
	}
	epochnanos := epoch.UnixNano()
	for datablock := 0; datablock < int(oh.datablockCount); datablock++ {
		dblock := make([]byte, oh.datablockSize)
		stream, ok := rvmap[oh.blocks[datablock].typeID]
		if !ok {
			stream = &ohstream{
				typeID: int(oh.blocks[datablock].typeID),
				points: make([]plugins.Point, 0, oh.datablockSize/10),
			}
			rvmap[oh.blocks[datablock].typeID] = stream
		}
		_, err := io.ReadFull(oh.reader, dblock)
		if err != nil {
			fmt.Printf("critical error: %v\n", err)
			os.Exit(1)
		}
		for offset := 0; (offset + 10) < int(oh.datablockSize); offset += 10 {
			index := offset / 10
			timestamp := epochnanos + int64(oh.blocks[datablock].timestamp*1e3)*1e6 + SpoofNanos*int64(index)
			//Lets also round the timestamp to the nearest millisecond
			timestamp = ((timestamp + 500e3) / 1e6) * 1e6
			value := math.Float32frombits(binary.LittleEndian.Uint32(dblock[offset+6:]))
			stream.points = append(stream.points, plugins.Point{Time: timestamp, Value: float64(value)})
		}
	}

	rv := make([]plugins.Stream, 0, len(rvmap))
	for _, s := range rvmap {
		rv = append(rv, s)
	}
	return rv
}

type ohstream struct {
	typeID       int
	points       []plugins.Point
	haveReturned bool
}

//The CollectionSuffix is what will be appended onto the user specified
//destination collection. It can be an empty string as long as the Tags
//are unique for all streams, otherwise the combination of CollectionSuffix
//and Tags must be unique
func (s *ohstream) CollectionSuffix() string {
	return ""
}

//The Tags form part of the identity of the stream. Specifically if there
//is a `name` tag, it is used in the plotter as the final element of the
//tree.
func (s *ohstream) Tags() map[string]string {
	return map[string]string{"name": fmt.Sprintf("id_%d", s.typeID)}
}

//Annotations contain additional metadata that is associated with the stream
//but is changeable or otherwise not suitable for identifying the stream
func (s *ohstream) Annotations() map[string]string {
	return nil
}

//Next returns a chunk of data for insertion. If the data is empty it is
//assumed that there is no more data to insert
func (s *ohstream) Next() (data []plugins.Point) {
	if s.haveReturned {
		return nil
	}
	s.haveReturned = true
	return s.points
}

//Total returns the total number of datapoints, used for progress estimation.
//If no total is available, return 0, false
func (s *ohstream) Total() (total int64, totalKnown bool) {
	//We could calculate this, but we don't right now
	return int64(len(s.points)), true
}
