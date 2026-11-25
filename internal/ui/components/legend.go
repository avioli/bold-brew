package components

import (
	"github.com/rivo/tview"

	"bbrew/internal/ui/theme"
)

// LegendText is the short help line shown in the status bar.
const LegendText = "? Help | Esc/q Quit"

type Legend struct {
	view  *tview.TextView
	theme *theme.Theme
}

func NewLegend(theme *theme.Theme) *Legend {
	legendView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignRight).
		SetTextColor(theme.LegendColor).
		SetText(LegendText)

	return &Legend{
		view:  legendView,
		theme: theme,
	}
}

func (l *Legend) View() *tview.TextView {
	return l.view
}

// Width is the number of columns the legend needs.
func (l *Legend) Width() int {
	return tview.TaggedStringWidth(LegendText) + 1
}
