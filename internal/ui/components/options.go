package components

import (
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// OptionKind is how an option's value is edited.
type OptionKind int

const (
	OptionChoice OptionKind = iota // cycles through Choices
	OptionNumber                   // digits, edited after Enter
	OptionText                     // free text, edited after Enter
)

// OptionItem is one setting in an OptionsView.
type OptionItem struct {
	Section  string // a header is drawn where the section changes
	Label    string
	Kind     OptionKind
	Choices  []string // OptionChoice values
	Value    string
	Describe func(value string) string // optional, at most about two lines

	initial string
}

// Changed reports whether the value differs from when the view was built.
func (o *OptionItem) Changed() bool { return o.Value != o.initial }

// Option view layout: the description, a blank row and the buttons sit
// under the list.
const (
	optionDescLines   = 2
	optionFooterLines = optionDescLines + 2
)

// OptionsView is a scrolling list of settings grouped in sections, with a
// description of the selected one and a row of buttons.
//
// Browsing: Up/Down, j/k, Ctrl+N/P move; g/G, Home/End, PgUp/PgDn and
// Ctrl+U/D jump; Left/Right, h/l and Space change a choice (Enter too);
// Enter edits a number or text, which Enter keeps and Esc reverts. Tab
// moves between the list and the buttons. Ctrl+S runs the first button,
// Esc (when not editing) calls the cancel function.
type OptionsView struct {
	*tview.Box
	items    []*OptionItem
	selected int
	offset   int // first visible list row

	editing bool
	edit    []rune
	cursor  int

	buttons      []string
	onButton     func(index int)
	onCancel     func()
	onSave       func()
	inButtons    bool
	activeButton int

	changedStyle tcell.Style
	headerStyle  tcell.Style
}

// NewOptionsView builds a view over items; their current values become the
// baseline Changed compares with.
func NewOptionsView(items []*OptionItem) *OptionsView {
	for _, it := range items {
		it.initial = it.Value
	}
	v := &OptionsView{
		Box:          tview.NewBox(),
		items:        items,
		changedStyle: tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true),
		headerStyle:  tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true), // like the details panel headings
	}
	v.SetBorder(true).SetBorderPadding(0, 0, 1, 1)
	return v
}

// SetButtons sets the buttons and what pressing one does. save runs on
// Ctrl+S, cancel on Esc.
func (v *OptionsView) SetButtons(labels []string, pressed func(index int), save, cancel func()) *OptionsView {
	v.buttons, v.onButton, v.onSave, v.onCancel = labels, pressed, save, cancel
	return v
}

// Items returns the options.
func (v *OptionsView) Items() []*OptionItem { return v.items }

// Selected returns the selected option.
func (v *OptionsView) Selected() *OptionItem {
	if len(v.items) == 0 {
		return nil
	}
	return v.items[v.selected]
}

// Editing reports whether a value is being edited.
func (v *OptionsView) Editing() bool { return v.editing }

// CommitEdit keeps the value being edited, if any.
func (v *OptionsView) CommitEdit() {
	if !v.editing {
		return
	}
	v.items[v.selected].Value = strings.TrimSpace(string(v.edit))
	v.editing = false
}

// listRow is one row of the list: a section header, a blank or an item.
type listRow struct {
	header string
	item   int // -1 for headers and blanks
}

func (v *OptionsView) rows() []listRow {
	var rows []listRow
	section := "\x00"
	for i, it := range v.items {
		if it.Section != section {
			if i > 0 {
				rows = append(rows, listRow{item: -1})
			}
			rows = append(rows, listRow{header: it.Section, item: -1})
			section = it.Section
		}
		rows = append(rows, listRow{item: i})
	}
	return rows
}

// rowOf is the list row of item i.
func (v *OptionsView) rowOf(rows []listRow, i int) int {
	for r, row := range rows {
		if row.item == i {
			return r
		}
	}
	return 0
}

func (v *OptionsView) listHeight() int {
	_, _, _, h := v.GetInnerRect()
	return max(h-optionFooterLines, 1)
}

// scrollToSelected keeps the selected item (and its section header, when
// it is the first of its section) in view.
func (v *OptionsView) scrollToSelected(rows []listRow) {
	h := v.listHeight()
	r := v.rowOf(rows, v.selected)
	top := r
	for top > 0 && rows[top-1].item < 0 && rows[top-1].header != "" {
		top--
	}
	if top < v.offset {
		v.offset = top
	}
	if r >= v.offset+h {
		v.offset = r - h + 1
	}
	v.offset = min(max(v.offset, 0), max(len(rows)-h, 0))
}

func (v *OptionsView) labelWidth() int {
	w := 0
	for _, it := range v.items {
		w = max(w, len([]rune(it.Label)))
	}
	return w + 4 // marker and gap
}

// put draws text at x, y, clipped to width, and returns the columns used.
func put(screen tcell.Screen, x, y, width int, text string, style tcell.Style) int {
	n := 0
	for _, r := range text {
		if n >= width {
			break
		}
		screen.SetContent(x+n, y, r, nil, style)
		n++
	}
	return n
}

// displayValue is how an item's value is shown when not being edited.
func displayValue(it *OptionItem) string {
	if it.Kind == OptionChoice {
		return "‹ " + it.Value + " ›"
	}
	if it.Value == "" {
		return "(empty)"
	}
	return it.Value
}

// Draw draws the list, the description and the buttons.
func (v *OptionsView) Draw(screen tcell.Screen) {
	v.DrawForSubclass(screen, v)
	x, y, width, height := v.GetInnerRect()
	if width <= 0 || height <= 0 {
		return
	}
	rows := v.rows()
	v.scrollToSelected(rows)
	listH := v.listHeight()
	labelW := min(v.labelWidth(), width/2+8)
	focused := v.HasFocus()

	for line := 0; line < listH && v.offset+line < len(rows); line++ {
		row := rows[v.offset+line]
		ry := y + line
		if row.item < 0 {
			if row.header != "" {
				put(screen, x, ry, width, row.header, v.headerStyle)
			}
			continue
		}
		it := v.items[row.item]
		sel := row.item == v.selected && !v.inButtons
		labelStyle := tcell.StyleDefault
		marker := "  "
		if sel {
			marker = "▸ "
			if focused {
				labelStyle = labelStyle.Reverse(true)
			}
		}
		put(screen, x, ry, width, marker, tcell.StyleDefault)
		put(screen, x+2, ry, labelW-2, it.Label, labelStyle)

		vx, vw := x+labelW, width-labelW
		if vw <= 0 {
			continue
		}
		if sel && v.editing {
			v.drawEdit(screen, vx, ry, vw)
			continue
		}
		style := tcell.StyleDefault
		if it.Changed() {
			style = v.changedStyle
		}
		if sel && focused {
			style = style.Underline(true)
		}
		put(screen, vx, ry, vw, displayValue(it), style)
	}

	// Scroll hints on the right edge.
	if v.offset > 0 {
		put(screen, x+width-1, y, 1, "▲", tcell.StyleDefault.Dim(true))
	}
	if v.offset+listH < len(rows) {
		put(screen, x+width-1, y+listH-1, 1, "▼", tcell.StyleDefault.Dim(true))
	}

	// Description.
	descY := y + height - optionFooterLines
	if it := v.Selected(); it != nil && it.Describe != nil && !v.inButtons {
		for i, line := range wrapLines(it.Describe(it.Value), width, optionDescLines) {
			put(screen, x, descY+i, width, line, tcell.StyleDefault.Dim(true))
		}
	}

	// Buttons, centred.
	v.drawButtons(screen, x, y+height-1, width)
}

func (v *OptionsView) drawEdit(screen tcell.Screen, x, y, width int) {
	// Scroll the edit text so the cursor stays visible.
	start := max(v.cursor-width+1, 0)
	style := tcell.StyleDefault.Reverse(true)
	for i := 0; i < width; i++ {
		r := ' '
		if start+i < len(v.edit) {
			r = v.edit[start+i]
		}
		s := style
		if start+i == v.cursor {
			s = tcell.StyleDefault.Bold(true).Underline(true)
		}
		if i >= len(v.edit)-start+1 {
			break
		}
		screen.SetContent(x+i, y, r, nil, s)
	}
}

func (v *OptionsView) buttonLayout(width int) (start int, labels []string) {
	total := 0
	for _, b := range v.buttons {
		labels = append(labels, "  "+b+"  ")
		total += len([]rune(b)) + 4 + 2
	}
	return max((width-total+2)/2, 0), labels
}

func (v *OptionsView) drawButtons(screen tcell.Screen, x, y, width int) {
	start, labels := v.buttonLayout(width)
	bx := x + start
	for i, label := range labels {
		style := tcell.StyleDefault.Reverse(true)
		if v.inButtons && i == v.activeButton && v.HasFocus() {
			style = tcell.StyleDefault.Bold(true).Underline(true)
		}
		bx += put(screen, bx, y, x+width-bx, label, style) + 2
	}
}

// wrapLines word-wraps text to width, keeping at most n lines (the last one
// ends with "..." when text was cut).
func wrapLines(text string, width, n int) []string {
	if width <= 0 {
		return nil
	}
	var lines []string
	var line []rune
	cut := false
	for _, word := range strings.Fields(text) {
		w := []rune(word)
		switch {
		case len(line) == 0:
			line = w
		case len(line)+1+len(w) <= width:
			line = append(append(line, ' '), w...)
		default:
			lines = append(lines, string(line))
			line = w
		}
		if len(lines) == n {
			cut = true
			break
		}
	}
	if len(line) > 0 && len(lines) < n {
		lines = append(lines, string(line))
	}
	if cut && len(lines) > 0 {
		last := []rune(lines[n-1])
		if len(last)+3 > width {
			last = last[:max(width-3, 0)]
		}
		lines[n-1] = string(last) + "..."
	}
	for i, l := range lines {
		if r := []rune(l); len(r) > width {
			lines[i] = string(r[:width])
		}
	}
	return lines
}

// move selects the item delta places away, clamped.
func (v *OptionsView) move(delta int) {
	if len(v.items) == 0 {
		return
	}
	v.selected = min(max(v.selected+delta, 0), len(v.items)-1)
}

// cycle changes a choice by delta, wrapping around.
func (v *OptionsView) cycle(delta int) {
	it := v.Selected()
	if it == nil || it.Kind != OptionChoice || len(it.Choices) == 0 {
		return
	}
	i := 0
	for j, c := range it.Choices {
		if c == it.Value {
			i = j
		}
	}
	it.Value = it.Choices[(i+delta+len(it.Choices))%len(it.Choices)]
}

func (v *OptionsView) startEdit() {
	it := v.Selected()
	if it == nil || it.Kind == OptionChoice {
		return
	}
	v.editing = true
	v.edit = []rune(it.Value)
	v.cursor = len(v.edit)
}

// insert adds text at the cursor (digits only for numbers).
func (v *OptionsView) insert(text string) {
	number := v.Selected().Kind == OptionNumber
	for _, r := range text {
		if r == '\n' || r == '\r' || !unicode.IsPrint(r) || (number && !unicode.IsDigit(r)) {
			continue
		}
		v.edit = append(v.edit[:v.cursor], append([]rune{r}, v.edit[v.cursor:]...)...)
		v.cursor++
	}
}

func (v *OptionsView) pageSize() int { return max(v.listHeight()/2, 1) }

// InputHandler handles browsing, editing and the buttons.
func (v *OptionsView) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return v.WrapInputHandler(func(event *tcell.EventKey, _ func(p tview.Primitive)) {
		if event.Key() == tcell.KeyCtrlS {
			v.CommitEdit()
			if v.onSave != nil {
				v.onSave()
			}
			return
		}
		switch {
		case v.editing:
			v.handleEdit(event)
		case v.inButtons:
			v.handleButtons(event)
		default:
			v.handleBrowse(event)
		}
	})
}

func (v *OptionsView) handleEdit(event *tcell.EventKey) {
	switch event.Key() {
	case tcell.KeyEnter, tcell.KeyTab:
		v.CommitEdit()
	case tcell.KeyEsc:
		v.editing = false
	case tcell.KeyLeft:
		v.cursor = max(v.cursor-1, 0)
	case tcell.KeyRight:
		v.cursor = min(v.cursor+1, len(v.edit))
	case tcell.KeyHome, tcell.KeyCtrlA:
		v.cursor = 0
	case tcell.KeyEnd, tcell.KeyCtrlE:
		v.cursor = len(v.edit)
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if v.cursor > 0 {
			v.edit = append(v.edit[:v.cursor-1], v.edit[v.cursor:]...)
			v.cursor--
		}
	case tcell.KeyDelete, tcell.KeyCtrlD:
		if v.cursor < len(v.edit) {
			v.edit = append(v.edit[:v.cursor], v.edit[v.cursor+1:]...)
		}
	case tcell.KeyCtrlU:
		v.edit, v.cursor = v.edit[:0], 0
	case tcell.KeyRune:
		v.insert(string(event.Rune()))
	}
}

func (v *OptionsView) handleButtons(event *tcell.EventKey) {
	switch event.Key() {
	case tcell.KeyTab, tcell.KeyBacktab, tcell.KeyUp, tcell.KeyCtrlP:
		v.inButtons = false
	case tcell.KeyLeft:
		v.activeButton = max(v.activeButton-1, 0)
	case tcell.KeyRight:
		v.activeButton = min(v.activeButton+1, len(v.buttons)-1)
	case tcell.KeyEnter:
		v.press(v.activeButton)
	case tcell.KeyEsc:
		v.cancel()
	case tcell.KeyRune:
		switch event.Rune() {
		case 'h':
			v.activeButton = max(v.activeButton-1, 0)
		case 'l':
			v.activeButton = min(v.activeButton+1, len(v.buttons)-1)
		case 'k':
			v.inButtons = false
		case ' ':
			v.press(v.activeButton)
		}
	}
}

func (v *OptionsView) handleBrowse(event *tcell.EventKey) {
	switch event.Key() {
	case tcell.KeyUp, tcell.KeyCtrlP:
		v.move(-1)
	case tcell.KeyDown, tcell.KeyCtrlN:
		v.move(1)
	case tcell.KeyHome:
		v.selected = 0
	case tcell.KeyEnd:
		v.selected = len(v.items) - 1
	case tcell.KeyPgUp, tcell.KeyCtrlU, tcell.KeyCtrlB:
		v.move(-v.pageSize())
	case tcell.KeyPgDn, tcell.KeyCtrlD, tcell.KeyCtrlF, tcell.KeyCtrlV:
		v.move(v.pageSize())
	case tcell.KeyLeft:
		v.cycle(-1)
	case tcell.KeyRight:
		v.cycle(1)
	case tcell.KeyEnter:
		if it := v.Selected(); it != nil && it.Kind == OptionChoice {
			v.cycle(1)
		} else {
			v.startEdit()
		}
	case tcell.KeyTab, tcell.KeyBacktab:
		if len(v.buttons) > 0 {
			v.inButtons = true
		}
	case tcell.KeyEsc:
		v.cancel()
	case tcell.KeyRune:
		switch event.Rune() {
		case 'k':
			v.move(-1)
		case 'j':
			v.move(1)
		case 'g':
			v.selected = 0
		case 'G':
			v.selected = len(v.items) - 1
		case 'h':
			v.cycle(-1)
		case 'l', ' ':
			if it := v.Selected(); it != nil && it.Kind == OptionChoice {
				v.cycle(1)
			} else if event.Rune() == ' ' {
				v.startEdit()
			}
		}
	}
}

func (v *OptionsView) press(i int) {
	if v.onButton != nil && i >= 0 && i < len(v.buttons) {
		v.onButton(i)
	}
}

func (v *OptionsView) cancel() {
	if v.onCancel != nil {
		v.onCancel()
	}
}

// PasteHandler inserts pasted text while editing.
func (v *OptionsView) PasteHandler() func(text string, setFocus func(p tview.Primitive)) {
	return v.WrapPasteHandler(func(text string, _ func(p tview.Primitive)) {
		if v.editing {
			v.insert(text)
		}
	})
}

// MouseHandler selects a row on click (a click on a choice's value changes
// it), presses buttons and scrolls with the wheel.
func (v *OptionsView) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (bool, tview.Primitive) {
	return v.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (bool, tview.Primitive) {
		mx, my := event.Position()
		if !v.InRect(mx, my) {
			return false, nil
		}
		switch action {
		case tview.MouseScrollUp:
			v.move(-1)
			return true, nil
		case tview.MouseScrollDown:
			v.move(1)
			return true, nil
		case tview.MouseLeftClick:
		default:
			return false, nil
		}
		setFocus(v)
		x, y, width, height := v.GetInnerRect()
		if my == y+height-1 { // buttons
			start, labels := v.buttonLayout(width)
			bx := x + start
			for i, label := range labels {
				w := len([]rune(label))
				if mx >= bx && mx < bx+w {
					v.CommitEdit()
					v.press(i)
					return true, nil
				}
				bx += w + 2
			}
			return true, nil
		}
		line := my - y
		rows := v.rows()
		if line < 0 || line >= v.listHeight() || v.offset+line >= len(rows) {
			return true, nil
		}
		row := rows[v.offset+line]
		if row.item < 0 {
			return true, nil
		}
		v.CommitEdit()
		v.inButtons = false
		if v.selected == row.item && mx >= x+min(v.labelWidth(), width/2+8) {
			if v.items[row.item].Kind == OptionChoice {
				v.cycle(1)
			} else {
				v.startEdit()
			}
		}
		v.selected = row.item
		return true, nil
	})
}
