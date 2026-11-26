package services

import (
	"sort"

	"bbrew/internal/models"
)

// depIndex maps package names to their direct dependencies and dependents,
// built from the package data already loaded (no brew calls).
type depIndex struct {
	forward   map[string][]string
	reverse   map[string][]string
	installed map[string]bool
}

func newDepIndex(pkgs []models.Package) *depIndex {
	ix := &depIndex{
		forward:   make(map[string][]string, len(pkgs)),
		reverse:   make(map[string][]string),
		installed: make(map[string]bool),
	}
	for i := range pkgs {
		p := &pkgs[i]
		deps := p.Dependencies()
		ix.forward[p.Name] = deps
		for _, d := range deps {
			ix.reverse[d] = append(ix.reverse[d], p.Name)
		}
		if p.LocallyInstalled {
			ix.installed[p.Name] = true
		}
	}
	return ix
}

// dependencies returns everything root depends on, recursively (like
// `brew deps`), excluding root.
func (ix *depIndex) dependencies(root string) map[string]bool {
	seen := map[string]bool{}
	queue := append([]string(nil), ix.forward[root]...)
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if seen[name] || name == root {
			continue
		}
		seen[name] = true
		queue = append(queue, ix.forward[name]...)
	}
	return seen
}

// dependents returns the packages that directly depend on root (like
// `brew uses`).
func (ix *depIndex) dependents(root string) map[string]bool {
	set := map[string]bool{}
	for _, name := range ix.reverse[root] {
		set[name] = true
	}
	return set
}

// installedDependents returns the installed packages that directly depend on
// root, sorted (like `brew uses --installed`).
func (ix *depIndex) installedDependents(root string) []string {
	var names []string
	for _, name := range ix.reverse[root] {
		if ix.installed[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// relationFilter shows only packages related to one package (its
// dependencies or dependents). While set it replaces the f filter.
type relationFilter struct {
	title   string
	members map[string]bool
}

// depIndex returns the dependency index, building it on first use.
func (s *AppService) dependencyIndex() *depIndex {
	if s.deps == nil {
		s.deps = newDepIndex(*s.packages)
	}
	return s.deps
}
