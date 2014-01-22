package main

import (
	"fmt"
)

type Connect struct {
	From  string
	To    string
	Level int
}

func (self *Connect) Key() string {
	return fmt.Sprintf("%s\t%s", self.From, self.To)
}
