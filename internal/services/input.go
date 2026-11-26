package services

import (
	"fmt"
	"io"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"bbrew/internal/models"
	"bbrew/internal/ui"
	"bbrew/internal/ui/components"
)

// Help screen sections, in display order.
const (
	sectionFilters  = "FILTERS"
	sectionActions  = "ACTIONS"
	sectionBrewfile = "BREWFILE"
	sectionView     = "VIEW"
)

// InputAction represents a user action that can be triggered by a key event.
type InputAction struct {
	Key     tcell.Key
	Rune    rune
	Name    string
	KeySlug string // how the key is shown in the help screen
	Section string // help screen section
	Action  func()
}

// matches reports whether the event triggers this action.
func (a *InputAction) matches(event *tcell.EventKey) bool {
	if a.Key == tcell.KeyRune {
		// Shift is implied by the rune itself; Alt/Ctrl make it a different key.
		return event.Key() == tcell.KeyRune && event.Rune() == a.Rune &&
			event.Modifiers()&(tcell.ModAlt|tcell.ModCtrl) == 0
	}
	return event.Key() == a.Key
}

// InputServiceInterface defines the interface for handling user input actions.
type InputServiceInterface interface {
	HandleKeyEventInput(event *tcell.EventKey) *tcell.EventKey
	EnableBrewfileMode()
}

// InputService implements the InputServiceInterface and handles key events for the application.
type InputService struct {
	appService     *AppService
	layout         ui.LayoutInterface
	brewService    BrewServiceInterface
	flatpakService FlatpakServiceInterface
	keyActions     []*InputAction

	// Actions for each key input
	ActionSearch        *InputAction
	ActionFilterPicker  *InputAction
	ActionSort          *InputAction
	ActionExport        *InputAction
	ActionVulnScan      *InputAction
	ActionInstall       *InputAction
	ActionUpdate        *InputAction
	ActionRemove        *InputAction
	ActionUpdateAll     *InputAction
	ActionInstallAll    *InputAction
	ActionRemoveAll     *InputAction
	ActionToggleBorders *InputAction
	ActionToggleDetails *InputAction
	ActionNextPanel     *InputAction
	ActionPrevPanel     *InputAction
	ActionShowDetails   *InputAction
	ActionHelp          *InputAction
	ActionBack          *InputAction
	ActionQuit          *InputAction
}

var NewInputService = func(appService *AppService, brewService BrewServiceInterface, flatpakService FlatpakServiceInterface) InputServiceInterface {
	s := &InputService{
		appService:     appService,
		layout:         appService.GetLayout(),
		brewService:    brewService,
		flatpakService: flatpakService,
	}

	// Initialize actions with key bindings and handlers
	s.ActionSearch = &InputAction{
		Key: tcell.KeyRune, Rune: '/', KeySlug: "/", Name: "Focus search",
		Action: s.handleSearchFieldEvent,
	}
	s.ActionFilterPicker = &InputAction{
		Key: tcell.KeyRune, Rune: 'f', KeySlug: "f", Name: "Filter picker", Section: sectionFilters,
		Action: s.handleFilterPickerEvent,
	}
	s.ActionSort = &InputAction{
		Key: tcell.KeyRune, Rune: 's', KeySlug: "s", Name: "Cycle sort (none, downloads, name)", Section: sectionFilters,
		Action: s.handleSortEvent,
	}
	s.ActionInstall = &InputAction{
		Key: tcell.KeyRune, Rune: 'i', KeySlug: "i", Name: "Install selected", Section: sectionActions,
		Action: s.handleInstallPackageEvent,
	}
	s.ActionUpdate = &InputAction{
		Key: tcell.KeyRune, Rune: 'u', KeySlug: "u", Name: "Update selected", Section: sectionActions,
		Action: s.handleUpdatePackageEvent,
	}
	s.ActionRemove = &InputAction{
		Key: tcell.KeyRune, Rune: 'r', KeySlug: "r", Name: "Remove selected", Section: sectionActions,
		Action: s.handleRemovePackageEvent,
	}
	s.ActionUpdateAll = &InputAction{
		Key: tcell.KeyRune, Rune: 'U', KeySlug: "U", Name: "Update all outdated", Section: sectionActions,
		Action: s.handleUpdateAllPackagesEvent,
	}
	s.ActionVulnScan = &InputAction{
		Key: tcell.KeyRune, Rune: 'v', KeySlug: "v", Name: "Vulnerability scan", Section: sectionActions,
		Action: s.handleVulnScanEvent,
	}
	s.ActionExport = &InputAction{
		Key: tcell.KeyRune, Rune: 'e', KeySlug: "e", Name: "Export to ~/Brewfile", Section: sectionActions,
		Action: s.handleExportEvent,
	}
	s.ActionInstallAll = &InputAction{
		Key: tcell.KeyCtrlA, KeySlug: "Ctrl+A", Name: "Install all from Brewfile", Section: sectionBrewfile,
		Action: s.handleInstallAllPackagesEvent,
	}
	s.ActionRemoveAll = &InputAction{
		Key: tcell.KeyCtrlR, KeySlug: "Ctrl+R", Name: "Remove all from Brewfile", Section: sectionBrewfile,
		Action: s.handleRemoveAllPackagesEvent,
	}
	s.ActionToggleBorders = &InputAction{
		Key: tcell.KeyRune, Rune: 'b', KeySlug: "b", Name: "Toggle borders", Section: sectionView,
		Action: s.layout.ToggleBorders,
	}
	s.ActionToggleDetails = &InputAction{
		Key: tcell.KeyRune, Rune: 'd', KeySlug: "d", Name: "Toggle details panel", Section: sectionView,
		Action: s.layout.ToggleDetails,
	}
	s.ActionNextPanel = &InputAction{
		Key: tcell.KeyTab, Action: func() { s.cyclePanel(1) },
	}
	s.ActionPrevPanel = &InputAction{
		Key: tcell.KeyBacktab, Action: func() { s.cyclePanel(-1) },
	}
	s.ActionShowDetails = &InputAction{
		Key: tcell.KeyEnter, Action: s.handleShowDetails,
	}
	s.ActionHelp = &InputAction{
		Key: tcell.KeyRune, Rune: '?', Action: s.handleHelpEvent,
	}
	s.ActionBack = &InputAction{
		Key: tcell.KeyEsc, Action: s.handleBack,
	}
	s.ActionQuit = &InputAction{
		Key: tcell.KeyRune, Rune: 'q', Action: s.handleQuitEvent,
	}

	// InstallAll/RemoveAll are added in Brewfile mode
	s.keyActions = []*InputAction{
		s.ActionSearch, s.ActionFilterPicker, s.ActionSort, s.ActionInstall, s.ActionUpdate, s.ActionRemove,
		s.ActionUpdateAll, s.ActionVulnScan, s.ActionExport,
		s.ActionToggleBorders, s.ActionToggleDetails, s.ActionNextPanel, s.ActionPrevPanel,
		s.ActionShowDetails, s.ActionHelp, s.ActionBack, s.ActionQuit,
	}

	return s
}

// outputWriter returns a thread-safe writer that streams to the output panel.
func (s *InputService) outputWriter() io.Writer {
	return ui.NewThreadSafeWriter(s.appService.app, s.layout.GetOutput().View())
}

// EnableBrewfileMode enables Brewfile mode, adding Install All and Remove All actions
func (s *InputService) EnableBrewfileMode() {
	s.keyActions = append(s.keyActions, s.ActionInstallAll, s.ActionRemoveAll)
}

// panels returns the focusable main panels in Tab order.
func (s *InputService) panels() []tview.Primitive {
	panels := []tview.Primitive{s.layout.GetTable().View()}
	if s.layout.DetailsVisible() {
		panels = append(panels, s.layout.GetDetails().View())
	}
	return append(panels, s.layout.GetOutput().View())
}

// focusedPanel returns the index of the focused main panel, or -1 when focus
// is elsewhere (search field, modal, help, picker).
func (s *InputService) focusedPanel() int {
	focus := s.appService.GetApp().GetFocus()
	for i, p := range s.panels() {
		if p == focus {
			return i
		}
	}
	return -1
}

// cyclePanel moves focus to the next (dir=1) or previous (dir=-1) panel.
func (s *InputService) cyclePanel(dir int) {
	panels := s.panels()
	i := max(s.focusedPanel(), 0)
	next := (i + dir + len(panels)) % len(panels)
	s.appService.GetApp().SetFocus(panels[next])
}

// HandleKeyEventInput processes key events and triggers the corresponding actions.
func (s *InputService) HandleKeyEventInput(event *tcell.EventKey) *tcell.EventKey {
	// The details panel may have been hidden (narrow screen) while focused.
	if !s.layout.DetailsVisible() && s.layout.GetDetails().View().HasFocus() {
		s.appService.GetApp().SetFocus(s.layout.GetTable().View())
	}

	panel := s.focusedPanel()
	if panel < 0 {
		// The search field, modals, help and pickers handle their own keys;
		// Esc closes overlays.
		if event.Key() == tcell.KeyEsc && !s.layout.GetSearch().Field().HasFocus() {
			s.closeOverlay()
			return nil
		}
		return event
	}

	if translated, handled := s.handleNavigation(event, s.panels()[panel]); handled {
		return translated
	}

	for _, input := range s.keyActions {
		if input.matches(event) && input.Action != nil {
			input.Action()
			return nil
		}
	}

	return event
}

// handleNavigation implements vim and emacs style movement on top of what
// tview provides (arrows, j/k, g/G, Home/End, PgUp/PgDn, Ctrl+F/B). It returns
// the event to pass on and whether it handled the key.
func (s *InputService) handleNavigation(event *tcell.EventKey, focus tview.Primitive) (*tcell.EventKey, bool) {
	key := func(k tcell.Key) (*tcell.EventKey, bool) {
		return tcell.NewEventKey(k, 0, tcell.ModNone), true
	}

	switch event.Key() {
	case tcell.KeyCtrlN:
		return key(tcell.KeyDown)
	case tcell.KeyCtrlP:
		return key(tcell.KeyUp)
	case tcell.KeyCtrlV:
		return key(tcell.KeyPgDn)
	case tcell.KeyCtrlD:
		s.scrollHalfPage(focus, 1)
		return nil, true
	case tcell.KeyCtrlU:
		s.scrollHalfPage(focus, -1)
		return nil, true
	case tcell.KeyRune:
		if event.Modifiers()&tcell.ModAlt != 0 {
			switch event.Rune() {
			case '<':
				return key(tcell.KeyHome)
			case '>':
				return key(tcell.KeyEnd)
			case 'v':
				return key(tcell.KeyPgUp)
			}
		}
	}
	return event, false
}

// scrollHalfPage moves the table selection or scrolls a text panel by half
// its height.
func (s *InputService) scrollHalfPage(focus tview.Primitive, dir int) {
	switch p := focus.(type) {
	case *tview.Table:
		_, _, _, height := p.GetInnerRect()
		step := max((height-1)/2, 1) // minus the fixed header row
		row, _ := p.GetSelection()
		last := p.GetRowCount() - 1
		if last < 1 {
			return
		}
		p.Select(min(max(row+dir*step, 1), last), 0)
	case *tview.TextView:
		_, _, _, height := p.GetInnerRect()
		step := max(height/2, 1)
		row, col := p.GetScrollOffset()
		p.ScrollTo(max(row+dir*step, 0), col)
	}
}

// handleShowDetails focuses the details panel, or shows the details in a
// popup when the panel is hidden on a narrow screen.
func (s *InputService) handleShowDetails() {
	if s.focusedPanel() != 0 {
		return // only from the table
	}
	if s.layout.DetailsVisible() {
		s.appService.GetApp().SetFocus(s.layout.GetDetails().View())
		return
	}

	row, _ := s.layout.GetTable().View().GetSelection()
	if row <= 0 || row-1 >= len(*s.appService.filteredPackages) {
		return
	}

	text := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetText(s.layout.GetDetails().View().GetText(false))
	text.SetBorder(true).
		SetBorderPadding(0, 0, 1, 1).
		SetTitle(" Details ").
		SetTitleAlign(tview.AlignLeft)

	// Enter, q or Esc close; everything else (j/k, g/G, arrows) scrolls.
	text.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyEnter, event.Key() == tcell.KeyRune && event.Rune() == 'q':
			s.closeOverlay()
			return nil
		case event.Key() == tcell.KeyCtrlN:
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case event.Key() == tcell.KeyCtrlP:
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		}
		return event
	})

	popup := tview.NewFlex().
		AddItem(nil, 1, 0, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 1, 0, false).
			AddItem(text, 0, 1, true).
			AddItem(nil, 1, 0, false),
			0, 1, true).
		AddItem(nil, 1, 0, false)

	pages := tview.NewPages().
		AddPage("main", s.layout.Root(), true, true).
		AddPage("details", popup, true, true)
	s.appService.GetApp().SetRoot(pages, true)
}

// closeOverlay returns from a modal, help or picker to the main layout.
func (s *InputService) closeOverlay() {
	s.appService.GetApp().SetRoot(s.layout.Root(), true)
	s.appService.GetApp().SetFocus(s.layout.GetTable().View())
}

// handleBack is called on Esc: from a side panel go back to the table, from
// the table quit.
func (s *InputService) handleBack() {
	if s.focusedPanel() > 0 {
		s.appService.GetApp().SetFocus(s.layout.GetTable().View())
		return
	}
	s.handleQuitEvent()
}

// handleSearchFieldEvent is called when the user presses the search key (/).
func (s *InputService) handleSearchFieldEvent() {
	s.appService.GetApp().SetFocus(s.layout.GetSearch().Field())
}

// handleQuitEvent is called when the user presses the quit key (q).
func (s *InputService) handleQuitEvent() {
	s.appService.GetApp().Stop()
}

// helpSections builds the help screen content from the key bindings.
func (s *InputService) helpSections() []components.HelpSection {
	sections := []components.HelpSection{
		{Title: "NAVIGATION", Keys: []components.HelpKey{
			{Keys: "↑/↓, j/k, Ctrl+N/P", Description: "Move down/up"},
			{Keys: "g/G, Home/End", Description: "Go to top/bottom"},
			{Keys: "Alt+< / Alt+>", Description: "Go to top/bottom"},
			{Keys: "←/→, h/l", Description: "Scroll columns (Type and Name stay)"},
			{Keys: "Ctrl+D/U", Description: "Half page down/up"},
			{Keys: "Ctrl+F/B, PgDn/PgUp", Description: "Page down/up"},
			{Keys: "Ctrl+V / Alt+V", Description: "Page down/up"},
			{Keys: "Tab / Shift+Tab", Description: "Next/previous panel"},
			{Keys: "Enter", Description: "Show details (popup when hidden)"},
			{Keys: "/", Description: "Focus search"},
			{Keys: "Esc", Description: "Back to table, or quit"},
			{Keys: "q", Description: "Quit"},
			{Keys: "?", Description: "This help"},
		}},
	}

	for _, title := range []string{sectionFilters, sectionActions, sectionBrewfile, sectionView} {
		section := components.HelpSection{Title: title}
		for _, a := range s.keyActions {
			if a.Section == title {
				section.Keys = append(section.Keys, components.HelpKey{Keys: a.KeySlug, Description: a.Name})
			}
		}
		if len(section.Keys) > 0 {
			sections = append(sections, section)
		}
	}
	return sections
}

// handleHelpEvent shows the help screen with all keyboard shortcuts.
func (s *InputService) handleHelpEvent() {
	helpPages, text := s.layout.GetHelpScreen().Build(s.layout.Root(), s.helpSections(), s.layout.Height()-2)

	// Scroll keys scroll the help; any other key closes it.
	text.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp, tcell.KeyDown, tcell.KeyPgUp, tcell.KeyPgDn, tcell.KeyHome, tcell.KeyEnd,
			tcell.KeyCtrlF, tcell.KeyCtrlB:
			return event
		case tcell.KeyCtrlN:
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case tcell.KeyCtrlP:
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		case tcell.KeyCtrlD:
			return tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone)
		case tcell.KeyCtrlU:
			return tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone)
		case tcell.KeyRune:
			switch event.Rune() {
			case 'j', 'k', 'g', 'G':
				return event
			}
		}
		s.closeOverlay()
		return nil
	})

	s.appService.GetApp().SetRoot(helpPages, true)
}

// applyFilter refreshes the search label and results after a filter change.
func (s *InputService) applyFilter() {
	s.updateFilterUI()
	s.appService.search(s.layout.GetSearch().Field().GetText(), true)
}

// updateFilterUI updates the search label based on the current filter state.
func (s *InputService) updateFilterUI() {
	label := s.appService.activeFilter.Label()
	switch {
	case s.appService.IsBrewfileMode() && label != "":
		label = "Brewfile - " + label
	case s.appService.IsBrewfileMode():
		label = "Brewfile"
	case label == "":
		label = "All"
	}
	s.layout.GetSearch().Field().SetLabel(tview.Escape("Search (" + label + "): "))
	s.layout.SetPackagesTitle(s.appService.activeFilter.Title())
}

// handleFilterPickerEvent shows a picker to combine a status filter
// (installed, outdated, leaves) with a kind filter (formulae, casks).
func (s *InputService) handleFilterPickerEvent() {
	type option struct {
		label    string
		shortcut rune
		active   func() bool
		toggle   func()
	}
	filter := &s.appService.activeFilter
	options := []option{
		{"None", 'n', filter.IsNone, func() { *filter = PackageFilter{} }},
		{"Installed", 'i', func() bool { return filter.Status == StatusInstalled }, func() { filter.ToggleStatus(StatusInstalled) }},
		{"Outdated", 'o', func() bool { return filter.Status == StatusOutdated }, func() { filter.ToggleStatus(StatusOutdated) }},
		{"Leaves", 'l', func() bool { return filter.Status == StatusLeaves }, func() { filter.ToggleStatus(StatusLeaves) }},
		{"Formulae", 'f', func() bool { return filter.Kind == KindFormulae }, func() { filter.ToggleKind(KindFormulae) }},
		{"Casks", 'c', func() bool { return filter.Kind == KindCasks }, func() { filter.ToggleKind(KindCasks) }},
	}

	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true).
		SetTitle(" Filter (Esc to close) ").
		SetBorderPadding(0, 0, 1, 1)

	refresh := func() {
		for i, o := range options {
			mark := "○"
			if o.active() {
				mark = "●"
			}
			list.SetItemText(i, mark+" "+o.label, "")
		}
	}
	for _, o := range options {
		toggle := o.toggle
		list.AddItem(o.label, "", o.shortcut, func() {
			toggle()
			refresh()
			s.applyFilter()
		})
	}
	refresh()

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
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

	const width = 28
	centered := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(list, len(options)+2, 0, true).
			AddItem(nil, 0, 1, false),
			width, 0, true).
		AddItem(nil, 0, 1, false)

	pages := tview.NewPages().
		AddPage("main", s.layout.Root(), true, true).
		AddPage("filter", centered, true, true)
	s.appService.GetApp().SetRoot(pages, true)
}

// handleSortEvent cycles through sort modes (None → Downloads → Name).
func (s *InputService) handleSortEvent() {
	newSort := s.appService.CycleSortMode()
	s.layout.GetNotifier().ShowSuccess(fmt.Sprintf("Sort: %s", newSort))
}

// handleExportEvent exports installed packages to ~/Brewfile.
func (s *InputService) handleExportEvent() {
	path, err := s.appService.ExportBrewfile()
	if err != nil {
		s.layout.GetNotifier().ShowError(fmt.Sprintf("Export failed: %v", err))
		return
	}
	s.layout.GetNotifier().ShowSuccess(fmt.Sprintf("Exported to %s", path))
}

// handleVulnScanEvent scans the selected package for known vulnerabilities using brew vulns.
func (s *InputService) handleVulnScanEvent() {
	if !s.appService.vulnsService.IsAvailable() {
		s.handleVulnInstallPrompt()
		return
	}

	row, _ := s.layout.GetTable().View().GetSelection()
	if row <= 0 || row-1 >= len(*s.appService.filteredPackages) {
		return
	}

	info := (*s.appService.filteredPackages)[row-1]
	if info.Type != models.PackageTypeFormula && info.Type != models.PackageTypeCask {
		s.layout.GetNotifier().ShowWarning("Vulnerability scan only available for Homebrew packages")
		return
	}

	s.layout.GetOutput().Clear()
	go func() {
		s.appService.app.QueueUpdateDraw(func() {
			s.layout.GetNotifier().ShowWarning(fmt.Sprintf("Scanning %s for vulnerabilities...", info.Name))
		})

		vulns, err := s.appService.vulnsService.ScanPackage(info.Name, s.outputWriter())

		s.appService.app.QueueUpdateDraw(func() {
			if err != nil {
				s.layout.GetNotifier().ShowError(fmt.Sprintf("Vuln scan failed: %v", err))
				return
			}

			if len(vulns) == 0 {
				s.layout.GetNotifier().ShowSuccess(fmt.Sprintf("%s: no vulnerabilities found", info.Name))
			} else {
				s.layout.GetNotifier().ShowError(fmt.Sprintf("%s: %d vulnerabilit%s found", info.Name, len(vulns), pluralY(len(vulns))))
			}

			// Refresh details panel with newly cached vulnerability data
			cachedVulns, _ := s.appService.vulnsService.GetCachedVulns(info.Name)
			s.layout.GetDetails().SetContent(&info, cachedVulns)
		})
	}()
}

// handleVulnInstallPrompt asks the user to install brew vulns when it's not available.
func (s *InputService) handleVulnInstallPrompt() {
	s.showModal(
		"brew vulns is not installed.\n\nInstall it now to enable vulnerability scanning?",
		func() {
			s.closeModal()
			s.layout.GetOutput().Clear()
			go func() {
				s.appService.app.QueueUpdateDraw(func() {
					s.layout.GetNotifier().ShowWarning("Installing brew vulns...")
				})
				cmd := brewCommand("install", "homebrew/brew-vulns/brew-vulns") // #nosec G204
				if err := ExecuteCommand(cmd, s.outputWriter()); err != nil {
					s.appService.app.QueueUpdateDraw(func() {
						s.layout.GetNotifier().ShowError("Failed to install brew vulns")
					})
					return
				}
				s.appService.vulnsService.(*VulnsService).resetAvailability()
				s.appService.app.QueueUpdateDraw(func() {
					s.layout.GetNotifier().ShowSuccess("brew vulns installed! Press v again to scan.")
				})
			}()
		},
		s.closeModal,
	)
}

// showModal displays a modal dialog with the specified text and confirmation/cancellation actions.
// This is used for actions like installing, removing, or updating packages, invoking user confirmation.
func (s *InputService) showModal(text string, confirmFunc func(), cancelFunc func()) {
	modal := s.layout.GetModal().Build(text, confirmFunc, cancelFunc)
	s.appService.app.SetRoot(modal, true)
}

// closeModal closes the currently displayed modal dialog and returns focus to the main table view.
func (s *InputService) closeModal() {
	s.closeOverlay()
}

// handleInstallPackageEvent is called when the user presses the installation key (i).
func (s *InputService) handleInstallPackageEvent() {
	row, _ := s.layout.GetTable().View().GetSelection()
	if row > 0 && row-1 < len(*s.appService.filteredPackages) {
		info := (*s.appService.filteredPackages)[row-1]
		s.showModal(
			fmt.Sprintf("Are you sure you want to install the package: %s?", info.Label()),
			func() {
				s.closeModal()
				s.layout.GetOutput().Clear()
				go func() {
					s.appService.app.QueueUpdateDraw(func() {
						s.layout.GetNotifier().ShowWarning(fmt.Sprintf("Installing %s...", info.Label()))
					})
					var err error
					switch info.Type {
					case models.PackageTypeFlatpak:
						err = s.flatpakService.InstallPackage(info, s.outputWriter())
					case models.PackageTypeMas:
						err = s.appService.masService.InstallApp(info, s.outputWriter())
					default:
						err = s.brewService.InstallPackage(info, s.outputWriter())
					}

					s.appService.app.QueueUpdateDraw(func() {
						if err != nil {
							s.layout.GetNotifier().ShowError(fmt.Sprintf("Failed to install %s", info.Label()))
							return
						}
						s.layout.GetNotifier().ShowSuccess(fmt.Sprintf("Installed %s", info.Label()))
					})
					if err == nil {
						s.appService.forceRefreshResults()
					}
				}()
			}, s.closeModal)
	}
}

// handleRemovePackageEvent is called when the user presses the removal key (r).
func (s *InputService) handleRemovePackageEvent() {
	row, _ := s.layout.GetTable().View().GetSelection()
	if row > 0 && row-1 < len(*s.appService.filteredPackages) {
		info := (*s.appService.filteredPackages)[row-1]
		s.showModal(
			fmt.Sprintf("Are you sure you want to remove the package: %s?", info.Label()),
			func() {
				s.closeModal()
				s.layout.GetOutput().Clear()
				go func() {
					s.appService.app.QueueUpdateDraw(func() {
						s.layout.GetNotifier().ShowWarning(fmt.Sprintf("Removing %s...", info.Label()))
					})
					var err error
					switch info.Type {
					case models.PackageTypeFlatpak:
						err = s.flatpakService.RemovePackage(info, s.outputWriter())
					case models.PackageTypeMas:
						err = s.appService.masService.RemoveApp(info, s.outputWriter())
					default:
						err = s.brewService.RemovePackage(info, s.outputWriter())
					}

					s.appService.app.QueueUpdateDraw(func() {
						if err != nil {
							s.layout.GetNotifier().ShowError(fmt.Sprintf("Failed to remove %s: may need sudo in terminal", info.Label()))
							return
						}
						s.layout.GetNotifier().ShowSuccess(fmt.Sprintf("Removed %s", info.Label()))
					})
					if err == nil {
						s.appService.forceRefreshResults()
					}
				}()
			}, s.closeModal)
	}
}

// handleUpdatePackageEvent is called when the user presses the update key (u).
func (s *InputService) handleUpdatePackageEvent() {
	row, _ := s.layout.GetTable().View().GetSelection()
	if row > 0 && row-1 < len(*s.appService.filteredPackages) {
		info := (*s.appService.filteredPackages)[row-1]
		s.showModal(
			fmt.Sprintf("Are you sure you want to update the package: %s?", info.Label()),
			func() {
				s.closeModal()
				s.layout.GetOutput().Clear()
				go func() {
					s.appService.app.QueueUpdateDraw(func() {
						s.layout.GetNotifier().ShowWarning(fmt.Sprintf("Updating %s...", info.Label()))
					})
					var err error
					switch info.Type {
					case models.PackageTypeFlatpak:
						err = s.flatpakService.UpdatePackage(info, s.outputWriter())
					case models.PackageTypeMas:
						// MAS apps are updated through the App Store; no CLI update supported
						err = nil
					default:
						err = s.brewService.UpdatePackage(info, s.outputWriter())
					}

					s.appService.app.QueueUpdateDraw(func() {
						if err != nil {
							s.layout.GetNotifier().ShowError(fmt.Sprintf("Failed to update %s", info.Label()))
							return
						}
						s.layout.GetNotifier().ShowSuccess(fmt.Sprintf("Updated %s", info.Label()))
					})
					if err == nil {
						s.appService.forceRefreshResults()
					}
				}()
			}, s.closeModal)
	}
}

// handleUpdateAllPackagesEvent is called when the user presses the update all key (U).
func (s *InputService) handleUpdateAllPackagesEvent() {
	s.showModal("Are you sure you want to update all Packages?", func() {
		s.closeModal()
		s.layout.GetOutput().Clear()
		go func() {
			s.appService.app.QueueUpdateDraw(func() {
				s.layout.GetNotifier().ShowWarning("Updating all Homebrew packages...")
			})
			if err := s.brewService.UpdateAllPackages(s.outputWriter()); err != nil {
				s.appService.app.QueueUpdateDraw(func() {
					s.layout.GetNotifier().ShowError("Failed to update Homebrew packages")
				})
				return
			}

			if s.flatpakService.IsFlatpakInstalled() {
				s.appService.app.QueueUpdateDraw(func() {
					s.layout.GetNotifier().ShowWarning("Updating all Flatpak packages...")
				})
				if err := s.flatpakService.UpdateAllPackages(s.outputWriter()); err != nil {
					s.appService.app.QueueUpdateDraw(func() {
						s.layout.GetNotifier().ShowError("Failed to update Flatpak packages")
					})
					return
				}
			}

			s.appService.app.QueueUpdateDraw(func() {
				s.layout.GetNotifier().ShowSuccess("Updated all Packages")
			})
			s.appService.forceRefreshResults()
		}()
	}, s.closeModal)
}

// batchOperation defines the configuration for a batch package operation.
type batchOperation struct {
	actionVerb    string // "Installing" or "Removing"
	actionTag     string // "INSTALL" or "REMOVE"
	skipCondition func(pkg models.Package) bool
	skipReason    string
	execute       func(pkg models.Package) error
}

// handleBatchPackageOperation processes multiple packages with progress notifications.
func (s *InputService) handleBatchPackageOperation(op batchOperation) {
	if !s.appService.IsBrewfileMode() {
		return
	}

	packages := *s.appService.GetBrewfilePackages()
	if len(packages) == 0 {
		s.layout.GetNotifier().ShowError("No packages found in Brewfile")
		return
	}

	// Count relevant packages
	actionable := 0
	for _, pkg := range packages {
		if !op.skipCondition(pkg) {
			actionable++
		}
	}

	if actionable == 0 {
		s.layout.GetNotifier().ShowWarning(fmt.Sprintf("No packages to process (%s)", op.skipReason))
		return
	}

	message := fmt.Sprintf("%s all packages from Brewfile?\n\nTotal: %d packages\nTo process: %d",
		op.actionVerb, len(packages), actionable)

	s.showModal(message, func() {
		s.closeModal()
		s.layout.GetOutput().Clear()
		go func() {
			current := 0
			total := len(packages)

			for _, pkg := range packages {
				current++
				pkgName := pkg.Name // Capture for closures

				if op.skipCondition(pkg) {
					s.appService.app.QueueUpdateDraw(func() {
						s.layout.GetNotifier().ShowWarning(fmt.Sprintf("[%d/%d] Skipping %s (%s)", current, total, pkgName, op.skipReason))
						fmt.Fprintf(s.layout.GetOutput().View(), "[SKIP] %s (%s)\n", pkgName, op.skipReason)
					})
					continue
				}

				s.appService.app.QueueUpdateDraw(func() {
					s.layout.GetNotifier().ShowWarning(fmt.Sprintf("[%d/%d] %s %s...", current, total, op.actionVerb, pkgName))
					fmt.Fprintf(s.layout.GetOutput().View(), "\n[%s] %s %s...\n", op.actionTag, op.actionVerb, pkgName)
				})

				if err := op.execute(pkg); err != nil {
					s.appService.app.QueueUpdateDraw(func() {
						s.layout.GetNotifier().ShowError(fmt.Sprintf("[%d/%d] Failed to process %s", current, total, pkgName))
						fmt.Fprintf(s.layout.GetOutput().View(), "[ERROR] Failed to process %s: %v\n", pkgName, err)
					})
					continue
				}

				s.appService.app.QueueUpdateDraw(func() {
					fmt.Fprintf(s.layout.GetOutput().View(), "[SUCCESS] %s processed successfully\n", pkgName)
				})
			}

			s.appService.app.QueueUpdateDraw(func() {
				s.layout.GetNotifier().ShowSuccess(fmt.Sprintf("Completed! Processed %d packages", total))
			})
			s.appService.forceRefreshResults()
		}()
	}, s.closeModal)
}

// handleInstallAllPackagesEvent is called when the user presses the install all key (Ctrl+A).
func (s *InputService) handleInstallAllPackagesEvent() {
	s.handleBatchPackageOperation(batchOperation{
		actionVerb:    "Installing",
		actionTag:     "INSTALL",
		skipCondition: func(pkg models.Package) bool { return pkg.LocallyInstalled },
		skipReason:    "already installed",
		execute: func(pkg models.Package) error {
			switch pkg.Type {
			case models.PackageTypeFlatpak:
				return s.flatpakService.InstallPackage(pkg, s.outputWriter())
			case models.PackageTypeMas:
				return s.appService.masService.InstallApp(pkg, s.outputWriter())
			default:
				return s.brewService.InstallPackage(pkg, s.outputWriter())
			}
		},
	})
}

// handleRemoveAllPackagesEvent is called when the user presses the remove all key (Ctrl+R).
func (s *InputService) handleRemoveAllPackagesEvent() {
	s.handleBatchPackageOperation(batchOperation{
		actionVerb:    "Removing",
		actionTag:     "REMOVE",
		skipCondition: func(pkg models.Package) bool { return !pkg.LocallyInstalled },
		skipReason:    "not installed",
		execute: func(pkg models.Package) error {
			switch pkg.Type {
			case models.PackageTypeFlatpak:
				return s.flatpakService.RemovePackage(pkg, s.outputWriter())
			case models.PackageTypeMas:
				return s.appService.masService.RemoveApp(pkg, s.outputWriter())
			default:
				return s.brewService.RemovePackage(pkg, s.outputWriter())
			}
		},
	})
}
