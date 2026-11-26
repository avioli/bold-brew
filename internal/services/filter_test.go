package services

import "testing"

func TestPackageFilterTitle(t *testing.T) {
	tests := []struct {
		filter PackageFilter
		want   string
	}{
		{PackageFilter{}, "Formulas & Casks"},
		{PackageFilter{Kind: KindFormulae}, "Formulas"},
		{PackageFilter{Kind: KindCasks}, "Casks"},
		{PackageFilter{Status: StatusInstalled}, "Installed Formulas & Casks"},
		{PackageFilter{Status: StatusInstalled, Kind: KindCasks}, "Installed Casks"},
		{PackageFilter{Status: StatusOutdated, Kind: KindFormulae}, "Outdated Formulas"},
		{PackageFilter{Status: StatusLeaves}, "Leaves"},
		{PackageFilter{Status: StatusLeaves, Kind: KindCasks}, "Cask Leaves"},
	}
	for _, tt := range tests {
		if got := tt.filter.Title(); got != tt.want {
			t.Errorf("%+v.Title() = %q, want %q", tt.filter, got, tt.want)
		}
	}
}
