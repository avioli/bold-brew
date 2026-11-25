package components

import (
	"bbrew/internal/ui/theme"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// HelpKey is one shortcut line in the help screen.
type HelpKey struct {
	Keys        string
	Description string
}

// HelpSection is a titled group of shortcuts.
type HelpSection struct {
	Title string
	Keys  []HelpKey
}

const (
	helpKeyWidth = 22
	helpBoxWidth = 64
)

// HelpScreen displays a modal overlay with all keyboard shortcuts
type HelpScreen struct {
	pages *tview.Pages
	theme *theme.Theme
}

// NewHelpScreen creates a new help screen component
func NewHelpScreen(theme *theme.Theme) *HelpScreen {
	return &HelpScreen{
		pages: tview.NewPages(),
		theme: theme,
	}
}

// View returns the help screen pages (for overlay functionality)
func (h *HelpScreen) View() *tview.Pages {
	return h.pages
}

// Build creates the help screen as an overlay on top of the main content.
// The box is at most maxHeight rows high (0 for no limit) and scrolls when
// the content does not fit. It returns the pages and the scrollable text.
func (h *HelpScreen) Build(mainContent tview.Primitive, sections []HelpSection, maxHeight int) (*tview.Pages, *tview.TextView) {
	content, lines := h.buildHelpContent(sections)

	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetText(content).
		SetTextAlign(tview.AlignLeft)

	textView.SetBackgroundColor(h.theme.ModalBgColor)
	textView.SetTextColor(h.theme.DefaultTextColor)
	textView.SetBorder(true).
		SetBorderPadding(0, 0, 2, 2).
		SetBorderColor(h.theme.BorderColor).
		SetTitle(" Help ").
		SetTitleAlign(tview.AlignCenter)

	boxHeight := lines + 2 // borders
	if maxHeight > 0 {
		boxHeight = min(boxHeight, maxHeight)
	}

	// Center the box in a flex layout
	centered := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(textView, boxHeight, 0, true).
			AddItem(nil, 0, 1, false),
			helpBoxWidth, 0, true).
		AddItem(nil, 0, 1, false)

	// Create pages with main content as background and help as overlay
	h.pages = tview.NewPages().
		AddPage("main", mainContent, true, true).
		AddPage("help", centered, true, true)

	return h.pages, textView
}

// buildHelpContent generates the formatted help text and its line count.
func (h *HelpScreen) buildHelpContent(sections []HelpSection) (string, int) {
	var sb strings.Builder
	lines := 0

	for _, section := range sections {
		sb.WriteString(h.formatSection(section.Title))
		lines++
		for _, k := range section.Keys {
			sb.WriteString(h.formatKey(k.Keys, k.Description))
			lines++
		}
		sb.WriteString("\n")
		lines++
	}

	fmt.Fprintf(&sb, "[%s]j/k to scroll, any other key to close[-]", h.getColorTag(h.theme.LegendColor))
	lines++

	return sb.String(), lines
}

// formatSection formats a section header
func (h *HelpScreen) formatSection(title string) string {
	return fmt.Sprintf("[%s::b]%s[-:-:-]\n", h.getColorTag(h.theme.SuccessColor), tview.Escape(title))
}

// formatKey formats a key-description pair
func (h *HelpScreen) formatKey(key, description string) string {
	return fmt.Sprintf("  [%s]%s[-] %s\n", h.getColorTag(h.theme.WarningColor),
		tview.Escape(padRight(key, helpKeyWidth)), tview.Escape(description))
}

func padRight(s string, width int) string {
	if pad := width - tview.TaggedStringWidth(tview.Escape(s)); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

// getColorTag converts a tcell.Color to a tview color tag
func (h *HelpScreen) getColorTag(color tcell.Color) string {
	return fmt.Sprintf("#%06x", color.Hex())
}
