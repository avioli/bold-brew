package services

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"

	"bbrew/internal/models"
)

// DiskUsageService measures the disk space of installed packages and of the
// Homebrew download cache by walking the file system. It never needs root:
// whatever cannot be read is counted in Unreadable and left out of the sizes.
type DiskUsageService struct {
	mu      sync.RWMutex
	usage   map[string]*models.DiskUsage // by usageKey
	totals  models.DiskTotals
	scanned bool
	gen     int // bumped by every Scan; older scans drop their results
	homeDir string

	fullDiskAccess func() bool // overrides the check, for tests
	fdaOnce        sync.Once
	fda            bool
	getDirs        func() brewDirs
	scanLock       sync.Mutex // one scan at a time
}

func NewDiskUsageService() *DiskUsageService {
	home, _ := os.UserHomeDir()
	return &DiskUsageService{
		usage:   map[string]*models.DiskUsage{},
		homeDir: home,
		getDirs: getBrewDirs,
	}
}

func usageKey(t models.PackageType, name string) string {
	return string(t) + ":" + name
}

// Get returns the measured usage of a package, or nil if it has none (yet).
func (s *DiskUsageService) Get(t models.PackageType, name string) *models.DiskUsage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.usage[usageKey(t, name)]
}

// Totals returns the totals of the last finished scan, and whether one has
// finished.
func (s *DiskUsageService) Totals() (models.DiskTotals, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.totals, s.scanned
}

// Scan measures the given packages (only installed formulae and casks take
// space; cache files are found for any package) and the whole cache. It
// blocks; run it in the background. It returns false if a newer scan
// replaced it.
//
// Results show up as they are measured: each installed package is visible
// to Get as soon as it is done (earlier results stay until replaced), and
// progress, if not nil, is called after each one. Totals change only when
// the whole scan has finished.
func (s *DiskUsageService) Scan(pkgs []models.Package, progress func()) bool {
	s.mu.Lock()
	s.gen++
	gen := s.gen
	s.mu.Unlock()

	s.scanLock.Lock()
	defer s.scanLock.Unlock()
	if s.stale(gen) {
		return false
	}

	dirs := s.getDirs()
	usage := map[string]*models.DiskUsage{}
	get := func(t models.PackageType, name string) *models.DiskUsage {
		k := usageKey(t, name)
		if usage[k] == nil {
			usage[k] = &models.DiskUsage{}
		}
		return usage[k]
	}
	var totals models.DiskTotals

	// Cache first (it only stats a few hundred files), so each package is
	// complete when it is published below and never changes afterwards.
	// A separate sizer, because downloads are reached through several
	// symlinks and each must count once.
	if dirs.Cache != "" {
		cz := newSizer()
		attributed := s.measureCache(cz, dirs.Cache, get)
		totals.Cache = newSizer().size(dirs.Cache)
		totals.CacheOther = max(totals.Cache-attributed, 0)
		totals.Unreadable += cz.unreadable
	}

	// Installed files. One sizer for all of them, so a file hard linked
	// into two places counts once.
	z := newSizer()
	for i := range pkgs {
		if s.stale(gen) {
			return false
		}
		p := &pkgs[i]
		if !p.LocallyInstalled {
			continue
		}
		var u *models.DiskUsage
		switch {
		case p.Type == models.PackageTypeFormula && p.Formula != nil:
			u = get(p.Type, p.Name)
			s.measureFormula(z, dirs, p.Formula, u)
			totals.Formulae += u.Installed
			totals.Old += u.Old
		case p.Type == models.PackageTypeCask && p.Cask != nil:
			u = get(p.Type, p.Name)
			s.measureCask(z, dirs, p.Cask, u)
			totals.Casks += u.Installed
			totals.AppData += u.AppData
		default:
			continue
		}
		if !s.publish(gen, usageKey(p.Type, p.Name), u) {
			return false
		}
		if progress != nil {
			progress()
		}
	}
	totals.Unreadable += z.unreadable

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gen != gen {
		return false
	}
	// The complete map also drops packages that are gone since the last scan.
	s.usage, s.totals, s.scanned = usage, totals, true
	return true
}

// publish makes one finished package visible to Get, unless a newer scan
// started.
func (s *DiskUsageService) publish(gen int, key string, u *models.DiskUsage) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gen != gen {
		return false
	}
	s.usage[key] = u // s.usage is only touched under s.mu
	return true
}

func (s *DiskUsageService) stale(gen int) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.gen != gen
}

// measureFormula sizes every installed version (keg) in the Cellar. The
// linked keg, or the newest installed one, is current; the rest are old,
// which `brew cleanup` would remove.
func (s *DiskUsageService) measureFormula(z *sizer, dirs brewDirs, f *models.Formula, u *models.DiskUsage) {
	if dirs.Cellar == "" {
		return
	}
	rack := filepath.Join(dirs.Cellar, f.Name)
	entries, err := os.ReadDir(rack)
	if err != nil {
		if !os.IsNotExist(err) {
			u.Unreadable++
		}
		return
	}
	current := f.LinkedKeg
	if current == "" && len(f.Installed) > 0 {
		current = f.Installed[len(f.Installed)-1].Version
	}
	// Measure the current keg first, so files hard linked between versions
	// count towards it rather than towards an old one.
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Name() == current && entries[j].Name() != current
	})
	before := z.unreadable
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		n := z.size(filepath.Join(rack, e.Name()))
		label := "old"
		if e.Name() == current || len(entries) == 1 {
			label = "current"
		} else {
			u.Old += n
		}
		u.Installed += n
		u.InstalledPaths = append(u.InstalledPaths, models.SizedPath{
			Path: filepath.Join(rack, e.Name()), Bytes: n, Label: label,
		})
	}
	u.Unreadable += z.unreadable - before
}

// caskInstallArtifacts are the cask artifact kinds whose "target" is a path
// the cask installed (binaries, completions and manpages are symlinks into
// the app or the Caskroom and are not counted).
var caskInstallArtifacts = map[string]bool{
	"app": true, "suite": true, "artifact": true, "font": true, "prefpane": true,
	"qlplugin": true, "mdimporter": true, "colorpicker": true, "dictionary": true,
	"service": true, "audio_unit_plugin": true, "vst_plugin": true, "vst3_plugin": true,
	"screen_saver": true, "input_method": true, "internet_plugin": true, "keyboard_layout": true,
}

// measureCask sizes the Caskroom entry, the installed artifacts, the paths
// the uninstall stanza deletes (where .pkg installed casks put their files)
// and, as app data, the paths the zap stanza trashes.
func (s *DiskUsageService) measureCask(z *sizer, dirs brewDirs, c *models.Cask, u *models.DiskUsage) {
	var installed, appData []string
	if dirs.Caskroom != "" {
		installed = append(installed, filepath.Join(dirs.Caskroom, c.Token))
	}
	for _, a := range c.Artifacts {
		for kind, v := range a {
			switch {
			case caskInstallArtifacts[kind]:
				if t, ok := a["target"].(string); ok {
					installed = append(installed, t) // literal: font names may contain [ ]
				}
			case kind == "pkg":
				u.PkgFiles = true
			case kind == "uninstall":
				installed = append(installed, s.stanzaPaths(v, u)...)
			case kind == "zap":
				appData = append(appData, s.stanzaPaths(v, u)...)
			}
		}
	}

	before := z.unreadable
	u.InstalledPaths, u.Installed = z.sizePaths(installed)
	u.AppDataPaths, u.AppData = z.sizePaths(appData)
	u.Unreadable += z.unreadable - before
}

// stanzaPaths returns the delete and trash paths of an uninstall or zap
// stanza ([{"delete": ..., "trash": [...]}]), with ~ and globs expanded.
// Privacy-protected locations are recorded in u.Skipped instead.
func (s *DiskUsageService) stanzaPaths(v any, u *models.DiskUsage) []string {
	var out []string
	list, _ := v.([]any)
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		for _, key := range []string{"delete", "trash"} {
			for _, p := range anyStrings(m[key]) {
				p = s.expandHome(p)
				if !filepath.IsAbs(p) {
					continue
				}
				if s.protected(p) {
					u.Skipped = append(u.Skipped, p)
					continue
				}
				if strings.ContainsAny(p, "*?") {
					matches, _ := filepath.Glob(p)
					out = append(out, matches...)
				} else {
					out = append(out, p)
				}
			}
		}
	}
	return out
}

// anyStrings converts a JSON string or list of strings.
func anyStrings(v any) []string {
	switch t := v.(type) {
	case string:
		return []string{t}
	case []any:
		var out []string
		for _, e := range t {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func (s *DiskUsageService) expandHome(p string) string {
	if s.homeDir != "" && (p == "~" || strings.HasPrefix(p, "~/")) {
		return filepath.Join(s.homeDir, p[1:])
	}
	return p
}

// appDataDirs (under the home directory) hold other apps' data. Since
// macOS 14, reading them asks "would like to access data from other apps"
// even with Full Disk Access, so they are never read.
var appDataDirs = []string{"Library/Containers", "Library/Group Containers"}

// privacyDirs (under the home directory) need Full Disk Access or a
// per-folder grant: without one, reading them prompts (Desktop, Documents,
// ...) or fails. They are read only with Full Disk Access.
var privacyDirs = []string{
	"Desktop", "Documents", "Downloads", "Pictures", "Movies", "Music",
	"Library/Mobile Documents", "Library/Mail", "Library/Messages", "Library/Safari",
	"Library/Calendars", "Library/Reminders", "Library/HomeKit", "Library/Photos",
	"Library/Application Support/AddressBook", "Library/Application Support/CallHistoryDB",
	"Library/Application Support/com.apple.TCC",
}

// protected reports whether reading p could make macOS ask for permission.
func (s *DiskUsageService) protected(p string) bool {
	if s.homeDir == "" {
		return false
	}
	rel, err := filepath.Rel(s.homeDir, p)
	if err != nil || strings.HasPrefix(rel, "..") {
		return false
	}
	under := func(dirs []string) bool {
		for _, d := range dirs {
			if rel == d || strings.HasPrefix(rel, d+string(filepath.Separator)) {
				return true
			}
		}
		return false
	}
	if under(appDataDirs) {
		return true
	}
	return under(privacyDirs) && !s.hasFullDiskAccess()
}

// hasFullDiskAccess reports whether this process (that is, the terminal
// running it) has Full Disk Access, checked once. Opening the user TCC
// database needs it, and failing to open it never prompts.
func (s *DiskUsageService) hasFullDiskAccess() bool {
	s.fdaOnce.Do(func() {
		if s.fullDiskAccess != nil {
			s.fda = s.fullDiskAccess()
			return
		}
		f, err := os.Open(filepath.Join(s.homeDir, "Library/Application Support/com.apple.TCC/TCC.db")) // #nosec G304 - fixed path
		if err == nil {
			_ = f.Close()
			s.fda = true
		}
	})
	return s.fda
}

// measureCache attributes the files in the Homebrew cache to packages by
// name. Formula downloads are $CACHE/<name>--<version> (and
// <name>_bottle_manifest--<version>), cask downloads $CACHE/Cask/<token>--<version>.ext;
// most are symlinks into $CACHE/downloads. It returns the attributed total.
func (s *DiskUsageService) measureCache(z *sizer, cache string, get func(models.PackageType, string) *models.DiskUsage) int64 {
	var total int64
	scan := func(dir string, t models.PackageType) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			name, _, ok := strings.Cut(e.Name(), "--")
			if !ok || name == "" {
				continue
			}
			name = strings.TrimSuffix(name, "_bottle_manifest")
			path := filepath.Join(dir, e.Name())
			target, err := filepath.EvalSymlinks(path)
			if err != nil {
				continue // dangling link
			}
			n := z.size(target)
			if n == 0 {
				continue
			}
			u := get(t, name)
			u.Cache += n
			u.CachePaths = append(u.CachePaths, models.SizedPath{Path: path, Bytes: n})
			total += n
		}
	}
	scan(cache, models.PackageTypeFormula)
	scan(filepath.Join(cache, "Cask"), models.PackageTypeCask)
	return total
}

// sizer adds up allocated sizes like du: symlinks are not followed and a
// file with several hard links counts once per sizer.
type sizer struct {
	seen       map[[2]uint64]bool
	unreadable int
}

func newSizer() *sizer {
	return &sizer{seen: map[[2]uint64]bool{}}
}

// size returns the allocated size of root and everything below it.
func (z *sizer) size(root string) int64 {
	var total int64
	_ = filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			if !os.IsNotExist(err) {
				z.unreadable++
			}
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			if !os.IsNotExist(err) {
				z.unreadable++
			}
			return nil
		}
		total += z.allocated(info)
		return nil
	})
	return total
}

func (z *sizer) allocated(info fs.FileInfo) int64 {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return info.Size()
	}
	if !info.IsDir() && st.Nlink > 1 {
		key := [2]uint64{uint64(st.Dev), st.Ino} // #nosec G115 - device ids are never negative
		if z.seen[key] {
			return 0
		}
		z.seen[key] = true
	}
	return st.Blocks * 512
}

// sizePaths measures paths that exist, skipping duplicates and paths inside
// another measured path, largest first.
func (z *sizer) sizePaths(paths []string) ([]models.SizedPath, int64) {
	clean := make([]string, 0, len(paths))
	for _, p := range paths {
		clean = append(clean, filepath.Clean(p))
	}
	sort.Strings(clean)

	var out []models.SizedPath
	var total int64
	last := ""
	for _, p := range clean {
		if last != "" && (p == last || strings.HasPrefix(p, last+string(filepath.Separator))) {
			continue
		}
		if _, err := os.Lstat(p); err != nil {
			if !os.IsNotExist(err) {
				z.unreadable++
			}
			continue
		}
		last = p
		n := z.size(p)
		out = append(out, models.SizedPath{Path: p, Bytes: n})
		total += n
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Bytes > out[j].Bytes })
	return out, total
}
