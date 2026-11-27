package services

import (
	"bbrew/internal/models"
	"fmt"
	"sort"
	"strings"

	"bbrew/internal/ui/components"

	"github.com/gdamore/tcell/v2"
)

// search filters the packages based on the search text and the current filter state.
func (s *AppService) search(searchText string, scrollToTop bool) {
	var filteredList []models.Package
	uniquePackages := make(map[string]bool)

	// Determine the source list based on the current filter state
	// If Brewfile mode is active, use brewfilePackages as the base source
	sourceList := s.packages
	if s.IsBrewfileMode() {
		sourceList = s.brewfilePackages
	}

	// Apply active filter on the source list
	sourceList = s.applyFilter(sourceList)

	if searchText == "" {
		filteredList = *sourceList
	} else {
		searchTextLower := strings.ToLower(searchText)
		for _, info := range *sourceList {
			if strings.Contains(strings.ToLower(info.Name), searchTextLower) ||
				strings.Contains(strings.ToLower(info.DisplayName), searchTextLower) ||
				strings.Contains(strings.ToLower(info.Description), searchTextLower) {
				if !uniquePackages[info.Name] {
					filteredList = append(filteredList, info)
					uniquePackages[info.Name] = true
				}
			}
		}
	}

	s.applySortOrder(filteredList)
	*s.filteredPackages = filteredList
	s.setResults(s.filteredPackages, scrollToTop)
}

// applySortOrder sorts the list in place based on the active sort mode.
// SortBrewfile keeps the natural (Brewfile) order.
func (s *AppService) applySortOrder(list []models.Package) {
	switch s.activeSort {
	case models.SortByDownloads:
		sort.Slice(list, func(i, j int) bool {
			return list[i].Analytics90dDownloads > list[j].Analytics90dDownloads
		})
	case models.SortByName:
		sort.SliceStable(list, func(i, j int) bool {
			return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
		})
	case models.SortBySize:
		sort.SliceStable(list, func(i, j int) bool {
			return s.installedSize(list[i]) > s.installedSize(list[j])
		})
	case models.SortByRank:
		// Unranked packages (0) go last.
		sort.SliceStable(list, func(i, j int) bool {
			a, b := list[i].Analytics90dRank, list[j].Analytics90dRank
			if (a > 0) != (b > 0) {
				return a > 0
			}
			return a < b
		})
	default: // SortBrewfile - keep Brewfile order
	}
}

// CycleSortMode advances to the next sort mode and re-applies the search.
func (s *AppService) CycleSortMode() models.SortMode {
	s.activeSort = s.activeSort.Next(s.IsBrewfileMode())
	s.search(s.layout.GetSearch().Field().GetText(), false)
	return s.activeSort
}

// applyFilter filters packages based on the active filter.
func (s *AppService) applyFilter(sourceList *[]models.Package) *[]models.Package {
	if s.relation != nil {
		filteredSource := &[]models.Package{}
		for _, info := range *sourceList {
			if s.relation.members[info.Name] {
				*filteredSource = append(*filteredSource, info)
			}
		}
		return filteredSource
	}

	if s.activeFilter.IsNone() {
		return sourceList
	}

	filteredSource := &[]models.Package{}
	for _, info := range *sourceList {
		if s.activeFilter.Matches(info) {
			*filteredSource = append(*filteredSource, info)
		}
	}
	return filteredSource
}

// forceRefreshResults forces a refresh of the Homebrew formulae and cask data and updates the results in the UI.
func (s *AppService) forceRefreshResults() {
	// Force refresh all data to get up-to-date versions and installed status
	_ = s.dataProvider.SetupData(true)
	newPackages := s.dataProvider.GetPackages()

	// If in Brewfile mode, load tap packages and verify installed status
	s.mu.Lock()
	s.packages = newPackages

	if s.IsBrewfileMode() {
		s.fetchTapPackages()
		_ = s.loadBrewfilePackages()
		*s.filteredPackages = *s.brewfilePackages
	} else {
		installedCasks := s.dataProvider.FetchInstalledCaskNames()
		installedFormulae := s.dataProvider.FetchInstalledFormulaNames()
		for i := range *s.packages {
			pkg := &(*s.packages)[i]
			if pkg.Type == models.PackageTypeCask {
				pkg.LocallyInstalled = installedCasks[pkg.Name]
			} else {
				pkg.LocallyInstalled = installedFormulae[pkg.Name]
			}
		}
		*s.filteredPackages = *s.packages
	}
	s.mu.Unlock()
	s.resetDependencyIndex()
	s.startDiskScan()
	s.requestReleaseDates()

	s.app.QueueUpdateDraw(func() {
		s.search(s.layout.GetSearch().Field().GetText(), false)
	})
}

// setResults updates the results table with the provided data and optionally scrolls to the top.
func (s *AppService) setResults(data *[]models.Package, scrollToTop bool) {
	sortColumn := -1
	switch s.activeSort {
	case models.SortByName:
		sortColumn = components.ColName
	case models.SortByDownloads:
		sortColumn = components.ColDownloads
	case models.SortByRank:
		sortColumn = components.ColRank
	case models.SortBySize:
		sortColumn = components.ColSize
	}

	rows := make([]components.Row, 0, len(*data))
	for _, info := range *data {
		row := components.Row{
			Type:         typeLetter(info.Type),
			Name:         info.Label(),
			Version:      info.Version,
			Description:  info.Description,
			Downloads:    fmt.Sprintf("%d", info.Analytics90dDownloads),
			Rank:         rankText(info.Analytics90dRank),
			Size:         s.sizeText(info),
			NameColor:    tcell.ColorDefault,
			VersionColor: tcell.ColorDefault,
		}

		// Name color based on status, description with warning prefix
		switch {
		case info.Disabled:
			row.NameColor = tcell.ColorRed
			row.Description = "[DISABLED] " + row.Description
		case info.Deprecated:
			row.NameColor = tcell.ColorYellow
			row.Description = "[DEPRECATED] " + row.Description
		case info.LocallyInstalled:
			row.NameColor = tcell.ColorGreen
		}
		if info.New {
			row.Description = "[NEW] " + row.Description
		}
		if info.LocallyInstalled && info.Outdated {
			row.VersionColor = tcell.ColorOrange
		}
		// Versions published very recently: yellow, or red when very fresh.
		_, release := s.releaseInfo(info)
		switch release {
		case components.ReleaseFresh:
			row.VersionColor = tcell.ColorRed
		case components.ReleaseRecent:
			row.VersionColor = tcell.ColorYellow
		}

		// Without colour: installed names bold, outdated versions underlined.
		// Disabled/deprecated already say so in the description.
		if s.noColor {
			if info.LocallyInstalled {
				row.NameAttr = tcell.AttrBold
			}
			if info.LocallyInstalled && info.Outdated {
				row.VersionAttr = tcell.AttrUnderline
			}
			if release != components.ReleaseOld {
				row.VersionAttr |= tcell.AttrReverse
			}
		}

		rows = append(rows, row)
	}
	s.layout.GetTable().SetRows(rows, sortColumn)

	// Update the details view with the first item in the list
	if len(*data) > 0 && scrollToTop {
		s.layout.GetTable().View().Select(1, 0)
		s.layout.GetTable().View().ScrollToBeginning()
		pkg := &(*data)[0]
		vulns, _ := s.vulnsService.GetCachedVulns(pkg.Name)
		s.layout.GetDetails().SetContent(pkg, vulns)
	} else if len(*data) == 0 {
		s.layout.GetDetails().SetContent(nil, nil)
	}

	// Update the filter counter
	// In Brewfile mode, show total Brewfile packages instead of all packages
	totalCount := len(*s.packages)
	if s.IsBrewfileMode() {
		totalCount = len(*s.brewfilePackages)
	}
	s.layout.GetSearch().UpdateCounter(totalCount, len(*s.filteredPackages))
}

// typeLetter returns the one-letter type indicator for a package type.
func typeLetter(t models.PackageType) string {
	switch t {
	case models.PackageTypeCask:
		return "C"
	case models.PackageTypeFlatpak:
		return "P"
	case models.PackageTypeMas:
		return "M"
	default:
		return "F"
	}
}

// rankText formats a 90 day analytics rank; 0 means unranked.
func rankText(rank int) string {
	if rank <= 0 {
		return ""
	}
	return fmt.Sprintf("%d", rank)
}
