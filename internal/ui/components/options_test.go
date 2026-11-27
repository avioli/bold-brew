package components

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func testOptions() *OptionsView {
	return NewOptionsView([]*OptionItem{
		{Section: "View", Label: "Sort", Kind: OptionChoice, Choices: []string{"name", "rank"}, Value: "name"},
		{Section: "View", Label: "Height", Kind: OptionNumber, Value: "5"},
		{Section: "Actions", Label: "Path", Kind: OptionText, Value: "~/B"},
	})
}

func press(v *OptionsView, k tcell.Key, r rune) {
	v.InputHandler()(tcell.NewEventKey(k, r, tcell.ModNone), nil)
}

func TestOptionsViewBrowseAndEdit(t *testing.T) {
	v := testOptions()
	v.SetRect(0, 0, 60, 20)

	press(v, tcell.KeyRune, 'l') // cycle the choice
	if v.items[0].Value != "rank" || !v.items[0].Changed() {
		t.Errorf("choice: %+v", v.items[0])
	}
	press(v, tcell.KeyRune, 'j')
	press(v, tcell.KeyRune, 'j') // j moves, never types
	if v.Selected().Label != "Path" || v.Editing() {
		t.Fatalf("selected %s, editing %v", v.Selected().Label, v.Editing())
	}
	press(v, tcell.KeyEnter, 0)
	press(v, tcell.KeyRune, 'j') // types while editing
	press(v, tcell.KeyEnter, 0)
	if v.items[2].Value != "~/Bj" || v.Editing() {
		t.Errorf("text: %q editing %v", v.items[2].Value, v.Editing())
	}
	press(v, tcell.KeyEnter, 0)
	press(v, tcell.KeyCtrlU, 0)
	press(v, tcell.KeyEsc, 0) // reverts the edit only
	if v.items[2].Value != "~/Bj" || v.Editing() {
		t.Errorf("esc: %q", v.items[2].Value)
	}

	press(v, tcell.KeyRune, 'k') // Height: digits only
	press(v, tcell.KeyEnter, 0)
	press(v, tcell.KeyRune, 'x')
	press(v, tcell.KeyRune, '0')
	press(v, tcell.KeyEnter, 0)
	if v.items[1].Value != "50" {
		t.Errorf("number: %q", v.items[1].Value)
	}

	cancelled, saved, pressed := false, false, -1
	v.SetButtons([]string{"Save", "Cancel"}, func(i int) { pressed = i },
		func() { saved = true }, func() { cancelled = true })
	press(v, tcell.KeyCtrlS, 0)
	press(v, tcell.KeyTab, 0)
	press(v, tcell.KeyRune, 'l')
	press(v, tcell.KeyEnter, 0)
	press(v, tcell.KeyEsc, 0)
	if !saved || pressed != 1 || !cancelled {
		t.Errorf("saved %v pressed %d cancelled %v", saved, pressed, cancelled)
	}
}

func TestOptionsViewRows(t *testing.T) {
	rows := testOptions().rows()
	var got []string
	for _, r := range rows {
		switch {
		case r.header != "":
			got = append(got, "#"+r.header)
		case r.item < 0:
			got = append(got, "-")
		default:
			got = append(got, "i")
		}
	}
	if s := strings.Join(got, " "); s != "#View i i - #Actions i" {
		t.Errorf("rows: %s", s)
	}
}

func TestWrapLines(t *testing.T) {
	got := wrapLines("one two three four five six", 10, 2)
	if len(got) != 2 || got[0] != "one two" || !strings.HasSuffix(got[1], "...") || len(got[1]) > 10 {
		t.Errorf("got %q", got)
	}
	if got := wrapLines("short", 10, 2); len(got) != 1 || got[0] != "short" {
		t.Errorf("got %q", got)
	}
}
