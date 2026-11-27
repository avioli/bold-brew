// Package config loads and saves the bbrew settings file,
// $XDG_CONFIG_HOME/bbrew/config.toml (~/.config/bbrew/config.toml when
// XDG_CONFIG_HOME is unset, on macOS too). A missing file or missing keys
// fall back to the defaults.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
)

// Allowed values of the enum settings, in the order the options panel lists
// them. The first is the default.
var (
	SortModes       = []string{"name", "downloads", "rank", "size"}
	StatusFilters   = []string{"none", "installed", "outdated", "leaves", "new"}
	KindFilters     = []string{"all", "formulae", "casks"}
	ReleaseAgeModes = []string{"selected", "outdated", "off"}
	Severities      = []string{"any", "low", "medium", "high", "critical"}
)

// View holds how the main screen starts.
type View struct {
	Sort      string `toml:"sort"`   // see SortModes
	Filter    string `toml:"filter"` // see StatusFilters
	Kind      string `toml:"kind"`   // see KindFilters
	Borders   bool   `toml:"borders"`
	Details   bool   `toml:"details"`
	Log       bool   `toml:"log"`
	LogHeight int    `toml:"log_height"`
}

// Startup holds the background work done at start.
type Startup struct {
	BrewUpdate       bool `toml:"brew_update"`        // run `brew update`
	CheckBbrewUpdate bool `toml:"check_bbrew_update"` // look for a newer bbrew
	DiskScan         bool `toml:"disk_scan"`          // measure disk usage
}

// ReleaseAge controls the warnings about versions published very recently.
type ReleaseAge struct {
	// Mode is which packages get their release date looked up: "selected"
	// (outdated ones plus the selected package), "outdated" or "off".
	Mode string `toml:"mode"`
	// Casks also looks up casks, from the Last-Modified header of the
	// download URL (a HEAD request to the vendor). Not every server sends one.
	Casks bool `toml:"casks"`
	// RecentHours: a version younger than this is flagged and its upgrade
	// asks for confirmation with a warning.
	RecentHours int `toml:"recent_hours"`
	// FreshHours: a version younger than this is flagged as very fresh.
	FreshHours int `toml:"fresh_hours"`
	// HoldInUpdateAll leaves recent versions out of "Update all outdated".
	HoldInUpdateAll bool `toml:"hold_in_update_all"`
}

// Actions controls package operations.
type Actions struct {
	Confirm    bool   `toml:"confirm"`     // ask before install, update, remove, reinstall
	Greedy     bool   `toml:"greedy"`      // Update all also upgrades auto-updating casks
	ExportPath string `toml:"export_path"` // where "Export installed" writes the Brewfile
}

// Vulns controls `brew vulns` scans.
type Vulns struct {
	MinSeverity string `toml:"min_severity"` // see Severities
}

// Cache controls bbrew's own cache.
type Cache struct {
	APITTLHours int `toml:"api_ttl_hours"` // how long the downloaded formula/cask lists are reused
}

// Config is the whole settings file.
type Config struct {
	View       View       `toml:"view"`
	Startup    Startup    `toml:"startup"`
	ReleaseAge ReleaseAge `toml:"release_age"`
	Actions    Actions    `toml:"actions"`
	Vulns      Vulns      `toml:"vulns"`
	Cache      Cache      `toml:"cache"`
}

// Default returns the settings used when the file does not set them.
func Default() Config {
	return Config{
		View: View{
			Sort: SortModes[0], Filter: StatusFilters[0], Kind: KindFilters[0],
			Borders: true, Details: true, Log: true, LogHeight: 5,
		},
		Startup: Startup{BrewUpdate: true, CheckBbrewUpdate: true, DiskScan: true},
		ReleaseAge: ReleaseAge{
			Mode: ReleaseAgeModes[0], Casks: true, RecentHours: 24, FreshHours: 1,
		},
		Actions: Actions{Confirm: true, ExportPath: "~/Brewfile"},
		Vulns:   Vulns{MinSeverity: Severities[0]},
		Cache:   Cache{APITTLHours: 24},
	}
}

// Limits for the numeric settings.
const (
	MinLogHeight = 2
	MaxLogHeight = 50
	MaxHours     = 24 * 365
)

// Normalize replaces invalid values with defaults and clamps numbers, so the
// rest of the app can trust a loaded Config.
func (c *Config) Normalize() {
	d := Default()
	enum := func(v *string, allowed []string, def string) {
		*v = strings.ToLower(strings.TrimSpace(*v))
		if !slices.Contains(allowed, *v) {
			*v = def
		}
	}
	enum(&c.View.Sort, SortModes, d.View.Sort)
	enum(&c.View.Filter, StatusFilters, d.View.Filter)
	enum(&c.View.Kind, KindFilters, d.View.Kind)
	enum(&c.ReleaseAge.Mode, ReleaseAgeModes, d.ReleaseAge.Mode)
	enum(&c.Vulns.MinSeverity, Severities, d.Vulns.MinSeverity)

	c.View.LogHeight = min(max(c.View.LogHeight, MinLogHeight), MaxLogHeight)
	c.ReleaseAge.RecentHours = min(max(c.ReleaseAge.RecentHours, 0), MaxHours)
	c.ReleaseAge.FreshHours = min(max(c.ReleaseAge.FreshHours, 0), c.ReleaseAge.RecentHours)
	if c.Cache.APITTLHours < 1 {
		c.Cache.APITTLHours = d.Cache.APITTLHours
	}
	c.Cache.APITTLHours = min(c.Cache.APITTLHours, MaxHours)
	if strings.TrimSpace(c.Actions.ExportPath) == "" {
		c.Actions.ExportPath = d.Actions.ExportPath
	}
}

// RecentWindow is RecentHours as a duration (0 turns the warnings off).
func (r ReleaseAge) RecentWindow() time.Duration { return time.Duration(r.RecentHours) * time.Hour }

// FreshWindow is FreshHours as a duration.
func (r ReleaseAge) FreshWindow() time.Duration { return time.Duration(r.FreshHours) * time.Hour }

// APITTL is APITTLHours as a duration.
func (c Cache) APITTL() time.Duration { return time.Duration(c.APITTLHours) * time.Hour }

// ExpandedExportPath is ExportPath with a leading ~ replaced by the home
// directory.
func (a Actions) ExpandedExportPath() (string, error) {
	return expandHome(a.ExportPath)
}

func expandHome(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~")), nil
}

// Dir returns the bbrew config directory: $XDG_CONFIG_HOME/bbrew, or
// ~/.config/bbrew. (Not ~/Library/Application Support on macOS: a terminal
// tool's settings belong where the user keeps their other dotfiles.)
func Dir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(dir) {
		return filepath.Join(dir, "bbrew")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".config", "bbrew")
	}
	return filepath.Join(home, ".config", "bbrew")
}

// Path returns the settings file path.
func Path() string { return filepath.Join(Dir(), "config.toml") }

// Load reads the settings file at path. A missing file gives the defaults
// and no error; a broken one gives the defaults and the error.
func Load(path string) (Config, error) {
	c := Default()
	data, err := os.ReadFile(path) // #nosec G304 -- the user's own config file
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if _, err := toml.Decode(string(data), &c); err != nil {
		return Default(), fmt.Errorf("%s: %w", path, err)
	}
	c.Normalize()
	return c, nil
}

// header is written at the top of a saved settings file.
const header = `# Bold Brew settings. Edit here or press o in bbrew.
# Missing keys use their defaults; invalid values are replaced on load.
#
# [view]         sort: name|downloads|rank|size  filter: none|installed|outdated|leaves|new
#                kind: all|formulae|casks
# [release_age]  mode: selected (outdated + the selected package) | outdated | off
#                casks: look up cask dates from the download's Last-Modified header
#                recent_hours: flag versions younger than this and warn before upgrading
#                fresh_hours: flag versions younger than this as very fresh
#                hold_in_update_all: leave recent versions out of "Update all outdated"
# [actions]      confirm: ask before install/update/remove/reinstall
#                greedy: Update all also upgrades casks that update themselves
# [vulns]        min_severity: any|low|medium|high|critical
# [cache]        api_ttl_hours: reuse the downloaded formula/cask lists this long

`

// Save writes c to path, creating the directory.
func Save(path string, c Config) error {
	var buf bytes.Buffer
	buf.WriteString(header)
	if err := toml.NewEncoder(&buf).Encode(c); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	// Write then rename, so a crash never leaves a half-written file.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

var (
	mu      sync.RWMutex
	current = Default()
)

// Get returns the settings in effect.
func Get() Config {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// Set replaces the settings in effect (it does not save them).
func Set(c Config) {
	c.Normalize()
	mu.Lock()
	current = c
	mu.Unlock()
}
