package main

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BTrDB/smartgridstore/tools/gen2ingress"
	"github.com/BTrDB/smartgridstore/tools/manifest"
	btrdb "gopkg.in/BTrDB/btrdb.v4"
)

var globalmu sync.RWMutex
var globalmap map[uint64]*GEPDevice

func init() {
	globalmap = make(map[uint64]*GEPDevice)
}

//Driver definition
type GEPIngress struct {
	inserter   *gen2ingress.Inserter
	identifier uint64
	mu         sync.Mutex
}

func (gep *GEPIngress) DIDPrefix() string {
	return "gep.generic.client"
}
func (gep *GEPIngress) InitiatesConnections() bool {
	return true
}
func (gep *GEPIngress) SetConn(in *gen2ingress.Inserter) {
	gep.inserter = in
}

func (gep *GEPIngress) HandleDevice(ctx context.Context, descriptor string) error {
	//Format: gep.generic.client/collection/prefix@ip:port?FILTER EXPRESSION
	suffix := strings.TrimPrefix(descriptor, gep.DIDPrefix())
	parts := strings.SplitN(suffix, "@", 2)
	if len(parts) != 2 {
		return fmt.Errorf("descriptor is malformed")
	}
	collectionPrefix := strings.TrimPrefix(parts[0], "/")
	collectionPrefix = strings.TrimSuffix(collectionPrefix, "/")
	if len(collectionPrefix) == 0 {
		return fmt.Errorf("device missing collection")
	}
	afterAtParts := strings.SplitN(parts[1], "?", 2)
	if len(afterAtParts) != 2 {
		return fmt.Errorf("descriptor is malformed")
	}
	host, port, err := net.SplitHostPort(afterAtParts[0])
	if err != nil {
		return fmt.Errorf("host:ip is malformed")
	}
	porti, err := strconv.ParseInt(port, 10, 16)
	if err != nil {
		return fmt.Errorf("host:ip is malformed")
	}
	expression := afterAtParts[1]
	//Special case to make common device descriptor cleaner
	if expression == "*" {
		expression = "FILTER ActiveMeasurements WHERE SignalID LIKE '%'"
	}

	device := &GEPDevice{
		descriptor:       descriptor,
		inserter:         gep.inserter,
		driver:           gep,
		collectionPrefix: collectionPrefix,
		host:             host,
		port:             uint16(porti),
		expression:       expression,
	}

	go gen2ingress.Loop(ctx, device.descriptor, device.begin)
	return nil
}

type GEPDevice struct {
	//	ctx              context.Context
	descriptor       string
	inserter         *gen2ingress.Inserter
	driver           *GEPIngress
	collectionPrefix string
	host             string
	port             uint16
	expression       string
	identifier       uint64

	chMeasurement chan *Measurement
	chMetadata    chan []byte
	chMessage     chan msg
	chFailed      chan bool

	metamap     map[string]*CMeasurementDetail
	metaraw     *CNewDataSet
	metachanged map[string]time.Time
}

type msg struct {
	IsError bool
	Msg     string
}
type Measurement struct {
	ID        uint32
	Source    string
	SignalID  string
	Tag       string
	Value     float64
	Timestamp int64
	Flags     uint32
}

func (d *GEPDevice) begin(ctx context.Context) error {
	//Obtain a new device ID
	d.driver.mu.Lock()
	d.driver.identifier++
	did := d.driver.identifier
	d.identifier = did
	d.driver.mu.Unlock()
	globalmu.Lock()
	globalmap[did] = d
	globalmu.Unlock()
	defer func() {
		globalmu.Lock()
		delete(globalmap, did)
		globalmu.Unlock()
	}()

	d.chMeasurement = make(chan *Measurement, 1000)
	d.chMetadata = make(chan []byte, 10)
	d.chMessage = make(chan msg, 100)
	d.chFailed = make(chan bool, 10)

	//Refresh the device metadata periodically
	go d.periodicallAskForMetadata(ctx)

	shortform := manifest.GetDescriptorShortForm(d.descriptor)
	okay := NewDriver(d.identifier, d.host, d.port, d.expression)
	if !okay {
		return fmt.Errorf("failed first connection")
	}
	//This function is supposed to synchronously handle the device
	lastMeasurementFailNotification := time.Time{}
	accumulatedMFail := 0
	for {
		select {
		case <-ctx.Done():
			//There is a race in the GEP driver. Delaying sidesteps that
			time.Sleep(1 * time.Second)
			Abort(d.identifier)
			return ctx.Err()
		case m := <-d.chMessage:
			t := "INFO"
			if m.IsError {
				t = "ERROR"
			}
			fmt.Printf("[%s] GEP %s: %s\n", shortform, t, m.Msg)
		case md := <-d.chMetadata:
			err := d.processMetadata(md)
			if err != nil {
				fmt.Printf("[%s] metadata parse fail: %s\n", shortform, err)
			}
		case m := <-d.chMeasurement:
			err := d.processMeasurement(m)
			if err != nil {
				accumulatedMFail++
				if time.Since(lastMeasurementFailNotification) > 30*time.Second {
					fmt.Printf("[%s] measurement process fail: %s (repeated %d times)\n", shortform, err, accumulatedMFail)
					lastMeasurementFailNotification = time.Now()
					accumulatedMFail = 0
					//This could potentially happen because a new device came online
					//and we don't have it's metadata
					RequestMetadata(d.identifier)
				}
			}
		case <-d.chFailed:
			time.Sleep(1 * time.Second)
			Abort(d.identifier)
			return fmt.Errorf("connection failed")
		}
	}
}

func (d *GEPDevice) processMetadata(data []byte) error {
	//We know we are not concurrently running with processMeasurement so we
	//don't need any locks
	raw, metamap, err := ParseXMLMetadata(data)
	if err != nil {
		return err
	}
	d.metamap = metamap
	d.metaraw = raw
	return nil
}

func (d *GEPDevice) periodicallAskForMetadata(ctx context.Context) {
	for {
		//Check every 3 hours
		for i := 0; i < 6*60*3; i++ {
			time.Sleep(10 * time.Second)
			if ctx.Err() != nil {
				return
			}
		}
		RequestMetadata(d.identifier)
	}
}

//Unlike Measurement() below, this can take as long as it needs
func (d *GEPDevice) processMeasurement(m *Measurement) error {
	//Look up the metadata for the measurement
	md := d.metamap[m.SignalID]
	if md == nil {
		return fmt.Errorf("no metadata found")
	}

	//Work out if metadata has changed since we last told the inserter
	t, err := time.Parse(time.RFC3339, md.CUpdatedOn)
	if err != nil {
		return fmt.Errorf("invalid metadata timestamp")
	}
	if md.PhasorDetail != nil {
		t2, err := time.Parse(time.RFC3339, md.PhasorDetail.CUpdatedOn)
		if err != nil {
			return fmt.Errorf("invalid metadata timestamp")
		}
		if t2.After(t) {
			t = t2
		}
	}
	requireMDUpdate := true
	lastupdate, tok := d.metachanged[m.SignalID]
	if tok {
		if !t.After(lastupdate) {
			requireMDUpdate = false
		}
	}

	var attributes map[string]string
	if requireMDUpdate {
		//NOTE you can't assume the md.DeviceDetail is there
		attributes = make(map[string]string)
		attributes["description"] = md.CDescription
		attributes["reference"] = md.CSignalReference
		attributes["acronym"] = md.CSignalAcronym
		attributes["devacronym"] = md.CDeviceAcronym
		attributes["id"] = md.CSignalID
		if md.CEnabled {
			attributes["enabled"] = "true"
		} else {
			attributes["enabled"] = "false"
		}
		if md.CInternal {
			attributes["internal"] = "true"
		} else {
			attributes["internal"] = "false"
		}

		if md.PhasorDetail != nil {
			attributes["label"] = md.PhasorDetail.CLabel
			attributes["phase"] = md.PhasorDetail.CPhase
			attributes["type"] = md.PhasorDetail.CType
		}
	}
	dacro := md.CDeviceAcronym
	if dacro == "" {
		dacro = "NODEVICE"
	}
	//I just decided this, I could be wrong
	name := md.CSignalReference
	name = strings.Replace(name, "/", "-", -1)
	ir := gen2ingress.InsertRecord{
		Data:              []btrdb.RawPoint{{Time: m.Timestamp, Value: m.Value}},
		Flags:             []uint64{uint64(m.Flags)},
		Name:              name,
		Collection:        d.collectionPrefix + "/" + dacro,
		Unit:              md.CSignalAcronym, //TODO fix this
		AnnotationChanges: attributes,
	}
	d.inserter.ProcessBatch([]gen2ingress.InsertRecord{ir})
	return nil
}

//For all of these functions: be quick with the data, we are in the C
//context
func (d *GEPDevice) Measurement(m *Measurement) {
	d.chMeasurement <- m
}

func (d *GEPDevice) Metadata(dat []byte) {
	//We need to copy the byte array. It's backed by C
	clone := make([]byte, len(dat))
	copy(clone[:], dat)
	d.chMetadata <- clone
}

func (d *GEPDevice) Message(isError bool, message string) {
	d.chMessage <- msg{isError, message}
}

func (d *GEPDevice) Failed() {
	d.chFailed <- true
}

func main() {
	gen2ingress.Gen2Ingress(&GEPIngress{})
	fmt.Printf("this should not have happened :/\n")
}

// func (d *GEPDevice) abortOnCtxDone() {
// 	<-d.ctx.Done()
// 	time.Sleep(1 * time.Second)
// 	Abort(d.identifier)
// }
