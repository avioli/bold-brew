package theme

import (
	"os"

	"github.com/gdamore/tcell/v2"
)

// NoColor reports whether colour output is disabled through the NO_COLOR
// environment variable (https://no-color.org/): set and not empty.
func NoColor() bool {
	return os.Getenv("NO_COLOR") != ""
}

// NoColorScreen wraps a tcell.Screen and drops all colours while keeping
// attributes (bold, underline, reverse, ...). tcell does not honour NO_COLOR
// itself, and wrapping the screen covers every widget, tview's included.
type NoColorScreen struct {
	tcell.Screen
}

func stripColor(style tcell.Style) tcell.Style {
	return style.Foreground(tcell.ColorDefault).Background(tcell.ColorDefault)
}

func (s NoColorScreen) SetContent(x, y int, primary rune, combining []rune, style tcell.Style) {
	s.Screen.SetContent(x, y, primary, combining, stripColor(style))
}

func (s NoColorScreen) SetCell(x, y int, style tcell.Style, ch ...rune) {
	s.Screen.SetCell(x, y, stripColor(style), ch...)
}

func (s NoColorScreen) SetStyle(style tcell.Style) {
	s.Screen.SetStyle(stripColor(style))
}

func (s NoColorScreen) Fill(r rune, style tcell.Style) {
	s.Screen.Fill(r, stripColor(style))
}
