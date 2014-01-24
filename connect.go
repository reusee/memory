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

func (self *Connect) Dump(mem *Memory) {
	fmt.Printf("%d ", self.Level)
	from := mem.Concepts[self.From]
	to := mem.Concepts[self.To]
	fmt.Printf("%s <%s> <%s>\n", whatName[from.What], from.Text, from.File)
	fmt.Printf("\t%s <%s> <%s>\n", whatName[to.What], to.Text, to.File)
}
