package services

import (
	"strings"
	"sync"
)

// brewDirs are the Homebrew locations bbrew reads directly.
type brewDirs struct {
	Cache    string // `brew --cache`
	Cellar   string // `brew --cellar`
	Caskroom string // `brew --caskroom`
}

var (
	brewDirsOnce   sync.Once
	brewDirsCached brewDirs
)

// getBrewDirs asks brew for its directories once. Missing ones are "".
var getBrewDirs = func() brewDirs {
	brewDirsOnce.Do(func() {
		ask := func(flag string) string {
			out, err := brewCommand(flag).Output()
			if err != nil {
				return ""
			}
			return strings.TrimSpace(string(out))
		}
		brewDirsCached = brewDirs{
			Cache:    ask("--cache"),
			Cellar:   ask("--cellar"),
			Caskroom: ask("--caskroom"),
		}
	})
	return brewDirsCached
}
