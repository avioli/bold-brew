package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Choice is a form field that cycles through a fixed list of values in
// place, drawn as "‹ value ›". Left/h picks the previous value, Right/l and
// Space the next; a click picks the next. Unlike tview.DropDown it opens no
// popup over the rows below. Use it for on/off settings too (options
// "no", "yes").
type Choice struct {
	*tview.Box
	label      string
	labelWidth int
	labelColor tcell.Color
	options    []string
	current    int
	disabled   bool
	finished   func(key tcell.Key)
	changed    func(index int)
}

// NewChoice returns a choice field showing options[current].
func NewChoice(label string, options []string, current int) *Choice {
	return &Choice{
		Box:        tview.NewBox(),
		label:      label,
		labelColor: tview.Styles.SecondaryTextColor,
		options:    options,
		current:    min(max(current, 0), max(len(options)-1, 0)),
	}
}

// NewToggle returns a choice field for an on/off setting, showing yes or no.
func NewToggle(label string, on bool) *Choice {
	current := 0
	if on {
		current = 1
	}
	return NewChoice(label, []string{"no", "yes"}, current)
}

// SetLabel sets the label drawn before the field.
func (c *Choice) SetLabel(label string) *Choice {
	c.label = label
	return c
}

// SetChangedFunc sets what to call when the value changes.
func (c *Choice) SetChangedFunc(f func(index int)) *Choice {
	c.changed = f
	return c
}

// Current returns the index and text of the current value.
func (c *Choice) Current() (int, string) {
	if len(c.options) == 0 {
		return -1, ""
	}
	return c.current, c.options[c.current]
}

// On reports whether a toggle shows yes.
func (c *Choice) On() bool { return c.current == 1 }

// GetLabel implements tview.FormItem.
func (c *Choice) GetLabel() string { return c.label }

// SetFormAttributes implements tview.FormItem.
func (c *Choice) SetFormAttributes(labelWidth int, labelColor, bgColor, _, _ tcell.Color) tview.FormItem {
	c.labelWidth = labelWidth
	c.labelColor = labelColor
	c.SetBackgroundColor(bgColor)
	return c
}

// GetFieldWidth implements tview.FormItem: "‹ " + widest value + " ›".
func (c *Choice) GetFieldWidth() int {
	w := 0
	for _, o := range c.options {
		w = max(w, tview.TaggedStringWidth(tview.Escape(o)))
	}
	return w + 4
}

// GetFieldHeight implements tview.FormItem.
func (c *Choice) GetFieldHeight() int { return 1 }

// SetFinishedFunc implements tview.FormItem.
func (c *Choice) SetFinishedFunc(handler func(key tcell.Key)) tview.FormItem {
	c.finished = handler
	return c
}

// SetDisabled implements tview.FormItem.
func (c *Choice) SetDisabled(disabled bool) tview.FormItem {
	c.disabled = disabled
	return c
}

// move steps the value by delta, wrapping around.
func (c *Choice) move(delta int) {
	if c.disabled || len(c.options) == 0 {
		return
	}
	c.current = (c.current + delta + len(c.options)) % len(c.options)
	if c.changed != nil {
		c.changed(c.current)
	}
}

// Draw draws the label and the field.
func (c *Choice) Draw(screen tcell.Screen) {
	c.DrawForSubclass(screen, c)
	x, y, width, height := c.GetInnerRect()
	if height < 1 || width < 1 {
		return
	}
	put := func(text string, style tcell.Style, maxWidth int) int {
		n := 0
		for _, r := range text {
			if n >= maxWidth {
				break
			}
			screen.SetContent(x+n, y, r, nil, style)
			n++
		}
		return n
	}

	labelWidth := c.labelWidth
	if labelWidth == 0 {
		labelWidth = len([]rune(c.label)) + 1
	}
	put(c.label, tcell.StyleDefault.Foreground(c.labelColor), min(labelWidth, width))
	x, width = x+labelWidth, width-labelWidth
	if width <= 0 {
		return
	}

	_, value := c.Current()
	field := "‹ " + value + " ›"
	for len([]rune(field)) < c.GetFieldWidth() {
		field += " "
	}
	style := tcell.StyleDefault.Reverse(true)
	if c.HasFocus() {
		style = style.Bold(true)
	}
	if c.disabled {
		style = style.Dim(true)
	}
	put(field, style, width)
}

// InputHandler cycles the value, and hands Tab, Enter and Esc back to the
// form.
func (c *Choice) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return c.WrapInputHandler(func(event *tcell.EventKey, _ func(p tview.Primitive)) {
		switch event.Key() {
		case tcell.KeyLeft:
			c.move(-1)
		case tcell.KeyRight:
			c.move(1)
		case tcell.KeyRune:
			switch event.Rune() {
			case 'h':
				c.move(-1)
			case 'l', ' ':
				c.move(1)
			}
		case tcell.KeyTab, tcell.KeyBacktab, tcell.KeyEnter, tcell.KeyEscape:
			if c.finished != nil {
				c.finished(event.Key())
			}
		}
	})
}

// MouseHandler focuses the field on click, and picks the next value when
// the field itself is clicked.
func (c *Choice) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (bool, tview.Primitive) {
	return c.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (bool, tview.Primitive) {
		x, y := event.Position()
		if !c.InRect(x, y) {
			return false, nil
		}
		if action == tview.MouseLeftClick {
			setFocus(c)
			bx, _, _, _ := c.GetInnerRect()
			if x >= bx+c.labelWidth {
				c.move(1)
			}
			return true, nil
		}
		return false, nil
	})
}
