package main

import (
	"crypto/sha512"
	"encoding/ascii85"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

var rootPath string

var LevelTime = []time.Duration{
	0,
}

func init() {
	_, rootPath, _, _ = runtime.Caller(0)
	rootPath, _ = filepath.Abs(rootPath)
	rootPath = filepath.Dir(rootPath)

	rand.Seed(time.Now().UnixNano())

	base := 2.2
	var total time.Duration
	for i := 0.0; i < 12; i++ {
		t := time.Duration(float64(time.Hour*24) * math.Pow(base, i))
		LevelTime = append(LevelTime, t)
		total += t
		//fmt.Printf("%d %s %s\n", int(i+1), formatDuration(t), formatDuration(total))
	}
}

func main() {
	// lock
	ln, err := net.Listen("tcp", "127.0.0.1:61297")
	if err != nil {
		fmt.Printf("lock failed.\n")
		return
	}
	defer ln.Close()

	mem := &Memory{
		Concepts: make(map[string]*Concept),
		Connects: make(map[string]*Connect),
	}
	mem.Load()

	getPendingConnect := func(now time.Time) []*Connect {
		var connects []*Connect
		for _, connect := range mem.Connects {
			from := mem.Concepts[connect.From]
			if (from.What == WORD || from.What == SENTENCE) && from.Incomplete {
				continue
			}

			lastHistory := connect.Histories[len(connect.Histories)-1]
			if lastHistory.Time.Add(LevelTime[lastHistory.Level]).After(now) {
				continue
			}

			connects = append(connects, connect)
		}
		return connects
	}

	var cmd string
	if len(os.Args) == 1 {
		cmd = "train"
	} else {
		cmd = os.Args[1]
	}
	max := 22
	if len(os.Args) == 2 && regexp.MustCompile(`[0-9]+`).MatchString(os.Args[1]) {
		max, _ = strconv.Atoi(os.Args[1])
		cmd = "train"
	}

	// add audio files
	if cmd == "add" {
		if len(os.Args) < 3 {
			fmt.Printf("word or sentence?\n")
			os.Exit(0)
		}
		t := os.Args[2]
		var what int
		if strings.HasPrefix(t, "w") {
			what = WORD
		} else if strings.HasPrefix(t, "s") {
			what = SENTENCE
		} else {
			fmt.Printf("unknown type\n")
			os.Exit(0)
		}

		p := filepath.Join(rootPath, "files")
		fmt.Printf("files directory: %s\n", p)

		hasher := sha512.New()
		for _, f := range os.Args[2:] {
			data, err := ioutil.ReadFile(f)
			if err != nil {
				continue
			}
			f, _ = filepath.Abs(f)
			f = strings.TrimPrefix(f, p)
			hasher.Reset()
			hasher.Write(data)
			buf := make([]byte, ascii85.MaxEncodedLen(sha512.Size))
			l := ascii85.Encode(buf, hasher.Sum(nil))

			concept := &Concept{
				What:     AUDIO,
				File:     f,
				FileHash: string(buf[:l]),
			}
			exists := mem.AddConcept(concept)
			if exists {
				fmt.Printf("skip %s\n", f)
			} else {
				fmt.Printf("add %s %s\n", f, concept.FileHash)
				// add text concept
				textConcept := &Concept{
					What:       what,
					Incomplete: true,
					Serial:     mem.NextSerial(),
				}
				mem.AddConcept(textConcept)
				// add connect
				mem.AddConnect(&Connect{
					From: concept.Key(),
					To:   textConcept.Key(),
					Histories: []History{
						{Level: 0, Time: time.Now()},
					},
				})
				if what == WORD {
					mem.AddConnect(&Connect{
						From: textConcept.Key(),
						To:   concept.Key(),
						Histories: []History{
							{Level: 0, Time: time.Now()},
						},
					})
				}
			}
		}

		// show stat
		fmt.Printf("%d concepts, %d connects\n", len(mem.Concepts), len(mem.Connects))
		mem.Save()

		// train
	} else if cmd == "train" {
		connects := getPendingConnect(time.Now())
		// sort
		sort.Sort(ConnectSorter{connects, mem})
		if len(connects) > max {
			connects = connects[:max]
		}
		// ui
		//ui_qt(connects, mem)
		ui_gtk(connects, mem)

		// complete connection
	} else if cmd == "complete" {
		for _, connect := range mem.Connects {
			from := mem.Concepts[connect.From]
			if !from.Incomplete && from.Text != "" {
				continue
			}
			if from.What != WORD {
				continue
			}
			to := mem.Concepts[connect.To]
			fmt.Printf("%s\n", to.File)
			to.Play()
			fmt.Scanf("%s\n", &from.Text)
			if from.Text == "" {
				continue
			}
			from.Incomplete = false
			mem.Save()
		}

		// train history
	} else if cmd == "history" {
		counter := make(map[string]int)
		for _, connect := range mem.Connects {
			for _, entry := range connect.Histories {
				t := entry.Time
				counter[fmt.Sprintf("%04d-%02d-%02d", t.Year(), t.Month(), t.Day())]++
			}
		}
		var dates []string
		for date := range counter {
			dates = append(dates, date)
		}
		sort.Strings(dates)
		for _, date := range dates {
			fmt.Printf("%s %d\n", date, counter[date])
		}
	}

}

type ConnectSorter struct {
	l []*Connect
	m *Memory
}

func (self ConnectSorter) Len() int {
	return len(self.l)
}

func (self ConnectSorter) Less(i, j int) bool {
	left, right := self.l[i], self.l[j]
	leftLastHistory := left.Histories[len(left.Histories)-1]
	rightLastHistory := right.Histories[len(right.Histories)-1]
	leftLesson := self.getLesson(left)
	rightLesson := self.getLesson(right)
	leftLevelOrder := self.getLevelOrder(left)
	rightLevelOrder := self.getLevelOrder(right)
	if leftLevelOrder < rightLevelOrder {
		return true
	} else if leftLevelOrder > rightLevelOrder {
		return false
	} else if leftLevelOrder == rightLevelOrder && (leftLevelOrder == 1 || leftLevelOrder == 3) { // old connect
		if leftLastHistory.Level < rightLastHistory.Level { // review low level first
			return true
		} else if leftLastHistory.Level > rightLastHistory.Level {
			return false
		} else if leftLastHistory.Level == rightLastHistory.Level { // same level
			if leftLesson < rightLesson { // review earlier lesson first
				return true
			} else if leftLesson > rightLesson {
				return false
			} else { // randomize
				if rand.Intn(2) == 1 { // randomize
					return true
				}
				return false
			}
		}
	} else if leftLevelOrder == rightLevelOrder && leftLevelOrder == 2 { // new connect
		if leftLesson < rightLesson { // learn earlier lesson first
			return true
		} else if leftLesson > rightLesson {
			return false
		} else { // same lesson
			leftTypeOrder := self.getTypeOrder(left)
			rightTypeOrder := self.getTypeOrder(right)
			if leftTypeOrder < rightTypeOrder {
				return true
			} else if leftTypeOrder > rightTypeOrder {
				return false
			} else {
				return leftLastHistory.Time.Before(rightLastHistory.Time)
			}
			return true
		}
		return true
	}
	return false
}

var lessonPattern = regexp.MustCompile("[0-9]+")

func (self ConnectSorter) getLesson(c *Connect) string {
	audio := self.m.Concepts[c.From]
	if audio.What != AUDIO {
		audio = self.m.Concepts[c.To]
	}
	return lessonPattern.FindStringSubmatch(audio.File)[0]
}

func (self ConnectSorter) getTypeOrder(c *Connect) int {
	fromWhat := self.m.Concepts[c.From].What
	toWhat := self.m.Concepts[c.To].What
	if fromWhat == AUDIO && toWhat == WORD {
		return 1
	} else if fromWhat == AUDIO && toWhat == SENTENCE {
		return 2
	}
	return 3
}

func (self ConnectSorter) getLevelOrder(c *Connect) int {
	lastHistory := c.Histories[len(c.Histories)-1]
	if lastHistory.Level >= 7 { // 7 -
		return 3
	}
	if lastHistory.Level > 0 { // 1 - 6
		return 1
	}
	return 2 // 0
}

func (self ConnectSorter) Swap(i, j int) {
	self.l[i], self.l[j] = self.l[j], self.l[i]
}

func formatDuration(duration time.Duration) string {
	var ret string
	var m, h, d, y time.Duration
	m = duration / time.Minute
	if m >= 60 {
		h = m / 60
		m = m % 60
	}
	if h >= 24 {
		d = h / 24
		h = h % 24
	}
	if d > 365 {
		y = d / 365
		d = d % 365
	}
	if y > 0 {
		ret += fmt.Sprintf("%dyears.", y)
	}
	if d > 0 {
		ret += fmt.Sprintf("%ddays.", d)
	}
	if h > 0 {
		ret += fmt.Sprintf("%dhours.", h)
	}
	if m > 0 {
		ret += fmt.Sprintf("%dmins.", m)
	}
	return ret
}
