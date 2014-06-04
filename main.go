package main

/*
#include <clutter/clutter.h>
#cgo pkg-config: clutter-1.0 gobject-2.0
*/
import "C"

import (
	"crypto/sha512"
	"encoding/ascii85"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unsafe"

	ct "./clutter-helper"
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

	base := 2.09
	var total time.Duration
	for i := 0.0; i < 12; i++ {
		t := time.Duration(float64(time.Hour*24) * math.Pow(base, i))
		LevelTime = append(LevelTime, t)
		total += t
		//fmt.Printf("%d %s %s\n", int(i+1), formatDuration(t), formatDuration(total))
	}
}

func main() {
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
	max := 80
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
						History{Level: 0, Time: time.Now()},
					},
				})
				if what == WORD {
					mem.AddConnect(&Connect{
						From: textConcept.Key(),
						To:   concept.Key(),
						Histories: []History{
							History{Level: 0, Time: time.Now()},
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
		ct.Init()
		_, path, _, _ := runtime.Caller(0)
		ui, err := ioutil.ReadFile(filepath.Join(filepath.Dir(path), "ui.lua"))
		if err != nil {
			log.Fatal(err)
		}
		bindings := ct.FromLua(string(ui))
		stage := (*C.ClutterActor)(bindings["stage"])
		gconnect(stage, "destroy", func() {
			ct.Quit()
		})
		keys := make(chan rune)
		gconnect(stage, "key-press-event", func(_, ev interface{}) {
			code := C.clutter_event_get_key_unicode((*C.ClutterEvent)(ev.(unsafe.Pointer)))
			select {
			case keys <- rune(code):
			default:
			}
		})
		setText := func(s string) {
			run(func() {
				C.clutter_text_set_markup(asText(bindings["text"]), toGStr(fmt.Sprintf(`<span font="32">%s</span>`, s)))
			})
		}
		setHint := func(s string) {
			run(func() {
				C.clutter_text_set_text(asText(bindings["hint"]), toGStr(s))
			})
		}
		setHistory := func(s string) {
			run(func() {
				C.clutter_text_set_text(asText(bindings["history"]), toGStr(s))
			})
		}
		go func() {
			ct.Main()
			os.Exit(0)
		}()

		setHint("press f to start")
		for {
			key := <-keys
			if key == 'f' {
				break
			}
		}
		setHint("")

		// train
		for _, connect := range connects {
			setHint("")
			setText("")

			var lines []string
			lastTime := time.Now()
			for i := len(connect.Histories) - 1; i >= 0; i-- {
				t := connect.Histories[i].Time
				lines = append(lines, formatDuration(lastTime.Sub(t)))
				lastTime = t
				lines = append(lines, fmt.Sprintf("%d %d-%02d-%02d %02d:%02d", connect.Histories[i].Level, t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute()))
			}
			setHistory(strings.Join(lines, "\n"))

			from := mem.Concepts[connect.From]
			to := mem.Concepts[connect.To]
			switch from.What {

			case AUDIO: // play audio
				setHint("playing...")
				from.Play()
				if to.What == WORD {
					setHint("press any key to show answer")
					<-keys
					setText(to.Text)
				}
			repeat:
				setHint("press G to levelup, T to reset level, Space to repeat")
			read_key:
				key := <-keys
				switch key {
				case 'g':
					lastHistory := connect.Histories[len(connect.Histories)-1]
					connect.Histories = append(connect.Histories, History{Level: lastHistory.Level + 1, Time: time.Now()})
					mem.Save()
				case 't':
					connect.Histories = append(connect.Histories, History{Level: 0, Time: time.Now()})
					mem.Save()
				case ' ':
					setHint("playing...")
					from.Play()
					setHint("")
					goto repeat
				default:
					goto read_key
				}

			case WORD: // show text
				setText(from.Text)
				setHint("press any key to play audio")
				<-keys
			repeat2:
				setHint("playing...")
				to.Play()
				setHint("press G to levelup, T to reset level, Space to repeat")
			read_key2:
				key := <-keys
				switch key {
				case 'g':
					lastHistory := connect.Histories[len(connect.Histories)-1]
					connect.Histories = append(connect.Histories, History{Level: lastHistory.Level + 1, Time: time.Now()})
					mem.Save()
				case 't':
					connect.Histories = append(connect.Histories, History{Level: 0, Time: time.Now()})
					mem.Save()
				case ' ':
					goto repeat2
				default:
					goto read_key2
				}

			default:
				panic("impossible")
			}
		}

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
		for date, _ := range counter {
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
	if leftLastHistory.Level > 0 && rightLastHistory.Level > 0 { // review first
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
	} else if leftLastHistory.Level > 0 && rightLastHistory.Level == 0 { // review first
		return true
	} else if leftLastHistory.Level == 0 && rightLastHistory.Level > 0 { // learn new later
		return false
	} else if leftLastHistory.Level == 0 && rightLastHistory.Level == 0 { // new connect
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
