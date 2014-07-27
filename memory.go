package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type Memory struct {
	Concepts map[string]*Concept
	Connects map[string]*Connect
	Serial   int
	saveLock sync.Mutex
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
	t0 := time.Now()
	self.saveLock.Lock()
	defer self.saveLock.Unlock()
	// encode
	buf := new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(self)
	if err != nil {
		log.Fatal(err)
	}
	// indent
	indented := new(bytes.Buffer)
	err = json.Indent(indented, buf.Bytes(), "", "    ")
	if err != nil {
		log.Fatal(err)
	}
	// write tmp file
	tmpPath := filepath.Join(rootPath, fmt.Sprintf("db.%d", rand.Int63()))
	out, err := os.Create(tmpPath)
	if err != nil {
		log.Fatal(err)
	}
	_, err = out.Write(indented.Bytes())
	if err != nil {
		log.Fatal(err)
	}
	out.Close()
	// rename
	filePath := filepath.Join(rootPath, "db.json")
	err = os.Rename(tmpPath, filePath)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("saved in %v.\n", time.Now().Sub(t0))
}

func (self *Memory) Load() {
	filePath := filepath.Join(rootPath, "db.json")
	f, err := os.Open(filePath)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		log.Fatal(err)
	}
	err = json.NewDecoder(f).Decode(self)
	if err != nil {
		log.Fatal(err)
	}
}
