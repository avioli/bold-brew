package models

import "testing"

func TestSortMode_String(t *testing.T) {
	tests := []struct {
		mode SortMode
		want string
	}{
		{SortByName, "Name"},
		{SortByDownloads, "Downloads"},
		{SortByRank, "Rank"},
		{SortBrewfile, "Brewfile order"},
	}

	for _, tt := range tests {
		if got := tt.mode.String(); got != tt.want {
			t.Errorf("SortMode(%d).String() = %q, want %q", tt.mode, got, tt.want)
		}
	}
}

func TestSortMode_Next(t *testing.T) {
	tests := []struct {
		mode     SortMode
		brewfile bool
		want     SortMode
	}{
		{SortByName, false, SortByDownloads},
		{SortByDownloads, false, SortByRank},
		{SortByRank, false, SortBySize},
		{SortBySize, false, SortByName},
		{SortBySize, true, SortBrewfile},
		{SortBrewfile, true, SortByName},
	}

	for _, tt := range tests {
		if got := tt.mode.Next(tt.brewfile); got != tt.want {
			t.Errorf("SortMode(%d).Next(%v) = %d, want %d", tt.mode, tt.brewfile, got, tt.want)
		}
	}
}

func TestSortMode_DefaultIsName(t *testing.T) {
	var s SortMode
	if s != SortByName {
		t.Errorf("zero SortMode = %v, want Name", s)
	}
}
