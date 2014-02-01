package main

import (
	"fmt"
	"time"
)

type History struct {
	Level int
	Time  time.Time
}

type Connect struct {
	From      string
	To        string
	Histories []History
}

func (self *Connect) Key() string {
	return fmt.Sprintf("%s\t%s", self.From, self.To)
}
