package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"bbrew/internal/config"
	"bbrew/internal/models"
	"bbrew/internal/ui/components"
)

// Release date lookups.
//
// Formulae: the bottle's OCI image index on GHCR carries
// org.opencontainers.image.created, the time the bottle was built and
// published, which is when `brew upgrade` could first install the version.
// Casks have no bottle; the download's Last-Modified header is the closest
// thing, and not every server sends one.
//
// Dates of a given version never change, so results are cached on disk.
// Failed lookups are retried after releaseRetryAfter.

const (
	releaseCacheFile   = "release_dates.json"
	releaseRetryAfter  = 6 * time.Hour
	releaseHTTPTimeout = 15 * time.Second
	releaseWorkers     = 4
	ghcrPrefix         = "https://ghcr.io/v2/"
	// ghcrAnonymousToken is the bearer token Homebrew itself sends for
	// public GHCR images (base64 of "@"); it saves a token request.
	ghcrAnonymousToken = "QQ=="
	ociIndexMediaType  = "application/vnd.oci.image.index.v1+json"
	ociCreatedKey      = "org.opencontainers.image.created"
)

// releaseEntry is one cached lookup result.
type releaseEntry struct {
	Time    time.Time `json:"time,omitzero"`
	Source  string    `json:"source,omitempty"`
	Checked time.Time `json:"checked"`
}

// ReleaseAgeService looks up and caches release dates.
type ReleaseAgeService struct {
	mu       sync.Mutex
	entries  map[string]releaseEntry // by releaseKey
	pending  map[string]bool
	queue    chan releaseJob
	client   *http.Client
	onUpdate func() // called (on a worker goroutine) after each lookup
	dirty    bool
	persist  bool // save to the disk cache (off in tests)
	startup  sync.Once

	// lookup functions, replaceable in tests
	formulaDate func(ctx context.Context, f *models.Formula) (time.Time, error)
	caskDate    func(ctx context.Context, c *models.Cask) (time.Time, error)
}

type releaseJob struct {
	key string
	pkg models.Package
}

// NewReleaseAgeService creates the service and loads the disk cache.
func NewReleaseAgeService() *ReleaseAgeService {
	s := &ReleaseAgeService{
		entries: make(map[string]releaseEntry),
		pending: make(map[string]bool),
		queue:   make(chan releaseJob, 4096),
		client:  &http.Client{Timeout: releaseHTTPTimeout},
		persist: true,
	}
	s.formulaDate = s.ghcrCreated
	s.caskDate = s.lastModified
	if data := readCacheFileWithTTL(releaseCacheFile, 2, 30*24*time.Hour); data != nil {
		_ = json.Unmarshal(data, &s.entries)
	}
	return s
}

// SetUpdateFunc sets what to call after each finished lookup.
func (s *ReleaseAgeService) SetUpdateFunc(f func()) { s.onUpdate = f }

// releaseKey identifies a package version.
func releaseKey(pkg models.Package) string {
	return string(pkg.Type) + ":" + pkg.Name + "@" + releaseVersion(pkg)
}

// releaseVersion is the version a lookup is for: for formulae the bottle
// tag (version, revision and rebuild), so a rebuilt bottle is a new release.
func releaseVersion(pkg models.Package) string {
	if pkg.Formula != nil {
		return bottleTag(pkg.Formula)
	}
	return pkg.Version
}

// lookupable reports whether pkg can have a release date under cfg.
func lookupable(pkg models.Package, cfg config.ReleaseAge) bool {
	if cfg.Mode == "off" {
		return false
	}
	switch {
	case pkg.Formula != nil:
		return strings.HasPrefix(pkg.Formula.Bottle.Stable.RootURL, ghcrPrefix)
	case pkg.Cask != nil:
		return cfg.Casks && pkg.Cask.URL != "" && pkg.Version != "latest"
	}
	return false
}

// Get returns what is known about pkg's release date.
func (s *ReleaseAgeService) Get(pkg models.Package) models.ReleaseInfo {
	info := models.ReleaseInfo{Version: pkg.Version}
	if !lookupable(pkg, config.Get().ReleaseAge) {
		return info
	}
	key := releaseKey(pkg)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending[key] {
		info.State = models.ReleasePending
		return info
	}
	e, ok := s.entries[key]
	switch {
	case !ok:
		info.State = models.ReleaseNotLooked
	case e.Time.IsZero():
		info.State = models.ReleaseUnknown
	default:
		info.State, info.Time, info.Source = models.ReleaseKnown, e.Time, e.Source
	}
	return info
}

// Request queues lookups for the packages that have no (fresh) result yet.
func (s *ReleaseAgeService) Request(pkgs []models.Package) {
	cfg := config.Get().ReleaseAge
	s.startup.Do(s.startWorkers)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, pkg := range pkgs {
		if !lookupable(pkg, cfg) {
			continue
		}
		key := releaseKey(pkg)
		if s.pending[key] {
			continue
		}
		if e, ok := s.entries[key]; ok && (!e.Time.IsZero() || time.Since(e.Checked) < releaseRetryAfter) {
			continue
		}
		select {
		case s.queue <- releaseJob{key, pkg}:
			s.pending[key] = true
		default: // queue full; a later Request retries
			return
		}
	}
}

// AnyPending reports whether a lookup for any of pkgs is queued or running.
func (s *ReleaseAgeService) AnyPending(pkgs []models.Package) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, pkg := range pkgs {
		if s.pending[releaseKey(pkg)] {
			return true
		}
	}
	return false
}

// Wait blocks until none of pkgs has a lookup pending, or ctx ends.
func (s *ReleaseAgeService) Wait(ctx context.Context, pkgs []models.Package) {
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		if !s.AnyPending(pkgs) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func (s *ReleaseAgeService) startWorkers() {
	for range releaseWorkers {
		go func() {
			for job := range s.queue {
				s.run(job)
			}
		}()
	}
}

// run performs one lookup and records the result.
func (s *ReleaseAgeService) run(job releaseJob) {
	ctx, cancel := context.WithTimeout(context.Background(), releaseHTTPTimeout)
	defer cancel()
	var t time.Time
	var source string
	var err error
	switch {
	case job.pkg.Formula != nil:
		t, err = s.formulaDate(ctx, job.pkg.Formula)
		source = "bottle"
	case job.pkg.Cask != nil:
		t, err = s.caskDate(ctx, job.pkg.Cask)
		source = "download"
	}
	// A date from the future or before Homebrew existed is a server quirk.
	if err != nil || t.After(time.Now().Add(time.Hour)) || t.Year() < 2009 {
		t, source = time.Time{}, ""
	}

	s.mu.Lock()
	delete(s.pending, job.key)
	s.entries[job.key] = releaseEntry{Time: t.UTC(), Source: source, Checked: time.Now().UTC()}
	s.dirty = true
	idle := len(s.pending) == 0
	s.mu.Unlock()

	if idle {
		s.save()
	}
	if s.onUpdate != nil {
		s.onUpdate()
	}
}

// save writes the cache when it changed.
func (s *ReleaseAgeService) save() {
	s.mu.Lock()
	if !s.dirty || !s.persist {
		s.mu.Unlock()
		return
	}
	data, err := json.Marshal(s.entries)
	s.dirty = false
	s.mu.Unlock()
	if err == nil && ensureCacheDir() == nil {
		writeCacheFile(releaseCacheFile, data)
	}
}

// bottleTag is the GHCR tag of a formula's current bottle: the version,
// "_<revision>" when revised and "-<rebuild>" when rebuilt (see Homebrew's
// GitHubPackages.version_rebuild).
func bottleTag(f *models.Formula) string {
	tag := f.Versions.Stable
	if f.Revision > 0 {
		tag += "_" + strconv.Itoa(f.Revision)
	}
	if r := f.Bottle.Stable.Rebuild; r > 0 {
		tag += "-" + strconv.Itoa(r)
	}
	return tag
}

// ghcrImageName maps a formula name to its image name, like Homebrew's
// GitHubPackages.image_formula_name: "@" becomes "/" and "+" becomes "x".
func ghcrImageName(name string) string {
	return strings.NewReplacer("@", "/", "+", "x").Replace(name)
}

// ghcrManifestURL is the image index URL of f's current bottle.
func ghcrManifestURL(f *models.Formula) string {
	root := strings.TrimSuffix(f.Bottle.Stable.RootURL, "/")
	return root + "/" + ghcrImageName(f.Name) + "/manifests/" + bottleTag(f)
}

// ghcrCreated fetches the bottle's creation time from GHCR.
func (s *ReleaseAgeService) ghcrCreated(ctx context.Context, f *models.Formula) (time.Time, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ghcrManifestURL(f), nil)
	if err != nil {
		return time.Time{}, err
	}
	req.Header.Set("Authorization", "Bearer "+ghcrAnonymousToken)
	req.Header.Set("Accept", ociIndexMediaType)
	resp, err := s.client.Do(req) // #nosec G107 -- URL built from the Homebrew API's bottle root
	if err != nil {
		return time.Time{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return time.Time{}, fmt.Errorf("GHCR: %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return time.Time{}, err
	}
	return parseOCICreated(data)
}

// parseOCICreated reads the creation time from an OCI image index: its own
// annotation, or else the newest of its manifests'.
func parseOCICreated(data []byte) (time.Time, error) {
	var index struct {
		Annotations map[string]string `json:"annotations"`
		Manifests   []struct {
			Annotations map[string]string `json:"annotations"`
		} `json:"manifests"`
	}
	if err := json.Unmarshal(data, &index); err != nil {
		return time.Time{}, err
	}
	if t, err := time.Parse(time.RFC3339, index.Annotations[ociCreatedKey]); err == nil {
		return t, nil
	}
	var newest time.Time
	for _, m := range index.Manifests {
		if t, err := time.Parse(time.RFC3339, m.Annotations[ociCreatedKey]); err == nil && t.After(newest) {
			newest = t
		}
	}
	if newest.IsZero() {
		return newest, errors.New("no creation time in the image index")
	}
	return newest, nil
}

// lastModified asks the cask's download server for Last-Modified: a HEAD
// request, or a one byte GET when HEAD is refused.
func (s *ReleaseAgeService) lastModified(ctx context.Context, c *models.Cask) (time.Time, error) {
	if !strings.HasPrefix(c.URL, "https://") && !strings.HasPrefix(c.URL, "http://") {
		return time.Time{}, errors.New("not an HTTP download")
	}
	try := func(method string) (time.Time, error) {
		req, err := http.NewRequestWithContext(ctx, method, c.URL, nil)
		if err != nil {
			return time.Time{}, err
		}
		req.Header.Set("User-Agent", "bbrew/"+AppVersion)
		if method == http.MethodGet {
			req.Header.Set("Range", "bytes=0-0")
		}
		resp, err := s.client.Do(req) // #nosec G107 -- the cask's own download URL
		if err != nil {
			return time.Time{}, err
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		_ = resp.Body.Close()
		if resp.StatusCode >= 400 {
			return time.Time{}, fmt.Errorf("%s: %s", method, resp.Status)
		}
		return http.ParseTime(resp.Header.Get("Last-Modified"))
	}
	t, err := try(http.MethodHead)
	if err != nil {
		t, err = try(http.MethodGet)
	}
	return t, err
}

// releaseLevel classifies a release as components.ReleaseFresh,
// ReleaseRecent or ReleaseOld (also when the date is unknown).
func releaseLevel(info models.ReleaseInfo, cfg config.ReleaseAge, now time.Time) int {
	if info.State != models.ReleaseKnown {
		return components.ReleaseOld
	}
	age := info.Age(now)
	switch {
	case age < cfg.FreshWindow():
		return components.ReleaseFresh
	case age < cfg.RecentWindow():
		return components.ReleaseRecent
	}
	return components.ReleaseOld
}
