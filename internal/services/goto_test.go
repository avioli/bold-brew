package services

import (
	"testing"

	"bbrew/internal/models"
)

func TestFormulaePageURL(t *testing.T) {
	tests := []struct {
		pkg  models.Package
		want string
	}{
		{models.Package{Name: "openssl@3", Formula: &models.Formula{Tap: "homebrew/core"}},
			"https://formulae.brew.sh/formula/openssl@3"},
		{models.Package{Name: "firefox", Cask: &models.Cask{Tap: "homebrew/cask"}},
			"https://formulae.brew.sh/cask/firefox"},
		{models.Package{Name: "thing", Formula: &models.Formula{Tap: "someone/tap"}}, ""},
		{models.Package{Name: "app", Type: models.PackageTypeMas}, ""},
	}
	for _, tt := range tests {
		if got := formulaePageURL(&tt.pkg); got != tt.want {
			t.Errorf("formulaePageURL(%s) = %q, want %q", tt.pkg.Name, got, tt.want)
		}
	}
}
