package models

// SortMode represents how the package list is sorted.
type SortMode int

const (
	SortByName      SortMode = iota // Alphabetical A-Z (the default)
	SortByDownloads                 // Most downloaded first
	SortByRank                      // Best 90 day analytics rank first
	SortBySize                      // Most disk space first
	SortBrewfile                    // Brewfile order (Brewfile mode only)
)

func (s SortMode) String() string {
	switch s {
	case SortByDownloads:
		return "Downloads"
	case SortByRank:
		return "Rank"
	case SortBySize:
		return "Size"
	case SortBrewfile:
		return "Brewfile order"
	default:
		return "Name"
	}
}

// Next cycles to the next sort mode: Name, Downloads, Rank, Size, then Brewfile
// order when brewfile is true.
func (s SortMode) Next(brewfile bool) SortMode {
	switch s {
	case SortByName:
		return SortByDownloads
	case SortByDownloads:
		return SortByRank
	case SortByRank:
		return SortBySize
	case SortBySize:
		if brewfile {
			return SortBrewfile
		}
	}
	return SortByName
}
