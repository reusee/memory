package main

import (
	"crypto/sha512"
	"encoding/ascii85"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	termbox "github.com/nsf/termbox-go"
)

var rootPath string

var LevelTime = []time.Duration{
	0,
	time.Minute * 30,
	time.Hour * 24,
	time.Hour * 24 * 2,
	time.Hour * 24 * 4,
	time.Hour * 24 * 8,
	time.Hour * 24 * 16,
	time.Hour * 24 * 32,
	time.Hour * 24 * 64,
	time.Hour * 24 * 128,
	time.Hour * 24 * 256,
	time.Hour * 24 * 512,
	time.Hour * 24 * 1024,
	time.Hour * 24 * 2048,
}

func init() {
	_, rootPath, _, _ = runtime.Caller(0)
	rootPath, _ = filepath.Abs(rootPath)
	rootPath = filepath.Dir(rootPath)

	rand.Seed(time.Now().UnixNano())
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("no command\n")
		os.Exit(0)
	}

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

			//if connect.Histories[0].Level != 0 { // old scheduler
			lastHistory := connect.Histories[len(connect.Histories)-1]
			if lastHistory.Time.Add(LevelTime[lastHistory.Level]).After(now) {
				continue
			}
			/*
				} else { // new scheduler
					if len(connect.Histories) == 1 { // new connect
					} else if len(connect.Histories) == 2 { // newly learnt
						if connect.Histories[1].Time.Add(time.Minute * 30).After(now) {
							continue
						}
					} else if len(connect.Histories) > 2 {
						t1 := connect.Histories[len(connect.Histories)-1].Time
						t2 := connect.Histories[len(connect.Histories)-2].Time
						dt := t1.Sub(t2) * 2
						if dt < time.Hour*24 {
							dt = time.Hour * 24
						}
						if t1.Add(dt).After(now) {
							continue
						}
					}
				}
			*/

			connects = append(connects, connect)
		}
		return connects
	}

	statConnects := func(conns interface{}) {
		c := make(map[int]int)
		switch cs := conns.(type) {
		case []*Connect:
			for _, conn := range cs {
				lastHistory := conn.Histories[len(conn.Histories)-1]
				c[lastHistory.Level]++
			}
		case map[string]*Connect:
			for _, conn := range cs {
				lastHistory := conn.Histories[len(conn.Histories)-1]
				c[lastHistory.Level]++
			}
		}
		for i := len(LevelTime) - 1; i >= 0; i-- {
			if c[i] > 0 {
				fmt.Printf("%d %d\n", i, c[i])
			}
		}
	}

	printHistories := func(connect *Connect, width, height int) {
		y := height/2 + 2
		lastTime := time.Now()
		for i := len(connect.Histories) - 1; i >= 0; i-- {
			t := connect.Histories[i].Time
			p(width/3, y, formatDuration(lastTime.Sub(t)))
			lastTime = t
			y++
			p(width/3, y, fmt.Sprintf("%d %d-%02d-%02d %02d:%02d", connect.Histories[i].Level, t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute()))
			y++
		}
	}

	cmd := os.Args[1]
	switch cmd {

	case "add": // add audio files
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

	case "train":
		connects := getPendingConnect(time.Now())
		// sort
		sort.Sort(ConnectSorter{connects, mem})
		// train
		err := termbox.Init()
		if err != nil {
			log.Fatal(err)
		}
		defer termbox.Close()
		width, height := termbox.Size()
		t0 := time.Now()
		for i, connect := range connects {
			if i > 80 || time.Now().Sub(t0) > time.Minute*8 {
				break
			}
			termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
			from := mem.Concepts[connect.From]
			to := mem.Concepts[connect.To]
			switch from.What {

			case AUDIO: // play audio
				printHistories(connect, width, height)
				p(width/3, height/2+1, ">                                                                                  ")
				from.Play()
				if to.What == WORD {
					p(width/3, height/2+1, "press any key to show text")
					termbox.PollEvent()
					p(width/3, height/2, to.Text)
				}
			repeat:
				p(width/3, height/2+1, "press Enter to levelup, Left to reset level, Tab to exit, other keys to repeat")
				ev := termbox.PollEvent()
				switch ev.Key {
				case termbox.KeyEnter:
					lastHistory := connect.Histories[len(connect.Histories)-1]
					connect.Histories = append(connect.Histories, History{Level: lastHistory.Level + 1, Time: time.Now()})
					mem.Save()
				case termbox.KeyArrowLeft:
					connect.Histories = append(connect.Histories, History{Level: 0, Time: time.Now()})
					mem.Save()
				case termbox.KeyTab:
					return
				default:
					p(width/3, height/2+1, ">                                                                                 ")
					from.Play()
					p(width/3, height/2+1, "                                                                                  ")
					goto repeat
				}

			case WORD: // show text
				p(width/3, height/2, from.Text)
				printHistories(connect, width, height)
				p(width/3, height/2+1, "press any key to play audio")
				termbox.PollEvent()
				to := mem.Concepts[connect.To]
			repeat2:
				p(width/3, height/2+1, ">                                                                                   ")
				termbox.Flush()
				to.Play()
				p(width/3, height/2+1, "press Enter to levelup, Left to reset level, Tab to exit, other keys to repeat")
				ev := termbox.PollEvent()
				switch ev.Key {
				case termbox.KeyEnter:
					lastHistory := connect.Histories[len(connect.Histories)-1]
					connect.Histories = append(connect.Histories, History{Level: lastHistory.Level + 1, Time: time.Now()})
					mem.Save()
				case termbox.KeyArrowLeft:
					connect.Histories = append(connect.Histories, History{Level: 0, Time: time.Now()})
					mem.Save()
				case termbox.KeyTab:
					return
				default:
					goto repeat2
				}
			default:
				panic("fixme") //TODO
			}
		}

	case "complete":
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
			from.Incomplete = false
			mem.Save()
		}

	case "stat":
		// concepts
		var total, word, sen, audio int
		for _, concept := range mem.Concepts {
			total++
			switch concept.What {
			case WORD:
				word++
			case SENTENCE:
				sen++
			case AUDIO:
				audio++
			}
		}
		fmt.Printf("%d concepts\n", total)
		fmt.Printf("%d words\n", word)
		fmt.Printf("%d sentences\n", sen)
		fmt.Printf("%d audios\n", audio)
		fmt.Printf("\n")

		// connectes
		statConnects(mem.Connects)
		fmt.Printf("%d connects\n\n", len(mem.Connects))

		cs := getPendingConnect(time.Now())
		statConnects(cs)
		fmt.Printf("%d pending\n", len(cs))

		// future
		print("\nfuture\n")
		last := 0
		for i := 0; i < 48; i++ {
			t := time.Now().Add(time.Hour * time.Duration(i))
			n := len(getPendingConnect(t))
			if n != last {
				fmt.Printf("%-5d %04d-%02d-%02d %02d:%02d\n", n-last, t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute())
			}
			last = n
		}

	default:
		fmt.Printf("unknown command\n")
		os.Exit(0)
	}
}

func p(x, y int, text string) {
	for _, r := range text {
		termbox.SetCell(x, y, r, termbox.ColorDefault, termbox.ColorDefault)
		x += wcwidth(r)
	}
	termbox.Flush()
}

func wcwidth(r rune) int {
	if r >= 0x1100 &&
		(r <= 0x115f || r == 0x2329 || r == 0x232a ||
			(r >= 0x2e80 && r <= 0xa4cf && r != 0x303f) ||
			(r >= 0xac00 && r <= 0xd7a3) ||
			(r >= 0xf900 && r <= 0xfaff) ||
			(r >= 0xfe30 && r <= 0xfe6f) ||
			(r >= 0xff00 && r <= 0xff60) ||
			(r >= 0xffe0 && r <= 0xffe6) ||
			(r >= 0x20000 && r <= 0x2fffd) ||
			(r >= 0x30000 && r <= 0x3fffd)) {
		return 2
	}
	return 1
}

type ConnectSorter struct {
	l []*Connect
	m *Memory
}

func (self ConnectSorter) Len() int {
	return len(self.l)
}

func (self ConnectSorter) pri(connect *Connect) int {
	lastHistory := connect.Histories[len(connect.Histories)-1]
	if lastHistory.Level > 1 {
		return lastHistory.Level*(-1000) - rand.Intn(1000)
	} else if lastHistory.Level == 1 {
		return -10000000 - rand.Intn(1024)
	}

	n := 0
	from := self.m.Concepts[connect.From]
	to := self.m.Concepts[connect.To]

	if from.What == WORD {
		n += 200
	} else if from.What == SENTENCE {
		n += 400
	} else if from.What == AUDIO {
		if to.What == SENTENCE {
			n += 100
		}
	}

	return n
}

func (self ConnectSorter) Less(i, j int) bool {
	x, y := self.pri(self.l[i]), self.pri(self.l[j])
	if x == y {
		return self.l[i].Histories[len(self.l[i].Histories)-1].Time.Before(self.l[j].Histories[len(self.l[j].Histories)-1].Time)
	}
	return x < y
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
