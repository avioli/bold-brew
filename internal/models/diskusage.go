package models

// SizedPath is one measured location on disk.
type SizedPath struct {
	Path  string
	Bytes int64
	Label string // e.g. a version, "current" or "old"
}

// DiskUsage is the disk space a package takes. All sizes are allocated
// bytes (like du), with hard links counted once.
type DiskUsage struct {
	Installed int64 // formula kegs (all versions); cask Caskroom entry plus installed artifacts
	Old       int64 // formula kegs other than the current one (part of Installed)
	Cache     int64 // downloads in the Homebrew cache
	AppData   int64 // cask zap paths: settings, caches and support files the app created

	InstalledPaths []SizedPath
	CachePaths     []SizedPath
	AppDataPaths   []SizedPath

	Skipped    []string // paths left out to avoid macOS privacy prompts
	Unreadable int      // files or directories that could not be read without more permissions
	PkgFiles   bool     // the cask runs a .pkg installer whose files are not measured
}

// DiskTotals sums disk usage over all packages.
type DiskTotals struct {
	Formulae   int64 // Cellar, all versions
	Old        int64 // old formula versions (part of Formulae)
	Casks      int64 // Caskroom and installed cask artifacts
	AppData    int64 // cask zap paths
	Cache      int64 // whole Homebrew cache
	CacheOther int64 // cache not attributed to a package (API data, orphans, ...)
	Unreadable int
}
