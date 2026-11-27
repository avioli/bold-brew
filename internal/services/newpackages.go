package services

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// newPackageNames returns the formulae and casks that are new since the last
// `brew update` that changed the package lists, the same ones `brew update`
// reports under "New Formulae" and "New Casks".
//
// When installing from the API (the default), `brew update` moves
// {formula,cask}_names.txt in $(brew --cache)/api to *_names.before.txt
// before fetching new data, and update-report diffs the two files. We do the
// same diff. Without a .before.txt (the first update, or
// HOMEBREW_NO_INSTALL_FROM_API) nothing is new.
func newPackageNames(cacheDir string) (formulae, casks map[string]bool) {
	if cacheDir == "" {
		return map[string]bool{}, map[string]bool{}
	}
	api := filepath.Join(cacheDir, "api")
	diff := func(kind string) map[string]bool {
		before := readNameSet(filepath.Join(api, kind+"_names.before.txt"))
		if before == nil {
			return map[string]bool{}
		}
		added := map[string]bool{}
		for name := range readNameSet(filepath.Join(api, kind+"_names.txt")) {
			if !before[name] {
				added[name] = true
			}
		}
		return added
	}
	return diff("formula"), diff("cask")
}

// readNameSet reads one name per line, or returns nil if the file cannot be read.
func readNameSet(path string) map[string]bool {
	f, err := os.Open(path) // #nosec G304 - path is inside the Homebrew cache
	if err != nil {
		return nil
	}
	defer f.Close()
	names := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if name := strings.TrimSpace(sc.Text()); name != "" {
			names[name] = true
		}
	}
	return names
}
