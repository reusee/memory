package main

import (
	"log"
	"os"
	"sync"
	"time"

	pyqt "github.com/reusee/go-pyqt5"
)

func ui_qt(connects []*Connect, mem *Memory) {
	// ui
	qt, err := pyqt.New(`
from PyQt5.QtWidgets import QWidget, QLabel, QVBoxLayout, QSizePolicy
from PyQt5.QtCore import Qt
class Win(QWidget):
	def __init__(self, **kwds):
		super().__init__(**kwds)
	def keyPressEvent(self, ev):
		Emit("key", ev.key())
win = Win(styleSheet = "background-color: black;")
hint = QLabel(alignment = Qt.AlignHCenter, styleSheet = "color: white; font-size: 16px;")
hint.setSizePolicy(QSizePolicy.Expanding, QSizePolicy.Minimum)
text = QLabel(alignment = Qt.AlignHCenter, styleSheet = "color: #0099CC; font-size: 64px;")
layout = QVBoxLayout()
layout.addStretch()
layout.addWidget(hint)
layout.addWidget(text)
layout.addStretch()
win.setLayout(layout)
win.showMaximized()
Connect("set-hint", lambda s: hint.setText(s))
Connect("set-text", lambda s: text.setText(s))
		`)
	if err != nil {
		log.Fatal(err)
	}
	defer qt.Close()
	qt.OnClose(func() {
		os.Exit(0)
	})
	keys := make(chan rune)
	qt.Connect("key", func(key float64) {
		select {
		case keys <- rune(key):
		default:
		}
	})
	setHint := func(s string) { qt.Emit("set-hint", s) }
	setText := func(s string) { qt.Emit("set-text", s) }

	setHint("press f to start")
	for {
		key := <-keys
		if key == 'F' {
			break
		}
	}
	setHint("")

	wg := new(sync.WaitGroup)
	save := func() {
		wg.Add(1)
		go func() {
			mem.Save()
			wg.Done()
		}()
	}

	// train
	for _, connect := range connects {
		setHint("")
		setText("")

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
			case 'G':
				lastHistory := connect.Histories[len(connect.Histories)-1]
				connect.Histories = append(connect.Histories, History{Level: lastHistory.Level + 1, Time: time.Now()})
				save()
			case 'T':
				connect.Histories = append(connect.Histories, History{Level: 0, Time: time.Now()})
				save()
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
			case 'G':
				lastHistory := connect.Histories[len(connect.Histories)-1]
				connect.Histories = append(connect.Histories, History{Level: lastHistory.Level + 1, Time: time.Now()})
				save()
			case 'T':
				connect.Histories = append(connect.Histories, History{Level: 0, Time: time.Now()})
				save()
			case ' ':
				goto repeat2
			default:
				goto read_key2
			}

		default:
			panic("impossible")
		}
	}

	wg.Wait()
}
