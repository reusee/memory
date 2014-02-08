package main

import (
	"fmt"
	"testing"
	"time"
)

func TestMain(t *testing.T) {
	d := time.Now().Sub(time.Time{})
	fmt.Printf("%s\n", formatDuration(d))
}
