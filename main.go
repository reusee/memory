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

	getPendingConnect := func() []*Connect {
		var connects []*Connect
		for _, connect := range mem.Connects {
			from := mem.Concepts[connect.From]
			if (from.What == WORD || from.What == SENTENCE) && from.Incomplete {
				continue
			}
			if connect.LevelUpTime.Add(LevelTime[connect.Level]).After(time.Now()) {
				continue
			}
			connects = append(connects, connect)
		}
		return connects
	}

	statConnects := func(conns interface{}) {
		c := make(map[int]int)
		switch cs := conns.(type) {
		case []*Connect:
			for _, conn := range cs {
				c[conn.Level]++
			}
		case map[string]*Connect:
			for _, conn := range cs {
				c[conn.Level]++
			}
		}
		for i := len(LevelTime) - 1; i >= 0; i-- {
			if c[i] > 0 {
				fmt.Printf("%d %d\n", i, c[i])
			}
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
					From:        concept.Key(),
					To:          textConcept.Key(),
					LevelUpTime: time.Now(),
				})
				if what == WORD {
					mem.AddConnect(&Connect{
						From:        textConcept.Key(),
						To:          concept.Key(),
						LevelUpTime: time.Now(),
					})
				}
			}
		}

		// show stat
		fmt.Printf("%d concepts, %d connects\n", len(mem.Concepts), len(mem.Connects))
		mem.Save()

	case "train":
		connects := getPendingConnect()
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
			//connect.Dump(mem)
			//continue
			if i > 100 || time.Now().Sub(t0) > time.Minute*10 {
				break
			}
			termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
			p(0, 0, fmt.Sprintf("%v", time.Now().Sub(t0)))
			p(0, 1, fmt.Sprintf("%d", i))
			from := mem.Concepts[connect.From]
			to := mem.Concepts[connect.To]
			switch from.What {
			case AUDIO: // play audio
			repeat:
				termbox.SetCell(width/3, height/2, rune('>'), termbox.ColorDefault, termbox.ColorDefault)
				termbox.SetCell(width/3, height/2+1, rune(fmt.Sprintf("%d", connect.Level)[0]), termbox.ColorDefault, termbox.ColorDefault)
				termbox.Flush()
				from.Play()
				termbox.SetCell(width/3, height/2, rune(' '), termbox.ColorDefault, termbox.ColorDefault)
				termbox.Flush()
				if to.What == WORD {
					termbox.PollEvent()
				}
				p(width/3, height/2+2, "=>"+to.Text)
				ev := termbox.PollEvent()
				switch ev.Key {
				case termbox.KeyEnter:
					connect.Level++
					connect.LevelUpTime = time.Now()
					mem.Save()
				case termbox.KeyArrowLeft:
					connect.Level = 1
					connect.LevelUpTime = time.Now()
					mem.Save()
				case termbox.KeyEsc:
					return
				default:
					goto repeat
				}

			case WORD: // show text
				p(width/3, height/2, "=>"+from.Text)
				termbox.SetCell(width/3, height/2+1, rune(fmt.Sprintf("%d", connect.Level)[0]), termbox.ColorDefault, termbox.ColorDefault)
				termbox.PollEvent()
				to := mem.Concepts[connect.To]
			repeat2:
				termbox.SetCell(width/3, height/2+2, rune('>'), termbox.ColorDefault, termbox.ColorDefault)
				termbox.Flush()
				to.Play()
				termbox.SetCell(width/3, height/2+2, rune(' '), termbox.ColorDefault, termbox.ColorDefault)
				termbox.Flush()
				ev := termbox.PollEvent()
				switch ev.Key {
				case termbox.KeyEnter:
					connect.Level++
					connect.LevelUpTime = time.Now()
					mem.Save()
				case termbox.KeyArrowLeft:
					connect.Level = 1
					connect.LevelUpTime = time.Now()
					mem.Save()
				case termbox.KeyEsc:
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

		cs := getPendingConnect()
		statConnects(cs)
		fmt.Printf("%d pending\n", len(cs))

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
	if connect.Level > 1 {
		return connect.Level*(-1000) - rand.Intn(1000)
	} else if connect.Level == 1 {
		return -(2 ^ 30) - rand.Intn(1024)
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
		return self.l[i].LevelUpTime.Before(self.l[j].LevelUpTime)
	}
	return x < y
}

func (self ConnectSorter) Swap(i, j int) {
	self.l[i], self.l[j] = self.l[j], self.l[i]
}
