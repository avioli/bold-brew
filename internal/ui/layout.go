package ui

import (
	"bbrew/internal/ui/components"
	"bbrew/internal/ui/theme"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Screen width breakpoints, from widest to narrowest. At or below each width
// the layout gives up a little more decoration to keep the table readable.
const (
	HideDownloadsWidth = 146 // hide the Downloads column
	CompactPadWidth    = 120 // shrink frame padding, short counter label
	CompactTypeWidth   = 100 // Type column becomes "T" with "F"/"C"
	HideDescWidth      = 99  // hide the Description column (below 100), until details hides
	NoBordersWidth     = 90  // drop the outer and row borders
	HideDetailsWidth   = 80  // hide the details panel
)

const (
	widePadding    = 3
	compactPadding = 1
	outputHeight   = 5
)

type LayoutInterface interface {
	Setup()
	Root() tview.Primitive

	// Resize adapts the layout to the screen size. Cheap to call on every draw.
	Resize(width, height int)
	// ToggleBorders hides or shows the borders (they are also hidden
	// automatically on narrow screens).
	ToggleBorders()
	// DetailsVisible reports whether the details panel is shown.
	DetailsVisible() bool
	// SetPackagesTitle sets the title of the package panel, e.g. "Installed Casks".
	SetPackagesTitle(title string)
	Height() int

	GetSearch() *components.Search
	GetTable() *components.Table
	GetDetails() *components.Details
	GetOutput() *components.Output
	GetLegend() *components.Legend
	GetNotifier() *components.Notifier
	GetModal() *components.Modal
	GetHelpScreen() *components.HelpScreen
}

// Panel titles drawn on the outer border.
const (
	titlePackagesDefault = "Formulas & Casks"
	titleDetails         = "Details"
	titleOutput          = "Log"
)

// titledGrid draws panel titles onto the grid's borders after the grid itself.
type titledGrid struct {
	*tview.Grid
	layout *Layout
}

func (g *titledGrid) Draw(screen tcell.Screen) {
	g.Grid.Draw(screen)
	g.layout.drawTitles(screen)
}

type Layout struct {
	grid        *titledGrid
	leftColumn  *tview.Flex
	mainContent *tview.Flex
	filtersArea *tview.Frame
	tableFrame  *tview.Frame

	search     *components.Search
	table      *components.Table
	details    *components.Details
	output     *components.Output
	legend     *components.Legend
	notifier   *components.Notifier
	modal      *components.Modal
	helpScreen *components.HelpScreen

	width, height int
	bordersHidden bool // user preference
	bordersShown  bool // current state
	detailsShown  bool
	packagesTitle string
	dirty         bool
	theme         *theme.Theme
}

func NewLayout(t *theme.Theme) LayoutInterface {
	l := &Layout{
		theme:         t,
		search:        components.NewSearch(t),
		table:         components.NewTable(t),
		details:       components.NewDetails(t),
		output:        components.NewOutput(t),
		legend:        components.NewLegend(t),
		notifier:      components.NewNotifier(t),
		modal:         components.NewModal(t),
		helpScreen:    components.NewHelpScreen(t),
		dirty:         true,
		packagesTitle: titlePackagesDefault,
	}
	l.grid = &titledGrid{Grid: tview.NewGrid(), layout: l}
	return l
}

func (l *Layout) setupLayout() {
	// Search and counter
	searchRow := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(l.search.Field(), 0, 1, false).
		AddItem(l.search.Counter(), 0, 1, false)

	l.filtersArea = tview.NewFrame(searchRow).
		SetBorders(0, 0, 0, 0, widePadding, widePadding)

	l.tableFrame = tview.NewFrame(l.table.Primitive()).
		SetBorders(0, 0, 0, 0, widePadding, widePadding)

	// Left column with search and table
	l.leftColumn = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(l.filtersArea, 2, 0, false).
		AddItem(l.tableFrame, 0, 1, false)

	// Central content: table takes the rest, details has a fixed width set
	// by Resize.
	l.mainContent = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(l.leftColumn, 0, 1, false).
		AddItem(l.details.View(), components.DetailsMinWidth, 0, false)

	// Status line: messages on the left, short help on the right
	statusRow := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(l.notifier.View(), 0, 1, false).
		AddItem(l.legend.View(), l.legend.Width(), 0, false)

	// Final layout: main content, output (full width), status line
	l.grid.
		SetRows(0, outputHeight, 1).
		SetColumns(0).
		SetBorders(true).
		AddItem(l.mainContent, 0, 0, 1, 1, 0, 0, true).
		AddItem(l.output.View(), 1, 0, 1, 1, 0, 0, false).
		AddItem(statusRow, 2, 0, 1, 1, 0, 0, false)
}

func (l *Layout) Setup() {
	l.setupLayout()
}

func (l *Layout) Root() tview.Primitive {
	return l.grid
}

func (l *Layout) Height() int { return l.height }

func (l *Layout) ToggleBorders() {
	l.bordersHidden = !l.bordersHidden
	l.dirty = true
}

func (l *Layout) Resize(width, height int) {
	if !l.dirty && width == l.width && height == l.height {
		return
	}
	l.width, l.height, l.dirty = width, height, false

	l.bordersShown = !l.bordersHidden && width > NoBordersWidth
	l.grid.SetBorders(l.bordersShown)
	l.details.SetInlineTitle(!l.bordersShown)

	pad := widePadding
	if width <= CompactPadWidth {
		pad = compactPadding
	}
	l.filtersArea.SetBorders(0, 0, 0, 0, pad, pad)
	l.tableFrame.SetBorders(0, 0, 0, 0, pad, pad)

	l.detailsShown = width > HideDetailsWidth
	detailsWidth := 0
	if l.detailsShown {
		detailsWidth = max(components.DetailsMinWidth, width/4)
	}
	l.mainContent.ResizeItem(l.details.View(), detailsWidth, 0)

	l.search.SetCompact(width <= CompactPadWidth)
	l.table.SetCompactType(width <= CompactTypeWidth)
	l.table.SetShowDownloads(width > HideDownloadsWidth)
	// Description returns when the details panel hides and frees its space.
	l.table.SetShowDescription(width > HideDescWidth || !l.detailsShown)
}

func (l *Layout) DetailsVisible() bool { return l.detailsShown }

func (l *Layout) SetPackagesTitle(title string) {
	if title == "" {
		title = titlePackagesDefault
	}
	l.packagesTitle = title
}

func (l *Layout) GetSearch() *components.Search         { return l.search }
func (l *Layout) GetTable() *components.Table           { return l.table }
func (l *Layout) GetDetails() *components.Details       { return l.details }
func (l *Layout) GetOutput() *components.Output         { return l.output }
func (l *Layout) GetLegend() *components.Legend         { return l.legend }
func (l *Layout) GetNotifier() *components.Notifier     { return l.notifier }
func (l *Layout) GetModal() *components.Modal           { return l.modal }
func (l *Layout) GetHelpScreen() *components.HelpScreen { return l.helpScreen }

// drawTitles writes each panel's title onto the border above it and joins the
// details separator to the horizontal borders. The focused panel's title is
// highlighted.
func (l *Layout) drawTitles(screen tcell.Screen) {
	if !l.bordersShown {
		return
	}

	borderStyle := tcell.StyleDefault.Foreground(l.theme.BorderColor)
	titleStyle := tcell.StyleDefault.Foreground(l.theme.TitleColor)

	// indent is the number of border columns before the title.
	title := func(p tview.Primitive, text string, focused bool, indent int) {
		x, y, width, _ := p.GetRect()
		x, width = x+indent-1, width-indent+1
		style := titleStyle
		if focused {
			style = style.Reverse(true)
		}
		for i, ch := range []rune(" " + text + " ") {
			if i >= width-2 {
				break
			}
			screen.SetContent(x+1+i, y-1, ch, nil, style)
		}
	}

	title(l.leftColumn, l.packagesTitle, l.table.View().HasFocus(), 1)
	title(l.output.View(), titleOutput, l.output.View().HasFocus(), 1)
	if !l.detailsShown {
		return
	}
	title(l.details.View(), titleDetails, l.details.View().HasFocus(), 2)

	// The details panel draws its own vertical separator; give it proper
	// junctions with the borders above and below.
	x, y, _, height := l.details.View().GetRect()
	screen.SetContent(x, y-1, tview.Borders.TopT, nil, borderStyle)
	screen.SetContent(x, y+height, tview.Borders.BottomT, nil, borderStyle)
}
