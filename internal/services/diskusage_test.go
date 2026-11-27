package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bbrew/internal/models"
)

func TestDiskUsageScan(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	dirs := brewDirs{
		Cache:    filepath.Join(root, "cache"),
		Cellar:   filepath.Join(root, "Cellar"),
		Caskroom: filepath.Join(root, "Caskroom"),
	}
	big := strings.Repeat("x", 64*1024)

	// Formula with a current and an old keg; the old one hard links a file
	// from the current one, which must count once.
	writeFile(t, filepath.Join(dirs.Cellar, "foo/2.0/bin/foo"), big)
	writeFile(t, filepath.Join(dirs.Cellar, "foo/1.0/bin/foo-old"), big)
	if err := os.Link(filepath.Join(dirs.Cellar, "foo/2.0/bin/foo"), filepath.Join(dirs.Cellar, "foo/1.0/bin/foo-link")); err != nil {
		t.Fatal(err)
	}

	// Cache: a formula bottle and manifest, a cask download, all symlinks
	// into downloads/, plus an orphan download.
	writeFile(t, filepath.Join(dirs.Cache, "downloads/aaa--foo--2.0.bottle.tar.gz"), big)
	writeFile(t, filepath.Join(dirs.Cache, "downloads/bbb--foo-2.0.bottle_manifest.json"), "{}")
	writeFile(t, filepath.Join(dirs.Cache, "downloads/ccc--bar.dmg"), big)
	writeFile(t, filepath.Join(dirs.Cache, "downloads/orphan"), big)
	link := func(target, name string) {
		if err := os.Symlink(target, filepath.Join(dirs.Cache, name)); err != nil {
			t.Fatal(err)
		}
	}
	link("downloads/aaa--foo--2.0.bottle.tar.gz", "foo--2.0")
	link("downloads/bbb--foo-2.0.bottle_manifest.json", "foo_bottle_manifest--2.0")
	if err := os.MkdirAll(filepath.Join(dirs.Cache, "Cask"), 0o755); err != nil {
		t.Fatal(err)
	}
	link("../downloads/ccc--bar.dmg", "Cask/bar--1.0.dmg")

	// Cask with an app (a name with glob characters), zap data and a
	// zap path inside a privacy-protected directory.
	app := filepath.Join(home, "Applications/Bar [x].app")
	writeFile(t, filepath.Join(app, "Contents/MacOS/bar"), big)
	writeFile(t, filepath.Join(dirs.Caskroom, "bar/1.0/.metadata"), "m")
	writeFile(t, filepath.Join(home, "Library/Caches/com.bar/data"), big)
	writeFile(t, filepath.Join(home, "Library/Containers/com.bar/data"), big)

	pkgs := []models.Package{
		{Name: "foo", Type: models.PackageTypeFormula, LocallyInstalled: true,
			Formula: &models.Formula{Name: "foo", LinkedKeg: "2.0"}},
		{Name: "bar", Type: models.PackageTypeCask, LocallyInstalled: true,
			Cask: &models.Cask{Token: "bar", Artifacts: []map[string]any{
				{"app": []any{"Bar [x].app"}, "target": app},
				{"binary": []any{"bar"}, "target": "/nonexistent/bin/bar"},
				{"zap": []any{map[string]any{"trash": []any{
					"~/Library/Caches/com.ba*",
					"~/Library/Containers/com.bar",
					"~/Library/Preferences/missing.plist",
				}}}},
			}}},
	}

	s := NewDiskUsageService()
	s.homeDir = home
	s.getDirs = func() brewDirs { return dirs }
	progress := 0
	if !s.Scan(pkgs, func() { progress++ }) {
		t.Fatal("scan reported stale")
	}

	if progress != 2 {
		t.Errorf("progress called %d times, want once per installed package", progress)
	}

	foo := s.Get(models.PackageTypeFormula, "foo")
	if foo == nil || len(foo.InstalledPaths) != 2 {
		t.Fatalf("foo usage = %+v", foo)
	}
	if foo.Old <= 0 || foo.Old >= foo.Installed {
		t.Errorf("foo old = %d of %d, want part of it", foo.Old, foo.Installed)
	}
	// Two distinct 64K files (the hard link counts once) plus directories.
	if foo.Installed < 2*64*1024 || foo.Installed >= 3*64*1024 {
		t.Errorf("foo installed = %d, hard link counted twice?", foo.Installed)
	}
	if len(foo.CachePaths) != 2 || foo.Cache < 64*1024 {
		t.Errorf("foo cache = %d %+v", foo.Cache, foo.CachePaths)
	}

	bar := s.Get(models.PackageTypeCask, "bar")
	if bar == nil {
		t.Fatal("no usage for bar")
	}
	if len(bar.InstalledPaths) != 2 || bar.Installed < 64*1024 {
		t.Errorf("bar installed = %d %+v", bar.Installed, bar.InstalledPaths)
	}
	if bar.Cache < 64*1024 {
		t.Errorf("bar cache = %d", bar.Cache)
	}
	if len(bar.AppDataPaths) != 1 || !strings.HasSuffix(bar.AppDataPaths[0].Path, "com.bar") {
		t.Errorf("bar app data = %+v", bar.AppDataPaths)
	}
	if len(bar.Skipped) != 1 || !strings.Contains(bar.Skipped[0], "Containers") {
		t.Errorf("bar skipped = %v", bar.Skipped)
	}

	tot, ok := s.Totals()
	if !ok || tot.Formulae != foo.Installed || tot.Casks != bar.Installed || tot.AppData != bar.AppData {
		t.Errorf("totals = %+v", tot)
	}
	if tot.CacheOther < 64*1024 {
		t.Errorf("orphan download not in CacheOther: %+v", tot)
	}
}

func TestDiskUsageProtected(t *testing.T) {
	s := &DiskUsageService{homeDir: "/Users/me"}
	for p, want := range map[string]bool{
		"/Users/me/Documents/x":                 true,
		"/Users/me/Library/Containers":          true,
		"/Users/me/Library/Group Containers/a":  true,
		"/Users/me/Library/Caches/x":            false,
		"/Users/me/DocumentsX":                  false,
		"/Library/Application Support/whatever": false,
	} {
		if got := s.protected(p); got != want {
			t.Errorf("protected(%q) = %v, want %v", p, got, want)
		}
	}
}
