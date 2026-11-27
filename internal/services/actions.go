package services

import (
	"fmt"

	"bbrew/internal/models"
	"bbrew/internal/ui/components"
)

// actionMenuItems returns the entries of the a (actions) menu. Except for
// Update all and Export, they act on the selected package.
func (s *InputService) actionMenuItems() []menuItem {
	return []menuItem{
		{'i', "Install", s.handleInstallPackageEvent},
		{'u', "Update", s.handleUpdatePackageEvent},
		{'r', "Remove", s.handleRemovePackageEvent},
		{'I', "Reinstall", s.handleReinstallEvent},
		{'U', "Update all outdated", s.handleUpdateAllPackagesEvent},
		{'F', "Fetch (download only)", func() {
			s.brewAction(brewActionSpec{command: "fetch", doing: "Fetching", done: "Fetched", typeFlag: true})
		}},
		{'p', "Pin (exclude from Update all)", func() {
			s.brewAction(brewActionSpec{command: "pin", doing: "Pinning", done: "Pinned", formulaOnly: true, installedOnly: true, refresh: true})
		}},
		{'P', "Unpin", func() {
			s.brewAction(brewActionSpec{command: "unpin", doing: "Unpinning", done: "Unpinned", formulaOnly: true, installedOnly: true, refresh: true})
		}},
		{'l', "Link", func() {
			s.brewAction(brewActionSpec{command: "link", doing: "Linking", done: "Linked", formulaOnly: true, installedOnly: true, refresh: true})
		}},
		{'L', "Unlink", func() {
			s.brewAction(brewActionSpec{command: "unlink", doing: "Unlinking", done: "Unlinked", formulaOnly: true, installedOnly: true, refresh: true})
		}},
		{'s', "Start service", func() { s.serviceAction("start", "Starting", "Started") }},
		{'S', "Stop service", func() { s.serviceAction("stop", "Stopping", "Stopped") }},
		{'T', "Restart service", func() { s.serviceAction("restart", "Restarting", "Restarted") }},
		{'v', "Vulnerability scan", s.handleVulnScanEvent},
		{'e', "Export installed to a Brewfile (see options)", s.handleExportEvent},
	}
}

// actionMenuHelp lists the a menu for the help screen.
func (s *InputService) actionMenuHelp() components.HelpSection {
	section := components.HelpSection{Title: "ACTIONS MENU (a, then...)"}
	for _, item := range s.actionMenuItems() {
		section.Keys = append(section.Keys, components.HelpKey{Keys: "a" + string(item.key), Description: item.label})
	}
	return section
}

// handleActionMenuEvent opens the a menu.
func (s *InputService) handleActionMenuEvent() {
	s.showMenu(" Actions ", s.actionMenuItems())
}

// brewActionSpec describes a simple brew command run on the selected package.
type brewActionSpec struct {
	command     string   // brew subcommand
	extra       []string // arguments before the package name
	doing, done string   // "Pinning", "Pinned"
	typeFlag    bool     // pass --formula/--cask
	formulaOnly bool
	// installedOnly requires the package to be installed.
	installedOnly bool
	// refresh reloads package data afterwards (state shown in the UI changed).
	refresh bool
}

// brewAction runs a brew command on the selected package, streaming output
// to the Log and reporting the result in the status line.
func (s *InputService) brewAction(spec brewActionSpec) {
	pkg := s.selectedBrewPackage("brew " + spec.command)
	if pkg == nil {
		return
	}
	if spec.formulaOnly && pkg.Type != models.PackageTypeFormula {
		s.layout.GetNotifier().ShowWarning(fmt.Sprintf("brew %s only works for formulae", spec.command))
		return
	}
	if spec.installedOnly && !pkg.LocallyInstalled {
		s.layout.GetNotifier().ShowWarning(fmt.Sprintf("%s is not installed", pkg.Label()))
		return
	}

	args := append([]string{spec.command}, spec.extra...)
	if spec.typeFlag {
		args = append(args, brewTypeFlag(pkg))
	}
	args = append(args, pkg.Name)

	name := pkg.Label()
	s.layout.GetOutput().Clear()
	s.layout.GetNotifier().ShowWarning(fmt.Sprintf("%s %s...", spec.doing, name))
	go func() {
		err := ExecuteCommand(brewCommand(args...), s.outputWriter()) // #nosec G204
		s.appService.app.QueueUpdateDraw(func() {
			if err != nil {
				s.layout.GetNotifier().ShowError(fmt.Sprintf("brew %s %s failed: %v", spec.command, name, err))
				return
			}
			s.layout.GetNotifier().ShowSuccess(fmt.Sprintf("%s %s", spec.done, name))
		})
		if err == nil && spec.refresh {
			s.appService.forceRefreshResults()
		}
	}()
}

// serviceAction runs brew services start|stop|restart on the selected
// formula, if it defines a service.
func (s *InputService) serviceAction(command, doing, done string) {
	pkg := s.selectedPackage()
	if pkg == nil {
		return
	}
	if pkg.Formula == nil || pkg.Formula.Service == nil {
		s.layout.GetNotifier().ShowWarning(fmt.Sprintf("%s does not define a service", pkg.Label()))
		return
	}
	s.brewAction(brewActionSpec{
		command: "services", extra: []string{command}, doing: doing + " service", done: done + " service",
		formulaOnly: true, installedOnly: true,
	})
}

// handleReinstallEvent reinstalls the selected package after confirmation.
func (s *InputService) handleReinstallEvent() {
	pkg := s.selectedBrewPackage("brew reinstall")
	if pkg == nil {
		return
	}
	if !pkg.LocallyInstalled {
		s.layout.GetNotifier().ShowWarning(fmt.Sprintf("%s is not installed", pkg.Label()))
		return
	}
	s.confirmAction(fmt.Sprintf("Are you sure you want to reinstall the package: %s?", pkg.Label()), func() {
		s.closeModal()
		s.brewAction(brewActionSpec{command: "reinstall", doing: "Reinstalling", done: "Reinstalled", typeFlag: true, refresh: true})
	})
}
