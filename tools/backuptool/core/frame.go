// Copyright (c) 2021 Michael Andersen
// Copyright (c) 2021 Regents of the University Of California
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package core

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/golang/snappy"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/crypto/sha3"
)

/*
A backup archive consists of N numbered files and a header file.
To recover the backup, you process the N numbered files in order.
Each of them has an arbitrary number of frames inside and each one
describes an operation on ceph or a set of operations on etcd.
Each frame is individually encrypted with GCM
*/
type Frame struct {
	Type    int
	Content []byte
}

var Sentinel = []byte{0xbd, 0xdb, 0x55, 0xaa}

const salt = "b9c4c1ee74b1db7b88c11f6dd8cab7196840ef2591d8885a313d2bf5d2866107"

//Wire header: 4b sentinel, 4b type, 8b length, 16b nonce

type FrameWriter struct {
	nonce     uint64
	noncetopb []byte
	aesk      []byte
	block     cipher.Block
	gcm       cipher.AEAD
}

func NewFrameWriter(passphrase string) *FrameWriter {
	fmt.Printf("generating (enc/dec)ryption keys from passphrase...\n")
	then := time.Now()
	keymaterial := pbkdf2.Key([]byte(passphrase), []byte(salt), 200000, 40, sha3.New512)
	fmt.Printf("done %s\n", time.Now().Sub(then))
	aesk := keymaterial[0:32]
	startingnonce := make([]byte, 8)
	rand.Read(startingnonce)

	block, err := aes.NewCipher(aesk)
	if err != nil {
		panic(err)
	}
	aesgcm, err := cipher.NewGCMWithNonceSize(block, 16)
	if err != nil {
		panic(err.Error())
	}
	return &FrameWriter{
		nonce:     0,
		noncetopb: startingnonce,
		aesk:      aesk,
		block:     block,
		gcm:       aesgcm,
	}
}

func (fw *FrameWriter) Write(to io.Writer, Type int, Content []byte) error {
	content := make([]byte, 0, len(Content)+40)
	ournonce := atomic.AddUint64(&fw.nonce, 1)
	nonceb := make([]byte, 16)
	copy(nonceb[0:8], fw.noncetopb)
	binary.BigEndian.PutUint64(nonceb[8:16], ournonce)
	CompressedContent := snappy.Encode(nil, Content)
	ciphertext := fw.gcm.Seal(nil, nonceb, CompressedContent, nil)
	ciphertextlen := make([]byte, 8)
	binary.BigEndian.PutUint64(ciphertextlen[0:8], uint64(len(ciphertext)))
	typebytes := make([]byte, 4)
	binary.BigEndian.PutUint32(typebytes, uint32(Type))
	content = append(content, Sentinel...)
	content = append(content, typebytes...)
	content = append(content, ciphertextlen...)
	content = append(content, nonceb...)
	content = append(content, ciphertext...)
	_, err := to.Write(content)
	return err
}

func (fw *FrameWriter) Read(in io.Reader) (*Frame, error) {
	//read header
	hdr := make([]byte, 32)
	_, err := io.ReadFull(in, hdr)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(hdr[0:4], Sentinel) {
		return nil, fmt.Errorf("File format is incorrect")
	}
	Type := binary.BigEndian.Uint32(hdr[4:8])
	CTlen := binary.BigEndian.Uint64(hdr[8:16])
	if CTlen > 1*1024*1024*1024 {
		return nil, fmt.Errorf("very large frame found")
	}
	Nonce := hdr[16:32]
	ciphertext := make([]byte, CTlen)
	_, err = io.ReadFull(in, ciphertext)
	plaintext, err := fw.gcm.Open(nil, Nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt, invalid passphrase?")
	}
	DecompressedPlaintxt, err := snappy.Decode(nil, plaintext)
	ChkErr(err)
	return &Frame{
		Type:    int(Type),
		Content: DecompressedPlaintxt,
	}, nil
}
