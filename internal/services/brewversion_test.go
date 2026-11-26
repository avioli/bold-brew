package services

import "testing"

func TestBrewMajorVersion(t *testing.T) {
	tests := map[string]int{
		"Homebrew 7.0.16-50-g038f8a1":                     7,
		"Homebrew 4.5.0\nHomebrew/homebrew-core (git)":    4,
		"Homebrew 10.1.2":                                 10,
		"Homebrew >=4.3.0 (shallow or no git repository)": 0,
		"":  0,
		"-": 0,
	}
	for in, want := range tests {
		if got := brewMajorVersion(in); got != want {
			t.Errorf("brewMajorVersion(%q) = %d, want %d", in, got, want)
		}
	}
}
