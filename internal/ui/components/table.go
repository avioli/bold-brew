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
)

const (
	maxNameWidth    = 32
	maxVersionWidth = 15
	minDescWidth    = 1
	sortIndicator   = " ▼"
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
}

// Table renders package rows and fits its columns to the available width:
// Type, Name, Version and Downloads keep their width and Description absorbs
// the pressure.
type Table struct {
	view  *fitTable
	theme *theme.Theme

	rows       []Row
	sortColumn int // column showing the sort indicator, -1 for none

	// Natural (uncapped by available space) widths, computed from all rows.
	nameWidth, versionWidth, downloadsWidth int

	compactType   bool
	showDownloads bool

	lastWidth int
	dirty     bool
}

// fitTable wraps tview.Table to re-fit columns whenever its width changes.
type fitTable struct {
	*tview.Table
	owner *Table
}

func (f *fitTable) Draw(screen tcell.Screen) {
	_, _, width, _ := f.GetInnerRect()
	if f.owner.dirty || width != f.owner.lastWidth {
		f.owner.render(width)
	}
	f.Table.Draw(screen)
}

func NewTable(theme *theme.Theme) *Table {
	table := &Table{
		theme:         theme,
		sortColumn:    -1,
		showDownloads: true,
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

// SetCompactType switches the Type column between "Type"/"[F]" and "T"/"F".
func (t *Table) SetCompactType(compact bool) {
	if t.compactType != compact {
		t.compactType = compact
		t.dirty = true
	}
}

// SetShowDownloads shows or hides the Downloads column.
func (t *Table) SetShowDownloads(show bool) {
	if t.showDownloads != show {
		t.showDownloads = show
		t.dirty = true
	}
}

// SetRows replaces the table content. sortColumn is the column to mark as
// the sort key, or -1.
func (t *Table) SetRows(rows []Row, sortColumn int) {
	t.rows = rows
	t.sortColumn = sortColumn

	t.nameWidth, t.versionWidth, t.downloadsWidth = 0, 0, 0
	for _, r := range rows {
		t.nameWidth = max(t.nameWidth, tview.TaggedStringWidth(tview.Escape(r.Name)))
		t.versionWidth = max(t.versionWidth, tview.TaggedStringWidth(tview.Escape(r.Version)))
		t.downloadsWidth = max(t.downloadsWidth, len(r.Downloads))
	}
	t.nameWidth = min(t.nameWidth, maxNameWidth)
	t.versionWidth = min(t.versionWidth, maxVersionWidth)

	// Render right away when the width is known, so selection works on the
	// new cells immediately; otherwise the first draw renders.
	if t.lastWidth > 0 {
		t.render(t.lastWidth)
	} else {
		t.dirty = true
	}
}

func (t *Table) header(col int, label string) string {
	if col == t.sortColumn {
		return label + sortIndicator
	}
	return label
}

// render rebuilds all cells for the given inner width.
func (t *Table) render(width int) {
	t.lastWidth = width
	t.dirty = false

	typeHeader := "Type"
	typeWidth := 4
	if t.compactType {
		typeHeader, typeWidth = "T", 1
	}

	headers := []string{
		typeHeader,
		t.header(ColName, "Name"),
		t.header(ColVersion, "Version"),
		t.header(ColDescription, "Description"),
	}
	if t.showDownloads {
		headers = append(headers, t.header(ColDownloads, "Downloads"))
	}

	widths := []int{
		typeWidth,
		max(t.nameWidth, tview.TaggedStringWidth(headers[ColName])),
		max(t.versionWidth, tview.TaggedStringWidth(headers[ColVersion])),
		0, // description, computed below
	}
	if t.showDownloads {
		widths = append(widths, max(t.downloadsWidth, tview.TaggedStringWidth(headers[ColDownloads])))
	}

	// tview reserves one column of separator after every column.
	used := 0
	for i, w := range widths {
		if i != ColDescription {
			used += w + 1
		}
	}
	widths[ColDescription] = max(width-used-1, minDescWidth)

	view := t.view.Table
	view.Clear()

	// Header cells are padded to the full column width, data cells are capped
	// at it, so every column has a stable width regardless of scrolling.
	for col, h := range headers {
		text := h
		pad := widths[col] - tview.TaggedStringWidth(h)
		if pad > 0 {
			if col == ColDownloads {
				text = strings.Repeat(" ", pad) + h
			} else {
				text = h + strings.Repeat(" ", pad)
			}
		}
		view.SetCell(0, col, &tview.TableCell{
			Text:            text,
			NotSelectable:   true,
			Align:           tview.AlignLeft,
			MaxWidth:        widths[col],
			Color:           t.theme.TableHeaderColor,
			BackgroundColor: t.theme.DefaultBgColor,
		})
	}

	for i, r := range t.rows {
		row := i + 1
		typeText := r.Type
		if !t.compactType {
			typeText = "[" + r.Type + "]"
		}
		view.SetCell(row, ColType, t.cell(typeText, widths[ColType], tcell.ColorDefault))
		view.SetCell(row, ColName, t.cell(r.Name, widths[ColName], r.NameColor))
		view.SetCell(row, ColVersion, t.cell(r.Version, widths[ColVersion], r.VersionColor))
		view.SetCell(row, ColDescription, t.cell(r.Description, widths[ColDescription], tcell.ColorDefault))
		if t.showDownloads {
			view.SetCell(row, ColDownloads, t.cell(r.Downloads, widths[ColDownloads], tcell.ColorDefault).
				SetAlign(tview.AlignRight))
		}
	}
}

func (t *Table) cell(text string, width int, color tcell.Color) *tview.TableCell {
	c := tview.NewTableCell(tview.Escape(text)).SetMaxWidth(width).SetSelectable(true)
	if color != tcell.ColorDefault {
		c.SetTextColor(color)
	}
	return c
}
