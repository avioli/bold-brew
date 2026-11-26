package models

// SortMode represents how the package list is sorted.
type SortMode int

const (
	SortNone        SortMode = iota // No explicit sort (preserves natural/API order)
	SortByDownloads                 // Most downloaded first
	SortByName                      // Alphabetical A-Z
	SortByRank                      // Best 90 day analytics rank first
)

func (s SortMode) String() string {
	switch s {
	case SortByDownloads:
		return "Downloads"
	case SortByName:
		return "Name"
	case SortByRank:
		return "Rank"
	default:
		return "None"
	}
}

// Next cycles to the next sort mode.
func (s SortMode) Next() SortMode {
	return (s + 1) % 4
}
