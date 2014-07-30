package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/reusee/lgtk"
)

func ui_gtk(connects []*Connect, mem *Memory) {
	// ui
	keys := make(chan rune)
	g, err := lgtk.New(`
Gdk = lgi.Gdk

css = Gtk.CssProvider.get_default()
print(css:load_from_data([[
GtkWindow {
	background-color: black;
	color: white;
}
#hint {
	font-size: 16px;
}
#text {
	font-size: 48px;
	color: #0099CC;
}
]]))
Gtk.StyleContext.add_provider_for_screen(Gdk.Screen.get_default(), css, 999)

win = Gtk.Window{
	Gtk.Grid{
		orientation = 'VERTICAL',
		Gtk.Label{
			expand = true,
		},
		Gtk.Label{
			id = 'hint',
			name = 'hint',
		},
		Gtk.Label{
			id = 'text',
			name = 'text',
		},
		Gtk.Label{
			expand = true,
		},
	},
}
function win:on_key_press_event(ev)
	Key(ev.keyval)
	return true
end
function win.on_destroy()
	Exit(0)
end
win:show_all()
	`,
		"Key", func(val rune) {
			select {
			case keys <- val:
			default:
			}
		},
	)
	if err != nil {
		log.Fatal(err)
	}
	setHint := func(s string) {
		g.Exec(func() {
			g.Eval(fmt.Sprintf(`win.child.hint:set_label("%s")`, s))
		})
	}
	setText := func(s string) {
		g.Exec(func() {
			g.Eval(fmt.Sprintf(`win.child.text:set_label("%s")`, s))
		})
	}

	setHint("press f to start")
	for {
		key := <-keys
		if key == 'f' {
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
			case 'g':
				lastHistory := connect.Histories[len(connect.Histories)-1]
				connect.Histories = append(connect.Histories, History{Level: lastHistory.Level + 1, Time: time.Now()})
				save()
			case 't':
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
			case 'g':
				lastHistory := connect.Histories[len(connect.Histories)-1]
				connect.Histories = append(connect.Histories, History{Level: lastHistory.Level + 1, Time: time.Now()})
				save()
			case 't':
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
