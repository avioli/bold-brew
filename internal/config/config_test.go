package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMissingGivesDefaults(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if c != Default() {
		t.Fatalf("got %+v", c)
	}
}

func TestLoadPartialKeepsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	data := "[view]\nsort = \"Rank\"\nlog_height = 999\n[release_age]\nrecent_hours = 6\nfresh_hours = 12\n[vulns]\nmin_severity = \"bogus\"\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.View.Sort != "rank" || c.View.LogHeight != MaxLogHeight || !c.View.Borders {
		t.Errorf("view: %+v", c.View)
	}
	if c.ReleaseAge.RecentHours != 6 || c.ReleaseAge.FreshHours != 6 || c.ReleaseAge.Mode != "selected" {
		t.Errorf("release age: %+v", c.ReleaseAge)
	}
	if c.Vulns.MinSeverity != "any" {
		t.Errorf("vulns: %+v", c.Vulns)
	}
}

func TestLoadBrokenFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[view\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err == nil || c != Default() {
		t.Fatalf("got %+v, %v", c, err)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.toml")
	want := Default()
	want.View.Sort = "size"
	want.Actions.Greedy = true
	want.ReleaseAge.HoldInUpdateAll = true
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	data, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(data), "# Bold Brew settings") {
		t.Error("header missing")
	}
}

func TestDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/x/cfg")
	if got := Dir(); got != "/x/cfg/bbrew" {
		t.Errorf("got %s", got)
	}
	t.Setenv("XDG_CONFIG_HOME", "relative")
	home, _ := os.UserHomeDir()
	if got := Dir(); got != filepath.Join(home, ".config", "bbrew") {
		t.Errorf("got %s", got)
	}
}
