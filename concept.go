package main

import (
	"fmt"
)

const (
	SOUND = iota
	WORD
	SENTENCE
)

type Concept struct {
	Serial     int
	What       int
	Text       string
	File       string
	FileHash   string
	Incomplete bool
}

func (self *Concept) Key() string {
	switch self.What {
	case WORD, SENTENCE:
		return fmt.Sprintf("text\t%d", self.Serial)
	case SOUND:
		return fmt.Sprintf("file\t%s", self.FileHash)
	default:
		panic("not here")
	}
}
