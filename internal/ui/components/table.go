package components

import (
	"strings"

	"bbrew/internal/ui/theme"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Column indices of the package table.
const (
	ColType = iota
	ColName
	ColVersion
	ColDescription
	ColDownloads
	numColumns
)

// Column sizing rules. See fitColumns.
const (
	maxNameWidth     = 32 // Name never grows beyond this (ellipsis)
	clipNameWidth    = 20 // Name may be clipped down to this
	maxVersionWidth  = 15 // Version never grows beyond this
	clipVersionWidth = 10 // Version may be clipped down to this
	minDescWidth     = 11 // len("Description"); Description prefers at least this
	lastDescWidth    = 5  // ...but shows down to this rather than hiding
	readDescWidth    = 20 // Description keeps this much next to Downloads

	// Downloads only shows at its natural position (offset 0) from this
	// table width. That is a 147 column screen with the details panel, or
	// 106 without it (borders, padding and the 37 column panel included).
	downloadsComfortWidth = 102
	sortIndicator         = " ▼"
)

// Row is the display data for one package row. Text fields are plain text
// (not tview-tagged); they are escaped when rendered.
type Row struct {
	Type         string // single letter: F, C, P, M
	Name         string
	Version      string
	Description  string
	Downloads    string
	NameColor    tcell.Color // tcell.ColorDefault for none
	VersionColor tcell.Color
	NameAttr     tcell.AttrMask // e.g. bold, for NO_COLOR
	VersionAttr  tcell.AttrMask
}

// fitInput holds the natural content widths a column layout is computed from.
type fitInput struct {
	name, version, downloads int // widest cell over all rows
	sortColumn               int
}

// fitResult is a computed column layout.
type fitResult struct {
	columns   []int // visible columns, in order
	widths    [numColumns]int
	headers   [numColumns]string
	compact   bool // Type shown as "F" rather than "[F]"
	offset    int  // clamped scroll offset
	maxOffset int
}

// scrollable columns, in order; Type and Name are always visible (frozen).
var scrollable = []int{ColVersion, ColDescription, ColDownloads}

func headerText(col int, label string, sortColumn int) string {
	if col == sortColumn {
		return label + sortIndicator
	}
	return label
}

// fitColumns lays out the columns in the given width.
//
//  1. Every column gets its minimum: Type 1 ("T"/"F"), Name up to 20, Version
//     up to 10, Description 20 next to Downloads and otherwise 11 (5 as a
//     last resort), Downloads its digits (header "Dl").
//  2. Type and Name are always shown. Version, Description and Downloads form
//     a window, starting at offset, of as many columns as fit at their
//     minimum. At offset 0, Downloads needs a table at least
//     downloadsComfortWidth wide; scrolling right can still reveal it.
//  3. Leftover space goes, in order, to Version (up to its content), Type
//     ("Type"/"[F]"), Downloads (header "Downloads"), Name (up to 32) and the
//     rest to Description.
//
// tview puts a one column separator after every column.
func fitColumns(width int, in fitInput, offset int) fitResult {
	var r fitResult
	s := in.sortColumn
	w := tview.TaggedStringWidth

	nameHeader := headerText(ColName, "Name", s)
	versionHeader := headerText(ColVersion, "Version", s)
	descHeader := headerText(ColDescription, "Description", s)
	dlFull := headerText(ColDownloads, "Downloads", s)
	dlShort := headerText(ColDownloads, "Dl", s)

	nat := [numColumns]int{
		ColType:        4, // "Type", "[F]"
		ColName:        max(min(in.name, maxNameWidth), w(nameHeader)),
		ColVersion:     max(min(in.version, maxVersionWidth), w(versionHeader)),
		ColDescription: 0, // flexible
		ColDownloads:   max(in.downloads, w(dlFull)),
	}
	minW := [numColumns]int{
		ColType:        1,
		ColName:        min(nat[ColName], clipNameWidth),
		ColVersion:     min(nat[ColVersion], clipVersionWidth),
		ColDescription: minDescWidth,
		ColDownloads:   max(in.downloads, w(dlShort)),
	}

	avail := width - (minW[ColType] + 1) - (minW[ColName] + 1)

	// window returns the scrollable columns that fit from offset o, with
	// their widths, and the space they use. Description keeps 20 columns
	// when Downloads shows next to it; without Downloads it may shrink to 11,
	// or to 5 as a last resort.
	window := func(o int) (cols, widths []int, used int) {
		add := func(col, cw int) {
			cols, widths, used = append(cols, col), append(widths, cw), used+cw+1
		}
		dlAllowed := o > 0 || width >= downloadsComfortWidth
		for _, col := range scrollable[o:] {
			left := avail - used
			switch col {
			case ColDescription:
				if dlAllowed && readDescWidth+1+minW[ColDownloads]+1 <= left {
					add(ColDescription, readDescWidth)
					add(ColDownloads, minW[ColDownloads])
					return cols, widths, used
				}
				cw := min(minW[col], left-1)
				if cw < lastDescWidth {
					return cols, widths, used
				}
				add(col, cw)
				return cols, widths, used // Downloads did not fit beside it
			case ColDownloads:
				if !dlAllowed || minW[col]+1 > left {
					return cols, widths, used
				}
				add(col, minW[col])
			default:
				if minW[col]+1 > left {
					return cols, widths, used
				}
				add(col, minW[col])
			}
		}
		return cols, widths, used
	}

	// The furthest offset is the first one whose window reaches the last
	// column; scrolling further would only hide columns on the left.
	last := scrollable[len(scrollable)-1]
	r.maxOffset = len(scrollable) - 1
	for o := range scrollable {
		if cols, _, _ := window(o); len(cols) > 0 && cols[len(cols)-1] == last {
			r.maxOffset = o
			break
		}
	}
	r.offset = min(max(offset, 0), r.maxOffset)

	cols, widths, used := window(r.offset)
	r.widths[ColType], r.widths[ColName] = minW[ColType], minW[ColName]
	shown := map[int]bool{}
	for i, col := range cols {
		r.widths[col] = widths[i]
		shown[col] = true
	}

	// Distribute leftover space.
	extra := max(avail-used, 0)
	grow := func(col, target int) {
		if add := min(max(target-r.widths[col], 0), extra); add > 0 {
			r.widths[col] += add
			extra -= add
		}
	}
	if shown[ColVersion] {
		grow(ColVersion, nat[ColVersion])
	}
	if extra >= nat[ColType]-r.widths[ColType] {
		grow(ColType, nat[ColType])
	}
	if shown[ColDownloads] && extra >= nat[ColDownloads]-r.widths[ColDownloads] {
		grow(ColDownloads, nat[ColDownloads])
	}
	grow(ColName, nat[ColName])
	if shown[ColDescription] {
		grow(ColDescription, r.widths[ColDescription]+extra)
	}

	r.compact = r.widths[ColType] < nat[ColType]
	r.headers = [numColumns]string{
		ColType:        "Type",
		ColName:        nameHeader,
		ColVersion:     versionHeader,
		ColDescription: descHeader,
		ColDownloads:   dlFull,
	}
	if r.compact {
		r.headers[ColType] = "T"
	}
	if r.widths[ColDownloads] < w(dlFull) {
		r.headers[ColDownloads] = dlShort
	}

	r.columns = append([]int{ColType, ColName}, cols...)
	return r
}

// Table renders package rows and fits its columns to the available width.
// Type and Name are frozen; Version, Description and Downloads scroll
// horizontally (h/l, Left/Right) when they do not all fit.
type Table struct {
	view  *fitTable
	theme *theme.Theme

	rows []Row
	in   fitInput

	offset int       // requested horizontal scroll offset
	layout fitResult // last computed layout

	lastWidth int
	dirty     bool
}

// fitTable wraps tview.Table to re-fit columns whenever its width changes.
type fitTable struct {
	*tview.Table
	owner *Table
}

func (f *fitTable) Draw(screen tcell.Screen) {
	x, y, width, _ := f.GetInnerRect()
	if f.owner.dirty || width != f.owner.lastWidth {
		f.owner.render(width)
	}
	f.Table.Draw(screen)
	f.owner.drawScrollHints(screen, x, y, width)
}

func NewTable(theme *theme.Theme) *Table {
	table := &Table{
		theme: theme,
		in:    fitInput{sortColumn: -1},
	}
	table.view = &fitTable{Table: tview.NewTable(), owner: table}

	view := table.view.Table
	view.SetBorders(false)
	view.SetSelectable(true, false)
	view.SetFixed(1, 0)

	// Use reverse video for selection to ensure visibility on any terminal
	// theme. When the table is not the active panel, dim the selection.
	focused := tcell.StyleDefault.Reverse(true)
	blurred := tcell.StyleDefault.Underline(true).Bold(true)
	view.SetSelectedStyle(focused)
	view.SetFocusFunc(func() { view.SetSelectedStyle(focused) })
	view.SetBlurFunc(func() { view.SetSelectedStyle(blurred) })

	// h/l and Left/Right scroll the non-frozen columns.
	view.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyLeft, event.Key() == tcell.KeyRune && event.Rune() == 'h':
			table.ScrollColumns(-1)
			return nil
		case event.Key() == tcell.KeyRight, event.Key() == tcell.KeyRune && event.Rune() == 'l':
			table.ScrollColumns(1)
			return nil
		}
		return event
	})

	return table
}

func (t *Table) SetSelectionHandler(handler func(row, column int)) {
	t.view.SetSelectionChangedFunc(handler)
}

// View returns the underlying tview table (for focus, selection, etc).
func (t *Table) View() *tview.Table {
	return t.view.Table
}

// Primitive returns the primitive to place in a layout. It must be used
// instead of View() there, so that columns are fitted on draw.
func (t *Table) Primitive() tview.Primitive {
	return t.view
}

func (t *Table) Clear() {
	t.rows = nil
	t.view.Clear()
}

// ScrollColumns moves the window of non-frozen columns by dir (+1 right,
// -1 left). It does nothing when all columns fit.
func (t *Table) ScrollColumns(dir int) {
	offset := min(max(t.layout.offset+dir, 0), t.layout.maxOffset)
	if offset != t.layout.offset {
		t.offset = offset
		t.render(t.lastWidth)
	}
}

// SetRows replaces the table content. sortColumn is the column to mark as
// the sort key, or -1.
func (t *Table) SetRows(rows []Row, sortColumn int) {
	t.rows = rows
	t.in = fitInput{sortColumn: sortColumn}
	for _, r := range rows {
		t.in.name = max(t.in.name, tview.TaggedStringWidth(tview.Escape(r.Name)))
		t.in.version = max(t.in.version, tview.TaggedStringWidth(tview.Escape(r.Version)))
		t.in.downloads = max(t.in.downloads, len(r.Downloads))
	}

	// Render right away when the width is known, so selection works on the
	// new cells immediately; otherwise the first draw renders.
	if t.lastWidth > 0 {
		t.render(t.lastWidth)
	} else {
		t.dirty = true
	}
}

// render rebuilds all cells for the given inner width.
func (t *Table) render(width int) {
	t.lastWidth = width
	t.dirty = false

	l := fitColumns(width, t.in, t.offset)
	t.layout = l

	view := t.view.Table
	view.Clear()

	// Header cells are padded to the full column width, data cells are capped
	// at it, so every column has a stable width regardless of scrolling.
	for pos, col := range l.columns {
		h := l.headers[col]
		text := h
		if pad := l.widths[col] - tview.TaggedStringWidth(h); pad > 0 {
			if col == ColDownloads {
				text = strings.Repeat(" ", pad) + h
			} else {
				text = h + strings.Repeat(" ", pad)
			}
		}
		view.SetCell(0, pos, &tview.TableCell{
			Text:            text,
			NotSelectable:   true,
			Align:           tview.AlignLeft,
			MaxWidth:        l.widths[col],
			Color:           t.theme.TableHeaderColor,
			BackgroundColor: t.theme.DefaultBgColor,
		})
	}

	for i, r := range t.rows {
		row := i + 1
		for pos, col := range l.columns {
			var c *tview.TableCell
			switch col {
			case ColType:
				text := r.Type
				if !l.compact {
					text = "[" + r.Type + "]"
				}
				c = t.cell(text, l.widths[col], tcell.ColorDefault)
			case ColName:
				c = t.cell(r.Name, l.widths[col], r.NameColor).SetAttributes(r.NameAttr)
			case ColVersion:
				c = t.cell(r.Version, l.widths[col], r.VersionColor).SetAttributes(r.VersionAttr)
			case ColDescription:
				c = t.cell(r.Description, l.widths[col], tcell.ColorDefault)
			case ColDownloads:
				c = t.cell(r.Downloads, l.widths[col], tcell.ColorDefault).SetAlign(tview.AlignRight)
			}
			view.SetCell(row, pos, c)
		}
	}
}

// drawScrollHints marks the header with ‹ and › when columns are scrolled
// out of view on either side. They go in separator columns, which are
// otherwise blank.
func (t *Table) drawScrollHints(screen tcell.Screen, x, y, width int) {
	l := t.layout
	style := tcell.StyleDefault.Foreground(t.theme.TableHeaderColor).Background(t.theme.DefaultBgColor)
	if l.offset > 0 {
		screen.SetContent(x+l.widths[ColType]+1+l.widths[ColName], y, '‹', nil, style)
	}
	if l.offset < l.maxOffset {
		screen.SetContent(x+width-1, y, '›', nil, style)
	}
}

func (t *Table) cell(text string, width int, color tcell.Color) *tview.TableCell {
	c := tview.NewTableCell(tview.Escape(text)).SetMaxWidth(width).SetSelectable(true)
	if color != tcell.ColorDefault {
		c.SetTextColor(color)
	}
	return c
}
