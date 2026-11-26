package services

import (
	"regexp"
	"strconv"
)

var brewVersionPattern = regexp.MustCompile(`Homebrew (\d+)\.`)

// brewMajorVersion extracts the major version from `brew --version` output
// ("Homebrew 7.0.16-50-g038f8a1"), or 0 when it cannot be parsed.
func brewMajorVersion(version string) int {
	m := brewVersionPattern.FindStringSubmatch(version)
	if m == nil {
		return 0
	}
	major, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return major
}

// brewSourceMinMajor is the first Homebrew major version with `brew source`.
const brewSourceMinMajor = 7

// hasBrewSource reports whether the installed Homebrew has `brew source`.
func (s *AppService) hasBrewSource() bool {
	return brewMajorVersion(s.brewVersion) >= brewSourceMinMajor
}
