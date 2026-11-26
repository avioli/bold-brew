package components

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"bbrew/internal/ui/theme"
)

type Search struct {
	field   *tview.InputField
	counter *tview.TextView
	theme   *theme.Theme

	compact         bool
	total, filtered int
}

func NewSearch(theme *theme.Theme) *Search {
	search := &Search{
		field:   tview.NewInputField(),
		counter: tview.NewTextView(),
		theme:   theme,
	}

	search.field.SetLabel(tview.Escape("Search (All): "))
	search.field.SetLabelColor(theme.SearchLabelColor)
	search.field.SetFieldStyle(tcell.StyleDefault.Italic(true).Underline(true))
	search.field.SetFieldBackgroundColor(theme.DefaultBgColor)
	search.field.SetFieldTextColor(theme.DefaultTextColor)
	search.counter.SetDynamicColors(true)
	search.counter.SetTextAlign(tview.AlignRight)
	return search
}

func (s *Search) SetHandlers(done func(key tcell.Key), changed func(text string)) {
	s.field.SetDoneFunc(done)
	s.field.SetChangedFunc(changed)
}

func (s *Search) UpdateCounter(total, filtered int) {
	s.total, s.filtered = total, filtered
	s.renderCounter()
}

// SetCompact switches the counter between "Filtered: M | Total: N" and "M/N".
func (s *Search) SetCompact(compact bool) {
	if s.compact != compact {
		s.compact = compact
		s.renderCounter()
	}
}

func (s *Search) renderCounter() {
	// Only mention the filtered count when something is filtered out.
	switch {
	case s.filtered == s.total && s.compact:
		s.counter.SetText(fmt.Sprintf("%d", s.total))
	case s.filtered == s.total:
		s.counter.SetText(fmt.Sprintf("Total: %d", s.total))
	case s.compact:
		s.counter.SetText(fmt.Sprintf("%d/%d", s.filtered, s.total))
	default:
		s.counter.SetText(fmt.Sprintf("Filtered: %d | Total: %d", s.filtered, s.total))
	}
}

func (s *Search) Field() *tview.InputField {
	return s.field
}

func (s *Search) Counter() *tview.TextView {
	return s.counter
}
