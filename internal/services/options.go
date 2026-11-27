package services

import (
	"fmt"
	"strconv"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"bbrew/internal/config"
	"bbrew/internal/ui/components"
)

// Options panel sections, in display order.
const (
	secView       = "View"
	secStartup    = "Startup"
	secReleaseAge = "Release age"
	secActions    = "Actions"
	secVulns      = "Vulns"
	secCache      = "Cache"
)

// optionField ties an options panel item to the setting it edits.
type optionField struct {
	item *components.OptionItem
	get  func(c config.Config) string
	set  func(c *config.Config, v string)
}

// yesNo converts between bool settings and the panel's yes/no values.
func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// describeBy describes a value: intro, then the value's own text, if any.
func describeBy(intro string, byValue map[string]string) func(string) string {
	return func(v string) string {
		if text := byValue[v]; text != "" {
			if intro == "" {
				return text
			}
			return intro + " " + text
		}
		return intro
	}
}

// describe returns a fixed description.
func describe(text string) func(string) string { return func(string) string { return text } }

// optionFields lists the settings shown in the options panel.
func optionFields() []optionField {
	var fields []optionField
	choice := func(section, label string, choices []string, desc func(string) string,
		get func(c config.Config) string, set func(c *config.Config, v string)) {
		fields = append(fields, optionField{&components.OptionItem{
			Section: section, Label: label, Kind: components.OptionChoice, Choices: choices, Describe: desc,
		}, get, set})
	}
	toggle := func(section, label string, desc func(string) string,
		get func(c config.Config) bool, set func(c *config.Config, v bool)) {
		choice(section, label, []string{"no", "yes"}, desc,
			func(c config.Config) string { return yesNo(get(c)) },
			func(c *config.Config, v string) { set(c, v == "yes") })
	}
	number := func(section, label string, desc func(string) string,
		get func(c config.Config) int, set func(c *config.Config, v int)) {
		fields = append(fields, optionField{&components.OptionItem{
			Section: section, Label: label, Kind: components.OptionNumber, Describe: desc,
		}, func(c config.Config) string { return strconv.Itoa(get(c)) },
			func(c *config.Config, v string) {
				if n, err := strconv.Atoi(v); err == nil {
					set(c, n)
				}
			}})
	}
	text := func(section, label string, desc func(string) string,
		get func(c config.Config) string, set func(c *config.Config, v string)) {
		fields = append(fields, optionField{&components.OptionItem{
			Section: section, Label: label, Kind: components.OptionText, Describe: desc,
		}, get, set})
	}

	choice(secView, "Start sort", config.SortModes,
		describeBy("Sort order at start (s cycles it).", map[string]string{
			"name":      "Alphabetical.",
			"downloads": "Most downloaded in the last 90 days first.",
			"rank":      "Best 90 day analytics rank first, unranked last.",
			"size":      "Most disk space first (needs the disk usage scan).",
		}),
		func(c config.Config) string { return c.View.Sort },
		func(c *config.Config, v string) { c.View.Sort = v })
	choice(secView, "Start filter", config.StatusFilters,
		describeBy("Filter at start (f opens the filter picker).", map[string]string{
			"none":      "All packages.",
			"installed": "Installed packages only.",
			"outdated":  "Installed packages with an update available.",
			"leaves":    "Installed formulae that nothing else depends on.",
			"new":       "Packages new since the last brew update that changed the lists.",
		}),
		func(c config.Config) string { return c.View.Filter },
		func(c *config.Config, v string) { c.View.Filter = v })
	choice(secView, "Start kind", config.KindFilters,
		describeBy("Package kinds shown at start, combined with the filter.", map[string]string{
			"formulae": "Formulae only.",
			"casks":    "Casks only.",
		}),
		func(c config.Config) string { return c.View.Kind },
		func(c *config.Config, v string) { c.View.Kind = v })
	toggle(secView, "Borders",
		describe("Draw borders around the panels (b toggles). Hidden on narrow terminals anyway."),
		func(c config.Config) bool { return c.View.Borders },
		func(c *config.Config, v bool) { c.View.Borders = v })
	toggle(secView, "Details panel",
		describe("Show the details panel (D toggles). Hidden on narrow terminals anyway."),
		func(c config.Config) bool { return c.View.Details },
		func(c *config.Config, v bool) { c.View.Details = v })
	toggle(secView, "Log panel",
		describe("Show the Log panel (L toggles). It reappears when a command writes to it."),
		func(c config.Config) bool { return c.View.Log },
		func(c *config.Config, v bool) { c.View.Log = v })
	number(secView, "Log height",
		describe(fmt.Sprintf("Rows for the Log panel, %d to %d. Alt+Up/Down resize it while running.",
			config.MinLogHeight, config.MaxLogHeight)),
		func(c config.Config) int { return c.View.LogHeight },
		func(c *config.Config, v int) { c.View.LogHeight = v })

	toggle(secStartup, "brew update",
		describeBy("", map[string]string{
			"yes": "Run brew update in the background at start, so versions and outdated packages are current.",
			"no":  "Skip brew update at start; versions are as of Homebrew's last update (installed state is still refreshed).",
		}),
		func(c config.Config) bool { return c.Startup.BrewUpdate },
		func(c *config.Config, v bool) { c.Startup.BrewUpdate = v })
	toggle(secStartup, "Check for a bbrew update",
		describe("Look for a newer bbrew at start and say so in the status line."),
		func(c config.Config) bool { return c.Startup.CheckBbrewUpdate },
		func(c *config.Config, v bool) { c.Startup.CheckBbrewUpdate = v })
	toggle(secStartup, "Measure disk usage",
		describeBy("", map[string]string{
			"yes": "Measure installed sizes in the background: Size column, sort by size, details.",
			"no":  "Do not measure disk usage; the Size column stays empty.",
		}),
		func(c config.Config) bool { return c.Startup.DiskScan },
		func(c *config.Config, v bool) { c.Startup.DiskScan = v })

	choice(secReleaseAge, "Look up dates for", config.ReleaseAgeModes,
		describeBy("", map[string]string{
			"selected": "Outdated packages, and the package you stop on in the list.",
			"outdated": "Outdated packages only: the versions an upgrade would install.",
			"off":      "Nothing: no release dates and no warnings about recent versions.",
		}),
		func(c config.Config) string { return c.ReleaseAge.Mode },
		func(c *config.Config, v string) { c.ReleaseAge.Mode = v })
	toggle(secReleaseAge, "Casks too",
		describeBy("", map[string]string{
			"yes": "Casks too, from the download's Last-Modified header: a HEAD request to the vendor's server. Not every server sends one.",
			"no":  "Formulae only (bottle dates from GHCR).",
		}),
		func(c config.Config) bool { return c.ReleaseAge.Casks },
		func(c *config.Config, v bool) { c.ReleaseAge.Casks = v })
	number(secReleaseAge, "Recent, hours",
		describe("Versions younger than this show yellow, and installing or updating them warns first. 0 turns the warnings off."),
		func(c config.Config) int { return c.ReleaseAge.RecentHours },
		func(c *config.Config, v int) { c.ReleaseAge.RecentHours = v })
	number(secReleaseAge, "Very fresh, hours",
		describe("Versions younger than this show red. At most the recent hours."),
		func(c config.Config) int { return c.ReleaseAge.FreshHours },
		func(c *config.Config, v int) { c.ReleaseAge.FreshHours = v })
	toggle(secReleaseAge, "Hold recent in Update all",
		describeBy("", map[string]string{
			"yes": "Update all upgrades everything except recent versions.",
			"no":  "Update all upgrades everything, after listing any recent versions.",
		}),
		func(c config.Config) bool { return c.ReleaseAge.HoldInUpdateAll },
		func(c *config.Config, v bool) { c.ReleaseAge.HoldInUpdateAll = v })

	toggle(secActions, "Confirm actions",
		describeBy("", map[string]string{
			"yes": "Ask before install, update, remove and reinstall.",
			"no":  "Run install, update, remove and reinstall at once. Recent versions still ask first.",
		}),
		func(c config.Config) bool { return c.Actions.Confirm },
		func(c *config.Config, v bool) { c.Actions.Confirm = v })
	toggle(secActions, "Greedy Update all",
		describeBy("", map[string]string{
			"yes": "Update all also upgrades casks that update themselves (brew upgrade --greedy).",
			"no":  "Update all skips casks that update themselves, like brew upgrade does.",
		}),
		func(c config.Config) bool { return c.Actions.Greedy },
		func(c *config.Config, v bool) { c.Actions.Greedy = v })
	text(secActions, "Export Brewfile to",
		describe("Where a, e writes the Brewfile of installed packages. ~ is your home directory."),
		func(c config.Config) string { return c.Actions.ExportPath },
		func(c *config.Config, v string) { c.Actions.ExportPath = v })

	choice(secVulns, "Minimum severity", config.Severities,
		func(v string) string {
			if v == "any" {
				return "Report every vulnerability brew vulns finds."
			}
			return fmt.Sprintf("Report only vulnerabilities rated %s or higher.", v)
		},
		func(c config.Config) string { return c.Vulns.MinSeverity },
		func(c *config.Config, v string) { c.Vulns.MinSeverity = v })

	number(secCache, "API lists, hours",
		describe("How long the downloaded formula and cask lists are reused before fetching them again."),
		func(c config.Config) int { return c.Cache.APITTLHours },
		func(c *config.Config, v int) { c.Cache.APITTLHours = v })
	return fields
}

// handleOptionsEvent opens the options panel (o). Save writes the settings
// file and applies what can change live; the start sort and filter apply
// from the next start.
func (s *InputService) handleOptionsEvent() {
	start := config.Get()
	fields := optionFields()
	items := make([]*components.OptionItem, len(fields))
	for i, f := range fields {
		f.item.Value = f.get(start)
		items[i] = f.item
	}
	view := components.NewOptionsView(items)
	view.SetTitle(" Options (Enter edit, ←/→ change, Ctrl+S save, Esc cancel) ")

	save := func() {
		c := start
		for _, f := range fields {
			f.set(&c, f.item.Value)
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
	defaults := func() {
		d := config.Default()
		for _, f := range fields {
			f.item.Value = f.get(d)
		}
	}
	view.SetButtons([]string{"Save", "Defaults", "Cancel"}, func(i int) {
		switch i {
		case 0:
			save()
		case 1:
			defaults()
		case 2:
			s.closeOverlay()
		}
	}, save, s.closeOverlay)

	// The view handles every key itself (Esc, and Q as text while editing).
	s.overlayCapture = func(event *tcell.EventKey) *tcell.EventKey { return event }

	rows := len(fields) + 2*6 // items, section headers and blanks
	centered := components.NewCentered(view, 84, rows+2+4, 1)

	pages := tview.NewPages().
		AddPage("main", s.layout.Root(), true, true).
		AddPage("options", centered, true, true)
	s.appService.GetApp().SetRoot(pages, true)
	s.appService.GetApp().SetFocus(view)
}
