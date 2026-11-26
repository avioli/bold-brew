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

const helpFooter = " j/k to scroll, any other key to close "

const (
	helpKeyWidth = 22
	// Box chrome around the text: borders (2) and padding (2 + 2).
	helpChromeWidth = 6
	// Key lines are "  <key padded to helpKeyWidth> <description>".
	helpKeyIndent = 2
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
// The box is as wide as its longest line, up to maxWidth (0 for no limit).
func (h *HelpScreen) Build(mainContent tview.Primitive, sections []HelpSection, maxWidth, maxHeight int) (*tview.Pages, *tview.TextView) {
	content, lines := h.buildHelpContent(sections)

	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetText(content).
		SetTextAlign(tview.AlignLeft)

	textView.SetBackgroundColor(h.theme.ModalBgColor)
	textView.SetTextColor(h.theme.DefaultTextColor)

	frame := tview.NewFrame(textView).
		SetBorders(0, 0, 0, 0, 2, 2)
	frame.SetBackgroundColor(h.theme.ModalBgColor)
	frame.SetBorder(true).
		SetBorderColor(h.theme.BorderColor).
		SetTitle(" Help ").
		SetTitleAlign(tview.AlignCenter)

	// Print the hint on the bottom border, like the title on the top one, so
	// it stays visible while the content scrolls.
	frame.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		tview.Print(screen, helpFooter, x+1, y+height-1, width-2, tview.AlignCenter, h.theme.LegendColor)
		return x + 1, y + 1, width - 2, height - 2
	})

	boxHeight := lines + 2 // borders
	if maxHeight > 0 {
		boxHeight = min(boxHeight, maxHeight)
	}

	// Center the box in a flex layout
	centered := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(frame, boxHeight, 0, true).
			AddItem(nil, 0, 1, false),
			h.boxWidth(sections, maxWidth), 0, true).
		AddItem(nil, 0, 1, false)

	// Create pages with main content as background and help as overlay
	h.pages = tview.NewPages().
		AddPage("main", mainContent, true, true).
		AddPage("help", centered, true, true)

	return h.pages, textView
}

// boxWidth fits the longest line (and the footer hint) without wrapping.
func (h *HelpScreen) boxWidth(sections []HelpSection, maxWidth int) int {
	width := tview.TaggedStringWidth(tview.Escape(helpFooter)) + 2
	for _, section := range sections {
		width = max(width, tview.TaggedStringWidth(tview.Escape(section.Title)))
		for _, k := range section.Keys {
			key := max(helpKeyWidth, tview.TaggedStringWidth(tview.Escape(k.Keys)))
			width = max(width, helpKeyIndent+key+1+tview.TaggedStringWidth(tview.Escape(k.Description)))
		}
	}
	width += helpChromeWidth
	if maxWidth > 0 {
		width = min(width, maxWidth)
	}
	return width
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

	// Drop the trailing blank line after the last section.
	return strings.TrimSuffix(sb.String(), "\n"), lines - 1
}

// formatSection formats a section header
func (h *HelpScreen) formatSection(title string) string {
	return fmt.Sprintf("[%s::b]%s[-:-:-]\n", h.getColorTag(h.theme.SuccessColor), tview.Escape(title))
}

// formatKey formats a key-description pair
func (h *HelpScreen) formatKey(key, description string) string {
	// Without colour, keys stand out in bold instead.
	style := h.getColorTag(h.theme.WarningColor)
	if theme.NoColor() {
		style += "::b"
	}
	return fmt.Sprintf("  [%s]%s[-:-:-] %s\n", style,
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
