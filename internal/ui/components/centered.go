package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Centered draws a primitive centred in the screen, at most maxWidth by
// maxHeight and at least margin cells from the edges. The size is worked
// out on every draw, so it follows terminal resizes.
type Centered struct {
	*tview.Box
	child               tview.Primitive
	maxWidth, maxHeight int
	margin              int
}

// NewCentered wraps child.
func NewCentered(child tview.Primitive, maxWidth, maxHeight, margin int) *Centered {
	return &Centered{Box: tview.NewBox(), child: child, maxWidth: maxWidth, maxHeight: maxHeight, margin: margin}
}

func (c *Centered) Draw(screen tcell.Screen) {
	sw, sh := screen.Size()
	w := max(min(c.maxWidth, sw-2*c.margin), 1)
	h := max(min(c.maxHeight, sh-2*c.margin), 1)
	c.child.SetRect((sw-w)/2, (sh-h)/2, w, h)
	c.child.Draw(screen)
}

func (c *Centered) Focus(delegate func(p tview.Primitive)) { delegate(c.child) }
func (c *Centered) HasFocus() bool                         { return c.child.HasFocus() }

func (c *Centered) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return c.child.InputHandler()
}

func (c *Centered) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (bool, tview.Primitive) {
	return c.child.MouseHandler()
}

func (c *Centered) PasteHandler() func(text string, setFocus func(p tview.Primitive)) {
	return c.child.PasteHandler()
}
