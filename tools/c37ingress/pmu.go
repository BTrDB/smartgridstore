package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

const QueueSize = 16000
const MaxBatch = 1000

type PMU struct {
	ctx       context.Context
	ctxcancel func()
	address   string
	id        uint16
	nickname  string
	conn      *net.TCPConn
	br        *bufio.Reader

	currentconfig *Config12Frame

	cfgmu sync.Mutex
	cfgs  map[uint16]*Config12Frame

	outputmu sync.RWMutex
	output   map[uint16]chan *DataFrame
}

func CreatePMU(address string, id uint16) *PMU {
	rv := &PMU{
		address:  address,
		nickname: fmt.Sprintf("%d@%s", id, address),
		id:       id,
		cfgs:     make(map[uint16]*Config12Frame),
		output:   make(map[uint16]chan *DataFrame),
	}
	go rv.dialloop()
	return rv
}

func (p *PMU) dialloop() {
	for {
		fmt.Printf("[%s] beginning dial\n", p.nickname)
		err := p.dial()
		fmt.Printf("[%s] fatal error: %v\n", p.nickname, err)
		time.Sleep(10 * time.Second)
		fmt.Printf("[%s] backoff over, reconnecting\n", p.nickname)
	}
}

func (p *PMU) dial() (err error) {
	defer func() {
		r := recover()
		if r != nil {
			err = fmt.Errorf("[%s] panic: %v", p.nickname, r)
		}
	}()
	p.ctx, p.ctxcancel = context.WithCancel(context.Background())
	addr, err := net.ResolveTCPAddr("tcp", p.address)
	if err != nil {
		return err
	}
	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		return err
	}
	fmt.Printf("[%s] dial succeeded\n", p.nickname)

	p.conn = conn
	p.br = bufio.NewReader(p.conn)
	p.initialConfigure()
	return p.process()
}

func (p *PMU) process() error {
	for {
		ch, frame, err := p.readFrame()
		if err != nil {
			fmt.Printf("[%s] frame read error: %v\n", p.nickname, err)
			p.conn.Close()
			return err
		}
		_ = ch
		cfg, ok := frame.(*Config12Frame)
		if ok {
			p.cfgmu.Lock()
			p.cfgs[ch.IDCODE] = cfg
			p.cfgmu.Unlock()
			p.sendStartCommand()
		}
		dat, ok := frame.(*DataFrame)
		if ok {
			p.outputmu.RLock()
			ochan, ok := p.output[dat.IDCODE]
			p.outputmu.RUnlock()
			if !ok {
				p.outputmu.Lock()
				ochan = make(chan *DataFrame, QueueSize)
				p.output[dat.IDCODE] = ochan
				p.outputmu.Unlock()
			}
			select {
			case ochan <- dat:
			default:
				fmt.Printf("[%s] WARNING QUEUE OVERFLOW. DROPPING DATA FROM %d\n", p.nickname, dat.IDCODE)
			}
		}
	}
}

func (p *PMU) GetBatch() (map[uint16][]*DataFrame, bool) {
	fulldrain := true
	chanz := make(map[uint16]chan *DataFrame)
	rv := make(map[uint16][]*DataFrame)
	p.outputmu.RLock()
	for k, v := range p.output {
		chanz[k] = v
	}
	p.outputmu.RUnlock()
	for idcode, ch := range chanz {
		slice := make([]*DataFrame, 0, MaxBatch)
		drained := false
	batchloop:
		for i := 0; i < MaxBatch; i++ {
			select {
			case d := <-ch:
				slice = append(slice, d)
			default:
				drained = true
				break batchloop
			}
		}
		if !drained {
			fulldrain = false
		}
		rv[idcode] = slice
	}
	return rv, fulldrain
}

func (p *PMU) initialConfigure() {
	time.Sleep(1 * time.Second)
	c := &CommandFrame{}
	c.IDCODE = p.id
	c.SetSOCToNow()
	c.SetSyncType(SYNC_TYPE_CMD)
	c.FRAMESIZE = CommonHeaderLength + 4
	c.CMD = uint16(CMD_SEND_CFG1)
	err := WriteChecksummedFrame(c, p.conn)
	if err != nil {
		panic(err)
	}
}

func (p *PMU) sendStartCommand() {
	c := &CommandFrame{}
	c.IDCODE = p.id
	c.SetSOCToNow()
	c.SetSyncType(SYNC_TYPE_CMD)
	c.FRAMESIZE = CommonHeaderLength + 4
	c.CMD = uint16(CMD_TURN_ON_TX)
	err := WriteChecksummedFrame(c, p.conn)
	if err != nil {
		panic(err)
	}
}

func (p *PMU) ConfigFor(idcode uint16) (*Config12Frame, error) {
	p.cfgmu.Lock()
	cfg, ok := p.cfgs[idcode]
	p.cfgmu.Unlock()
	if !ok {
		return nil, fmt.Errorf("No config found")
	}
	return cfg, nil
}
func (p *PMU) readFrame() (*CommonHeader, Frame, error) {
	r := p.br
	initialByte, err := r.ReadByte()
	if err != nil {
		return nil, nil, err
	}
	skipped := 0
	for initialByte != 0xAA {
		skipped++
		initialByte, err = r.ReadByte()
	}
	if skipped != 0 {
		fmt.Printf("[%s] SYNC LOSS DETECTED, SKIPPED %d BYTES RESYNCING\n", p.nickname, skipped)
	}
	raw := make([]byte, CommonHeaderLength)
	nread, err := io.ReadFull(r, raw[1:])
	if err != nil {
		return nil, nil, err
	}
	if nread != len(raw[1:]) {
		fmt.Printf("READ LEN MISMATCH %d vs %d\n", nread, len(raw[1:]))
	}
	raw[0] = initialByte
	rcopy := make([]byte, CommonHeaderLength)
	copy(rcopy, raw)

	ch, err := ReadCommonHeader(bytes.NewBuffer(rcopy))
	if err != nil {
		return nil, nil, err
	}

	rest := make([]byte, int(ch.FRAMESIZE)-CommonHeaderLength)
	_, err = io.ReadFull(r, rest)
	if err != nil {
		return nil, nil, err
	}

	raw = append(raw, rest[:len(rest)-2]...)
	realchk := Checksum(raw)
	expectedchk := (int(rest[len(rest)-2]) << 8) + int(rest[len(rest)-1])
	if expectedchk != int(realchk) {
		fmt.Printf("[%s] frame checksum failure type=%d, got=%x expected=%x\n", p.nickname, ch.SyncType(), realchk, expectedchk)
		//the spec says silently ignore frames with bad checksums
		return ch, nil, nil
	}
	if ch.SyncType() == SYNC_TYPE_CFG2 {
		cfg2, err := ReadConfig12Frame(ch, bytes.NewBuffer(rest))
		if err != nil {
			return nil, nil, err
		}
		return ch, cfg2, nil
	}
	if ch.SyncType() == SYNC_TYPE_DATA {
		cfg, _ := p.ConfigFor(ch.IDCODE)
		if cfg == nil {
			fmt.Printf("[%s] dropping data frame: no config\n", p.nickname)
			return ch, nil, nil
		}
		dat, err := ReadDataFrame(ch, cfg, bytes.NewBuffer(rest))
		if err != nil {
			return nil, nil, err
		}
		return ch, dat, nil
	}
	if ch.SyncType() == SYNC_TYPE_CFG1 {
		cfg1, err := ReadConfig12Frame(ch, bytes.NewBuffer(rest))
		if err != nil {
			return nil, nil, err
		}
		return ch, cfg1, nil
	}
	if ch.SyncType() == SYNC_TYPE_CFG3 {
		fmt.Printf("[%s] WARN got CFG3 which is not supported\n", p.nickname)
		return ch, nil, nil
	}
	return ch, nil, fmt.Errorf("Unknown frame type")
}
