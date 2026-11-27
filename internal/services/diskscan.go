package services

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"bbrew/internal/models"
	"bbrew/internal/ui/components"
)

// startDiskScan measures disk usage in the background. The table (sizes,
// size sort) and the details panel refresh as packages are measured, and
// the totals are reported at the end.
// A scan started while another runs replaces it.
func (s *AppService) startDiskScan() {
	s.mu.RLock()
	pkgs := make([]models.Package, len(*s.packages))
	copy(pkgs, *s.packages)
	s.mu.RUnlock()

	refresh := func() {
		s.search(s.layout.GetSearch().Field().GetText(), false)
		s.refreshDetails()
	}

	// Show sizes as they come in, redrawing at most every
	// diskRefreshInterval, and never queueing a redraw while one is pending.
	var last time.Time
	var pending atomic.Bool
	progress := func() {
		if time.Since(last) < diskRefreshInterval || !pending.CompareAndSwap(false, true) {
			return
		}
		last = time.Now()
		s.app.QueueUpdateDraw(func() {
			refresh()
			pending.Store(false)
		})
	}

	go func() {
		if !s.diskUsage.Scan(pkgs, progress) {
			return
		}
		totals, _ := s.diskUsage.Totals()
		s.app.QueueUpdateDraw(func() {
			refresh()
			s.layout.GetNotifier().ShowInfo(diskTotalsMessage(totals))
		})
	}()
}

// diskRefreshInterval is how often the table redraws while a disk scan runs.
const diskRefreshInterval = 400 * time.Millisecond

// refreshDetails redraws the details panel for the selected package.
func (s *AppService) refreshDetails() {
	row, _ := s.layout.GetTable().View().GetSelection()
	if row <= 0 || row-1 >= len(*s.filteredPackages) {
		return
	}
	pkg := &(*s.filteredPackages)[row-1]
	vulns, _ := s.vulnsService.GetCachedVulns(pkg.Name)
	s.layout.GetDetails().SetContent(pkg, vulns)
}

// installedSize is the measured installed size of a package, or 0.
func (s *AppService) installedSize(pkg models.Package) int64 {
	if !pkg.LocallyInstalled {
		return 0
	}
	if u := s.diskUsage.Get(pkg.Type, pkg.Name); u != nil {
		return u.Installed
	}
	return 0
}

// sizeText is the Size column: the installed size, empty when not installed
// or not measured yet.
func (s *AppService) sizeText(pkg models.Package) string {
	if n := s.installedSize(pkg); n > 0 {
		return components.FormatBytes(n)
	}
	return ""
}

// diskTotalsMessage summarises a scan for the status bar.
func diskTotalsMessage(t models.DiskTotals) string {
	parts := []string{"Disk: formulae " + components.FormatBytes(t.Formulae)}
	if t.Old > 0 {
		parts[0] += fmt.Sprintf(" (%s old)", components.FormatBytes(t.Old))
	}
	parts = append(parts,
		"casks "+components.FormatBytes(t.Casks),
		"cask app data "+components.FormatBytes(t.AppData),
		"cache "+components.FormatBytes(t.Cache))
	msg := strings.Join(parts, ", ")
	if t.Unreadable > 0 {
		msg += fmt.Sprintf(" (%d unreadable)", t.Unreadable)
	}
	return msg
}
