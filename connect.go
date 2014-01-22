package main

import (
	"fmt"
	"time"
)

type Connect struct {
	From        string
	To          string
	Level       int
	LevelUpTime time.Time
}

func (self *Connect) Key() string {
	return fmt.Sprintf("%s\t%s", self.From, self.To)
}
