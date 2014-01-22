package main

import (
	"bytes"
	"compress/zlib"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type Memory struct {
	Concepts map[string]*Concept
	Connects map[string]*Connect
	Serial   int
}

func (self *Memory) AddConcept(concept *Concept) (exists bool) {
	if _, exists = self.Concepts[concept.Key()]; exists {
		return
	}
	self.Concepts[concept.Key()] = concept
	return
}

func (self *Memory) AddConnect(connect *Connect) (exists bool) {
	if _, exists = self.Connects[connect.Key()]; exists {
		return
	}
	self.Connects[connect.Key()] = connect
	return
}

func (self *Memory) NextSerial() int {
	self.Serial += 1
	return self.Serial
}

func (self *Memory) Save() {
	// gob
	buf := new(bytes.Buffer)
	err := gob.NewEncoder(buf).Encode(self)
	if err != nil {
		log.Fatal(err)
	}
	// compress
	zbuf := new(bytes.Buffer)
	w := zlib.NewWriter(zbuf)
	w.Write(buf.Bytes())
	w.Close()
	// write tmp file
	tmpPath := filepath.Join(rootPath, fmt.Sprintf("db.%d", rand.Int63()))
	out, err := os.Create(tmpPath)
	if err != nil {
		log.Fatal(err)
	}
	_, err = out.Write(zbuf.Bytes())
	if err != nil {
		log.Fatal(err)
	}
	out.Close()
	// test
	f, err := os.Open(tmpPath)
	if err != nil {
		log.Fatal(err)
	}
	zbuf.Reset()
	r, err := zlib.NewReader(f)
	if err != nil {
		log.Fatal(err)
	}
	io.Copy(zbuf, r)
	r.Close()
	mem := new(Memory)
	err = gob.NewDecoder(zbuf).Decode(mem)
	if err != nil {
		log.Fatal(err)
	}
	if len(mem.Concepts) != len(self.Concepts) || len(mem.Connects) != len(self.Connects) {
		log.Fatalf("save error")
	}
	// rename
	filePath := filepath.Join(rootPath, "db")
	err = os.Rename(tmpPath, filePath)
	if err != nil {
		log.Fatal(err)
	}
}

func (self *Memory) Load() {
	filePath := filepath.Join(rootPath, "db")
	f, err := os.Open(filePath)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		log.Fatal(err)
	}
	r, err := zlib.NewReader(f)
	if err != nil {
		log.Fatal(err)
	}
	buf := new(bytes.Buffer)
	io.Copy(buf, r)
	err = gob.NewDecoder(buf).Decode(self)
	if err != nil {
		log.Fatal(err)
	}
}
