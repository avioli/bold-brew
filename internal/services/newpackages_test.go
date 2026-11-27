package services

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestNewPackageNames(t *testing.T) {
	cache := t.TempDir()
	writeFile(t, filepath.Join(cache, "api/formula_names.before.txt"), "a\nb\n")
	writeFile(t, filepath.Join(cache, "api/formula_names.txt"), "a\nb\nc\n")
	writeFile(t, filepath.Join(cache, "api/cask_names.before.txt"), "x\ngone\n")
	writeFile(t, filepath.Join(cache, "api/cask_names.txt"), "x\ny\nz\n")

	f, c := newPackageNames(cache)
	if want := map[string]bool{"c": true}; !reflect.DeepEqual(f, want) {
		t.Errorf("formulae = %v, want %v", f, want)
	}
	if want := map[string]bool{"y": true, "z": true}; !reflect.DeepEqual(c, want) {
		t.Errorf("casks = %v, want %v", c, want)
	}
}

func TestNewPackageNamesWithoutBaseline(t *testing.T) {
	cache := t.TempDir()
	writeFile(t, filepath.Join(cache, "api/formula_names.txt"), "a\n")
	f, c := newPackageNames(cache)
	if len(f) != 0 || len(c) != 0 {
		t.Errorf("got %v, %v; want nothing new without a .before.txt", f, c)
	}
	if f, c := newPackageNames(""); len(f) != 0 || len(c) != 0 {
		t.Errorf("no cache dir: got %v, %v", f, c)
	}
}
