package services

import (
	"fmt"
	"net/url"
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
	items := []menuItem{
		{'g', "Go to top", func() { s.goToTop(prevFocus) }},
		{'h', "Open homepage", s.openHomepage},
		{'f', "Open formulae.brew.sh page", s.openFormulaePage},
	}
	// brew source arrived in Homebrew 7.
	if s.appService.hasBrewSource() {
		items = append(items, menuItem{'s', "Open source repository", s.openSource})
	}
	return append(items, []menuItem{
		{'I', "Open formula/cask on GitHub", func() { s.brewToBrowser("Opening GitHub page of", "info", "--github") }},
		{'i', "Show brew info in the Log", func() { s.brewToLog("info") }},
		{'d', "Filter: dependencies (recursive)", s.showDependencies},
		{'u', "Filter: packages that use this", s.showDependents},
		{'l', "Show brew log in the Log", func() { s.brewToLog("log") }},
		{'L', "Show brew livecheck in the Log", func() { s.brewToLog("livecheck") }},
		{'C', "Show brew cat (source) in the Log", func() { s.brewToLog("cat") }},
	}...)
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

// menuLabel formats a menu entry: the trigger key in the help screen's key
// colour, or "(k)" when colour is disabled.
func (s *InputService) menuLabel(key rune, label string) string {
	if s.appService.noColor {
		return tview.Escape(fmt.Sprintf("(%c) %s", key, label))
	}
	return fmt.Sprintf("[#%06x]%s[-] %s", s.appService.theme.WarningColor.Hex(),
		tview.Escape(string(key)), tview.Escape(label))
}

// newMenuList creates a list for a popup menu whose entries are triggered by
// keys[i]. Pressing a key selects that entry and runs onKey(i); q closes the
// menu; j/k and Ctrl+N/P move. Entries are added by the caller.
func (s *InputService) newMenuList(title string, keys []rune, onKey func(i int)) *tview.List {
	// The default selected style is built from theme colours that are all the
	// terminal default, which makes the current item invisible. Use reverse
	// video across the row, like the package table (works with NO_COLOR too).
	list := tview.NewList().ShowSecondaryText(false).
		SetSelectedStyle(tcell.StyleDefault.Reverse(true)).
		SetHighlightFullLine(true)
	list.SetBorder(true).
		SetTitle(title).
		SetBorderPadding(0, 0, 1, 1)

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune {
			for i, k := range keys {
				if event.Rune() == k {
					list.SetCurrentItem(i)
					onKey(i)
					return nil
				}
			}
		}
		switch {
		case event.Key() == tcell.KeyRune && event.Rune() == 'q':
			s.closeOverlay()
			return nil
		case event.Key() == tcell.KeyRune && event.Rune() == 'j', event.Key() == tcell.KeyCtrlN:
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case event.Key() == tcell.KeyRune && event.Rune() == 'k', event.Key() == tcell.KeyCtrlP:
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		}
		return event
	})
	return list
}

// showOverlay centers a list over the main layout.
func (s *InputService) showOverlay(name string, list *tview.List, width int) {
	centered := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(list, list.GetItemCount()+2, 0, true).
			AddItem(nil, 0, 1, false),
			width, 0, true).
		AddItem(nil, 0, 1, false)

	pages := tview.NewPages().
		AddPage("main", s.layout.Root(), true, true).
		AddPage(name, centered, true, true)
	s.appService.GetApp().SetRoot(pages, true)
}

// showMenu shows a popup menu. Pressing an item's key (or Enter on it)
// closes the menu and runs the item; Esc or q close it.
func (s *InputService) showMenu(title string, items []menuItem) {
	keys := make([]rune, len(items))
	for i, item := range items {
		keys[i] = item.key
	}
	run := func(i int) {
		s.closeOverlay()
		items[i].action()
	}
	list := s.newMenuList(title, keys, run)

	width := tview.TaggedStringWidth(title) + 4
	for i, item := range items {
		text := s.menuLabel(item.key, item.label)
		list.AddItem(text, "", 0, func() { run(i) })
		width = max(width, tview.TaggedStringWidth(text)+6) // border, padding, margin
	}

	s.showOverlay("menu", list, width)
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
	s.openURL(url)
}

// openURL opens a web page with the system opener.
func (s *InputService) openURL(url string) {
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}
	s.runInBackground(fmt.Sprintf("Opening %s", url), exec.Command(opener, url), false) // #nosec G204
}

// formulaePageURL returns the package's page on formulae.brew.sh, or "" when
// it is not there (only the official homebrew/core and homebrew/cask taps
// are listed).
func formulaePageURL(pkg *models.Package) string {
	switch {
	case pkg.Formula != nil && (pkg.Formula.Tap == "" || pkg.Formula.Tap == "homebrew/core"):
		return "https://formulae.brew.sh/formula/" + url.PathEscape(pkg.Name)
	case pkg.Cask != nil && (pkg.Cask.Tap == "" || pkg.Cask.Tap == "homebrew/cask"):
		return "https://formulae.brew.sh/cask/" + url.PathEscape(pkg.Name)
	}
	return ""
}

// openFormulaePage opens the selected package's page on formulae.brew.sh.
func (s *InputService) openFormulaePage() {
	pkg := s.selectedPackage()
	if pkg == nil {
		return
	}
	page := formulaePageURL(pkg)
	if page == "" {
		s.layout.GetNotifier().ShowWarning(fmt.Sprintf("%s is not listed on formulae.brew.sh", pkg.Label()))
		return
	}
	s.openURL(page)
}

// openSource opens the selected formula's source repository with
// `brew source` (formulae only). The menu offers it on Homebrew 7+ only.
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

// selectedBrewPackage returns the selected package if brew commands apply to
// it, warning otherwise.
func (s *InputService) selectedBrewPackage(what string) *models.Package {
	pkg := s.selectedPackage()
	if pkg == nil {
		return nil
	}
	if !isBrewPackage(pkg) {
		s.layout.GetNotifier().ShowWarning(what + " only works for Homebrew packages")
		return nil
	}
	return pkg
}

// brewToLog runs `brew <command> --formula|--cask <name>` into the Log.
func (s *InputService) brewToLog(command string) {
	pkg := s.selectedBrewPackage("brew " + command)
	if pkg == nil {
		return
	}
	s.runInBackground(fmt.Sprintf("brew %s %s", command, pkg.Name),
		brewCommand(command, brewTypeFlag(pkg), pkg.Name), true) // #nosec G204
}

// brewToBrowser runs a brew command that opens a browser page.
func (s *InputService) brewToBrowser(what string, args ...string) {
	pkg := s.selectedBrewPackage("brew " + args[0])
	if pkg == nil {
		return
	}
	args = append(args, brewTypeFlag(pkg), pkg.Name)
	s.runInBackground(fmt.Sprintf("%s %s", what, pkg.Label()), brewCommand(args...), false) // #nosec G204
}

// showDependencies filters the list to the selected package's dependencies,
// recursively (like brew deps).
func (s *InputService) showDependencies() {
	pkg := s.selectedPackage()
	if pkg == nil {
		return
	}
	members := s.appService.dependencyIndex().dependencies(pkg.Name)
	s.setRelationFilter(members, fmt.Sprintf("Dependencies of %s", pkg.Name),
		fmt.Sprintf("%s has no dependencies", pkg.Name))
}

// showDependents filters the list to the packages that directly depend on
// the selected package (like brew uses).
func (s *InputService) showDependents() {
	pkg := s.selectedPackage()
	if pkg == nil {
		return
	}
	members := s.appService.dependencyIndex().dependents(pkg.Name)
	s.setRelationFilter(members, fmt.Sprintf("Used by %s", pkg.Name),
		fmt.Sprintf("Nothing depends on %s", pkg.Name))
}

func (s *InputService) setRelationFilter(members map[string]bool, title, emptyMessage string) {
	if len(members) == 0 {
		s.layout.GetNotifier().ShowWarning(emptyMessage)
		return
	}
	s.appService.relation = &relationFilter{title: title, members: members}
	s.applyFilter()
	s.layout.GetNotifier().ShowInfo(title + " (Esc to clear)")
}

// clearRelationFilter removes a dependency filter. It reports whether one
// was set.
func (s *InputService) clearRelationFilter() bool {
	if s.appService.relation == nil {
		return false
	}
	s.appService.relation = nil
	s.applyFilter()
	return true
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
