package components

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestChoiceCycles(t *testing.T) {
	c := NewChoice("Sort", []string{"name", "downloads", "rank"}, 0)
	var finished tcell.Key = -1
	c.SetFinishedFunc(func(k tcell.Key) { finished = k })
	handle := c.InputHandler()
	press := func(k tcell.Key, r rune) { handle(tcell.NewEventKey(k, r, tcell.ModNone), nil) }

	press(tcell.KeyLeft, 0) // wraps to the last value
	if _, v := c.Current(); v != "rank" {
		t.Errorf("left: %s", v)
	}
	press(tcell.KeyRune, ' ')
	press(tcell.KeyRight, 0)
	if _, v := c.Current(); v != "downloads" {
		t.Errorf("space, right: %s", v)
	}
	press(tcell.KeyEnter, 0)
	if finished != tcell.KeyEnter {
		t.Errorf("enter not handed to the form: %v", finished)
	}
	if c.GetFieldWidth() != len("downloads")+4 {
		t.Errorf("width %d", c.GetFieldWidth())
	}

	tg := NewToggle("Borders", true)
	if !tg.On() {
		t.Error("toggle should start on")
	}
	tg.InputHandler()(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone), nil)
	if tg.On() {
		t.Error("space should switch it off")
	}
	if _, v := tg.Current(); v != "no" {
		t.Errorf("toggle shows %s", v)
	}
}
