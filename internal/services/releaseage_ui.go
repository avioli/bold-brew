package services

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"bbrew/internal/config"
	"bbrew/internal/models"
	"bbrew/internal/ui/components"
)

const (
	// releaseSelectDelay is how long a package must stay selected before
	// its release date is looked up (no lookups while scrolling past).
	releaseSelectDelay = 400 * time.Millisecond
	// releaseRedrawDelay batches redraws after lookups finish.
	releaseRedrawDelay = 300 * time.Millisecond
	// releaseConfirmWait is how long an upgrade waits for missing dates
	// before asking anyway.
	releaseConfirmWait = 8 * time.Second
	// releaseListMax is how many recent packages a confirmation lists.
	releaseListMax = 10
)

// setupReleaseAge connects release date lookups to the UI.
func (s *AppService) setupReleaseAge() {
	var pending atomic.Bool
	s.releaseAge.SetUpdateFunc(func() {
		if !pending.CompareAndSwap(false, true) {
			return
		}
		time.AfterFunc(releaseRedrawDelay, func() {
			s.app.QueueUpdateDraw(func() {
				pending.Store(false)
				s.setResults(s.filteredPackages, false)
				s.refreshDetails()
			})
		})
	})
	s.layout.GetDetails().SetReleaseFunc(func(pkg *models.Package) (models.ReleaseInfo, int) {
		return s.releaseInfo(*pkg)
	})
}

// releaseInfo returns what is known about pkg's release date and its level
// (components.ReleaseOld, ReleaseRecent or ReleaseFresh).
func (s *AppService) releaseInfo(pkg models.Package) (models.ReleaseInfo, int) {
	info := s.releaseAge.Get(pkg)
	return info, releaseLevel(info, config.Get().ReleaseAge, time.Now())
}

// requestReleaseDates looks up the release dates of the outdated packages,
// the versions an upgrade would install.
func (s *AppService) requestReleaseDates() {
	s.releaseAge.Request(s.outdatedPackages(false))
}

// releaseAgeChanged re-checks release dates after the settings changed.
func (s *AppService) releaseAgeChanged() {
	s.requestReleaseDates()
	s.setResults(s.filteredPackages, false)
	s.refreshDetails()
}

// scheduleSelectedRelease looks up the selected package's release date once
// it stays selected for a moment (release_age.mode = "selected"). Called on
// the UI goroutine.
func (s *AppService) scheduleSelectedRelease(pkg models.Package) {
	if s.releaseTimer != nil {
		s.releaseTimer.Stop()
	}
	if config.Get().ReleaseAge.Mode != "selected" {
		return
	}
	s.releaseTimer = time.AfterFunc(releaseSelectDelay, func() {
		s.releaseAge.Request([]models.Package{pkg})
	})
}

// outdatedPackages lists the installed Homebrew packages that `brew upgrade`
// would upgrade: outdated and not pinned. With greedy, casks that update
// themselves and whose installed version differs from the latest are
// included too.
func (s *AppService) outdatedPackages(greedy bool) []models.Package {
	s.mu.RLock()
	defer s.mu.RUnlock()
	seen := make(map[string]bool)
	var out []models.Package
	add := func(list *[]models.Package) {
		if list == nil {
			return
		}
		for _, p := range *list {
			key := string(p.Type) + ":" + p.Name
			if seen[key] || !p.LocallyInstalled {
				continue
			}
			upgradable := p.Outdated
			switch {
			case p.Formula != nil:
				upgradable = upgradable && !p.Formula.Pinned
			case p.Cask != nil:
				c := p.Cask
				upgradable = upgradable || (greedy && c.AutoUpdates && c.Installed != nil &&
					c.Version != "latest" && *c.Installed != c.Version)
			default:
				upgradable = false
			}
			if upgradable {
				seen[key] = true
				out = append(out, p)
			}
		}
	}
	add(s.packages)
	add(s.brewfilePackages)
	return out
}

// withRecentReleases calls then (on the UI goroutine) with the packages
// among pkgs whose version was published within release_age.recent_hours.
// Missing dates are looked up first, waiting up to releaseConfirmWait.
func (s *InputService) withRecentReleases(pkgs []models.Package, then func(recent []models.Package)) {
	cfg := config.Get().ReleaseAge
	if cfg.Mode == "off" || cfg.RecentHours == 0 || len(pkgs) == 0 {
		then(nil)
		return
	}
	ra := s.appService.releaseAge
	ra.Request(pkgs)
	collect := func() []models.Package {
		var recent []models.Package
		for _, p := range pkgs {
			if _, level := s.appService.releaseInfo(p); level != components.ReleaseOld {
				recent = append(recent, p)
			}
		}
		return recent
	}
	if !ra.AnyPending(pkgs) {
		then(collect())
		return
	}
	s.layout.GetNotifier().ShowWarning("Checking release dates...")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), releaseConfirmWait)
		defer cancel()
		ra.Wait(ctx, pkgs)
		s.appService.app.QueueUpdateDraw(func() {
			then(collect())
		})
	}()
}

// recentReleaseList lists recent packages for a confirmation, one per line,
// e.g. "jq 1.8.1, 3h ago".
func (s *InputService) recentReleaseList(recent []models.Package) string {
	var lines []string
	for i, p := range recent {
		if i == releaseListMax {
			lines = append(lines, fmt.Sprintf("...and %d more", len(recent)-i))
			break
		}
		info, _ := s.appService.releaseInfo(p)
		lines = append(lines, fmt.Sprintf("%s %s, %s", p.Label(), p.Version, components.FormatAge(time.Since(info.Time))))
	}
	return strings.Join(lines, "\n")
}

// recentWarning is the confirmation text for acting on recent versions.
func (s *InputService) recentWarning(recent []models.Package) string {
	hours := config.Get().ReleaseAge.RecentHours
	return fmt.Sprintf("⚠ Released in the last %dh:\n\n%s\n\nVery new versions have had little time "+
		"for vulnerability scanners and reports to catch up.", hours, s.recentReleaseList(recent))
}
