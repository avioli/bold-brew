package services

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"bbrew/internal/config"
	"bbrew/internal/ui/components"
)

// optionField is one row of the options form: how to build it from the
// settings and how to read it back.
type optionField struct {
	label string
	item  tview.FormItem
	read  func(c *config.Config)
}

// optionsForm builds the form rows for c, in display order.
func optionsForm(c config.Config) []optionField {
	var fields []optionField

	choice := func(label string, options []string, value string, set func(c *config.Config, v string)) {
		ch := components.NewChoice(label, options, slices.Index(options, value))
		fields = append(fields, optionField{label, ch, func(c *config.Config) {
			_, v := ch.Current()
			set(c, v)
		}})
	}
	toggle := func(label string, value bool, set func(c *config.Config, v bool)) {
		tg := components.NewToggle(label, value)
		fields = append(fields, optionField{label, tg, func(c *config.Config) { set(c, tg.On()) }})
	}
	number := func(label string, value int, set func(c *config.Config, v int)) {
		in := tview.NewInputField().SetText(strconv.Itoa(value)).SetFieldWidth(8).
			SetAcceptanceFunc(tview.InputFieldInteger)
		fields = append(fields, optionField{label, in, func(c *config.Config) {
			if v, err := strconv.Atoi(in.GetText()); err == nil {
				set(c, v)
			}
		}})
	}
	text := func(label, value string, set func(c *config.Config, v string)) {
		in := tview.NewInputField().SetText(value) // fills the row
		fields = append(fields, optionField{label, in, func(c *config.Config) { set(c, in.GetText()) }})
	}

	choice("View: start sort", config.SortModes, c.View.Sort, func(c *config.Config, v string) { c.View.Sort = v })
	choice("View: start filter", config.StatusFilters, c.View.Filter, func(c *config.Config, v string) { c.View.Filter = v })
	choice("View: start kind", config.KindFilters, c.View.Kind, func(c *config.Config, v string) { c.View.Kind = v })
	toggle("View: borders", c.View.Borders, func(c *config.Config, v bool) { c.View.Borders = v })
	toggle("View: details panel", c.View.Details, func(c *config.Config, v bool) { c.View.Details = v })
	toggle("View: Log panel", c.View.Log, func(c *config.Config, v bool) { c.View.Log = v })
	number("View: Log height", c.View.LogHeight, func(c *config.Config, v int) { c.View.LogHeight = v })

	toggle("Startup: brew update", c.Startup.BrewUpdate, func(c *config.Config, v bool) { c.Startup.BrewUpdate = v })
	toggle("Startup: check for bbrew update", c.Startup.CheckBbrewUpdate, func(c *config.Config, v bool) { c.Startup.CheckBbrewUpdate = v })
	toggle("Startup: measure disk usage", c.Startup.DiskScan, func(c *config.Config, v bool) { c.Startup.DiskScan = v })

	choice("Release age: look up", config.ReleaseAgeModes, c.ReleaseAge.Mode, func(c *config.Config, v string) { c.ReleaseAge.Mode = v })
	toggle("Release age: casks (Last-Modified)", c.ReleaseAge.Casks, func(c *config.Config, v bool) { c.ReleaseAge.Casks = v })
	number("Release age: recent, hours", c.ReleaseAge.RecentHours, func(c *config.Config, v int) { c.ReleaseAge.RecentHours = v })
	number("Release age: very fresh, hours", c.ReleaseAge.FreshHours, func(c *config.Config, v int) { c.ReleaseAge.FreshHours = v })
	toggle("Release age: hold recent in Update all", c.ReleaseAge.HoldInUpdateAll, func(c *config.Config, v bool) { c.ReleaseAge.HoldInUpdateAll = v })

	toggle("Actions: confirm install/update/remove", c.Actions.Confirm, func(c *config.Config, v bool) { c.Actions.Confirm = v })
	toggle("Actions: Update all is greedy", c.Actions.Greedy, func(c *config.Config, v bool) { c.Actions.Greedy = v })
	text("Actions: export Brewfile to", c.Actions.ExportPath, func(c *config.Config, v string) { c.Actions.ExportPath = v })

	choice("Vulns: minimum severity", config.Severities, c.Vulns.MinSeverity, func(c *config.Config, v string) { c.Vulns.MinSeverity = v })

	number("Cache: API lists, hours", c.Cache.APITTLHours, func(c *config.Config, v int) { c.Cache.APITTLHours = v })
	return fields
}

// handleOptionsEvent opens the options panel (o). Save writes the settings
// file and applies what can change live; the start sort and filter apply
// from the next start.
func (s *InputService) handleOptionsEvent() {
	s.showOptions(config.Get())
}

func (s *InputService) showOptions(start config.Config) {
	fields := optionsForm(start)

	form := tview.NewForm().
		SetFieldStyle(tcell.StyleDefault.Reverse(true)).
		SetButtonStyle(tcell.StyleDefault.Reverse(true)).
		SetButtonActivatedStyle(tcell.StyleDefault.Background(s.appService.theme.SuccessColor).
			Foreground(tcell.ColorBlack).Bold(true)).
		SetButtonsAlign(tview.AlignCenter).
		SetItemPadding(0)
	form.SetBorder(true).
		SetTitle(" Options (←/→ change, Ctrl+S save, Esc cancel) ").
		SetBorderPadding(0, 0, 1, 1)
	for _, f := range fields {
		form.AddFormItem(f.item)
	}

	// Mark the focused row before each draw, so moving up and down is easy
	// to follow (with NO_COLOR too).
	view := &markedForm{Form: form, before: func() {
		focused, _ := form.GetFocusedItemIndex()
		for i, f := range fields {
			prefix := "  "
			if i == focused {
				prefix = "▸ "
			}
			switch item := f.item.(type) {
			case *components.Choice:
				item.SetLabel(prefix + f.label)
			case *tview.InputField:
				item.SetLabel(prefix + f.label)
			}
		}
	}}

	save := func() {
		c := start
		for _, f := range fields {
			f.read(&c)
		}
		c.Normalize()
		path := config.Path()
		if err := config.Save(path, c); err != nil {
			s.layout.GetNotifier().ShowError(fmt.Sprintf("Could not save settings: %v", err))
			return
		}
		config.Set(c)
		s.closeOverlay()
		s.layout.ApplyPrefs(c.View.Borders, c.View.Details, c.View.Log, c.View.LogHeight)
		s.appService.releaseAgeChanged()
		s.layout.GetNotifier().ShowSuccess("Settings saved to " + path)
	}
	form.AddButton("Save", save).
		AddButton("Defaults", func() { s.showOptions(config.Default()) }).
		AddButton("Cancel", s.closeOverlay)

	// Keys before the form sees them: Ctrl+S saves, Esc cancels, Up/Down
	// move between rows (Left/Right change a choice, or move the cursor in
	// a text field).
	s.overlayCapture = func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlS:
			save()
			return nil
		case tcell.KeyEsc:
			s.closeOverlay()
			return nil
		case tcell.KeyUp:
			return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
		case tcell.KeyDown:
			return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
		}
		return event
	}

	width := min(76, max(s.layout.Width()-4, 40))
	height := min(len(fields)+5, max(s.layout.Height()-2, 10)) // + borders, gap, buttons, spare
	centered := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(view, height, 0, true).
			AddItem(nil, 0, 1, false),
			width, 0, true).
		AddItem(nil, 0, 1, false)

	pages := tview.NewPages().
		AddPage("main", s.layout.Root(), true, true).
		AddPage("options", centered, true, true)
	s.appService.GetApp().SetRoot(pages, true)
	s.appService.GetApp().SetFocus(view)
}

// markedForm runs before ahead of every draw of the form.
type markedForm struct {
	*tview.Form
	before func()
}

func (f *markedForm) Draw(screen tcell.Screen) {
	f.before()
	f.Form.Draw(screen)
}
