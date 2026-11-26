package services

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"bbrew/internal/models"
	"bbrew/internal/ui/components"
)

// menuItem is one entry of a popup menu.
type menuItem struct {
	key    rune
	label  string
	action func()
}

// goMenuItems returns the entries of the g menu. They act on the selected
// package (except "go to top").
func (s *InputService) goMenuItems(prevFocus tview.Primitive) []menuItem {
	return []menuItem{
		{'g', "Go to top", func() { s.goToTop(prevFocus) }},
		{'h', "Open homepage", s.openHomepage},
		{'s', "Open source repository", s.openSource},
		{'i', "Show brew info in the Log", s.showBrewInfo},
	}
}

// goMenuHelp lists the g menu for the help screen.
func (s *InputService) goMenuHelp() components.HelpSection {
	section := components.HelpSection{Title: "GO MENU (g, then...)"}
	for _, item := range s.goMenuItems(nil) {
		section.Keys = append(section.Keys, components.HelpKey{Keys: "g" + string(item.key), Description: item.label})
	}
	return section
}

// handleGoMenuEvent opens the g menu.
func (s *InputService) handleGoMenuEvent() {
	prevFocus := s.appService.GetApp().GetFocus()
	s.showMenu(" Go ", s.goMenuItems(prevFocus))
}

// showMenu shows a popup menu. Pressing an item's key (or Enter on it)
// closes the menu and runs the item; Esc or q close it.
func (s *InputService) showMenu(title string, items []menuItem) {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true).
		SetTitle(title).
		SetBorderPadding(0, 0, 1, 1)

	width := tview.TaggedStringWidth(title) + 4
	for _, item := range items {
		action := item.action
		list.AddItem(item.label, "", item.key, func() {
			s.closeOverlay()
			action()
		})
		width = max(width, tview.TaggedStringWidth(item.label)+10)
	}

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyRune && event.Rune() == 'q':
			s.closeOverlay()
			return nil
		case event.Key() == tcell.KeyCtrlN:
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case event.Key() == tcell.KeyCtrlP:
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		}
		return event
	})

	centered := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(list, len(items)+2, 0, true).
			AddItem(nil, 0, 1, false),
			width, 0, true).
		AddItem(nil, 0, 1, false)

	pages := tview.NewPages().
		AddPage("main", s.layout.Root(), true, true).
		AddPage("menu", centered, true, true)
	s.appService.GetApp().SetRoot(pages, true)
}

// selectedPackage returns the package selected in the table, or nil.
func (s *InputService) selectedPackage() *models.Package {
	row, _ := s.layout.GetTable().View().GetSelection()
	if row <= 0 || row-1 >= len(*s.appService.filteredPackages) {
		return nil
	}
	pkg := (*s.appService.filteredPackages)[row-1]
	return &pkg
}

// goToTop scrolls the panel that had focus before the menu to its top.
func (s *InputService) goToTop(focus tview.Primitive) {
	switch p := focus.(type) {
	case *tview.Table:
		if p.GetRowCount() > 1 {
			p.Select(1, 0)
		}
		p.ScrollToBeginning()
	case *tview.TextView:
		p.ScrollToBeginning()
	}
	if focus != nil {
		s.appService.GetApp().SetFocus(focus)
	}
}

// brewTypeFlag returns the --formula/--cask flag for a brew package.
func brewTypeFlag(pkg *models.Package) string {
	if pkg.Type == models.PackageTypeCask {
		return "--cask"
	}
	return "--formula"
}

// isBrewPackage reports whether brew commands apply to the package.
func isBrewPackage(pkg *models.Package) bool {
	return pkg.Type == models.PackageTypeFormula || pkg.Type == models.PackageTypeCask
}

// openHomepage opens the selected package's homepage in a browser, with
// `brew home` for Homebrew packages and the system opener otherwise.
func (s *InputService) openHomepage() {
	pkg := s.selectedPackage()
	if pkg == nil {
		return
	}
	if isBrewPackage(pkg) {
		s.runInBackground(fmt.Sprintf("Opening homepage of %s", pkg.Label()),
			brewCommand("home", brewTypeFlag(pkg), pkg.Name), false) // #nosec G204
		return
	}

	url := pkg.Homepage
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		s.layout.GetNotifier().ShowWarning(fmt.Sprintf("No homepage for %s", pkg.Label()))
		return
	}
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}
	s.runInBackground(fmt.Sprintf("Opening %s", url), exec.Command(opener, url), false) // #nosec G204
}

// openSource opens the selected formula's source repository with
// `brew source` (Homebrew 7+, formulae only).
func (s *InputService) openSource() {
	pkg := s.selectedPackage()
	if pkg == nil {
		return
	}
	if pkg.Type != models.PackageTypeFormula {
		s.layout.GetNotifier().ShowWarning("brew source only works for formulae")
		return
	}
	s.runInBackground(fmt.Sprintf("Opening source of %s", pkg.Label()),
		brewCommand("source", pkg.Name), false) // #nosec G204
}

// showBrewInfo runs `brew info` for the selected package into the Log.
func (s *InputService) showBrewInfo() {
	pkg := s.selectedPackage()
	if pkg == nil {
		return
	}
	if !isBrewPackage(pkg) {
		s.layout.GetNotifier().ShowWarning("brew info only works for Homebrew packages")
		return
	}
	s.runInBackground(fmt.Sprintf("brew info %s", pkg.Name),
		brewCommand("info", brewTypeFlag(pkg), pkg.Name), true) // #nosec G204
}

// runInBackground runs a command, streaming its output to the Log, and
// reports failure in the status line. clearLog empties the Log first.
func (s *InputService) runInBackground(what string, cmd *exec.Cmd, clearLog bool) {
	if clearLog {
		s.layout.GetOutput().Clear()
	}
	s.layout.GetNotifier().ShowInfo(what + "...")
	go func() {
		err := ExecuteCommand(cmd, s.outputWriter())
		if err == nil {
			return
		}
		s.appService.app.QueueUpdateDraw(func() {
			s.layout.GetNotifier().ShowError(fmt.Sprintf("%s failed: %v", what, err))
		})
	}()
}
