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
	time.Minute * 10,
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
				})
				if what == WORD {
					mem.AddConnect(&Connect{
						From: textConcept.Key(),
						To:   concept.Key(),
					})
				}
			}
		}

		// show stat
		fmt.Printf("%d concepts, %d connects\n", len(mem.Concepts), len(mem.Connects))
		mem.Save()

	case "train":
		// get connects
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
		// sort
		sort.Sort(ConnectSorter{connects, mem})
		fmt.Printf("%d to train\n", len(connects))
		// train
		err := termbox.Init()
		if err != nil {
			log.Fatal(err)
		}
		defer termbox.Close()
		width, height := termbox.Size()
		t0 := time.Now()
		for i, connect := range connects {
			if i > 100 || time.Now().Sub(t0) > time.Minute * 10 {
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
		c := make(map[int]int)
		for _, conn := range mem.Connects {
			c[conn.Level]++
		}
		for level, count := range c {
			fmt.Printf("%d %d %v\n", level, count, LevelTime[level])
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
func (self ConnectSorter) Less(i, j int) bool {
	left := self.l[i]
	leftFrom := self.m.Concepts[left.From]
	leftTo := self.m.Concepts[left.To]
	right := self.l[j]
	rightFrom := self.m.Concepts[right.From]
	rightTo := self.m.Concepts[right.To]
	ltext, laudio := leftFrom, leftTo
	if leftFrom.What == AUDIO {
		ltext, laudio = leftTo, leftFrom
	}
	rtext, raudio := rightFrom, rightTo
	if rightFrom.What == AUDIO {
		rtext, raudio = rightTo, rightFrom
	}
	if left.Level == right.Level {
		if left.Level == 0 {
			if leftFrom.What == AUDIO && rightFrom.What != AUDIO {
				return true
			}
			if ltext.What == rtext.What {
				return laudio.File < raudio.File
			} else {
				if ltext.What == WORD && rtext.What == SENTENCE {
					return true
				} else {
					return false
				}
			}
		} else {
			return rand.Intn(2) == 0
		}
	} else {
		return left.Level > right.Level
	}
}
func (self ConnectSorter) Swap(i, j int) {
	self.l[i], self.l[j] = self.l[j], self.l[i]
}
