package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

const (
	AUDIO = iota
	WORD
	SENTENCE
)

var whatName = map[int]string{
	AUDIO:    "audio",
	WORD:     "word",
	SENTENCE: "sentence",
}

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
	case AUDIO:
		return fmt.Sprintf("file\t%s", self.FileHash)
	default:
		panic("not here")
	}
}

func (self *Concept) Play() {
	cmd := exec.Command("mpv", filepath.Join(rootPath, "files", self.File))
	cmd.Run()
}
