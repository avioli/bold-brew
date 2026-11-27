package services

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"bbrew/internal/config"
	"bbrew/internal/models"
	"bbrew/internal/ui/components"
)

func testFormula(name, version string, revision, rebuild int) *models.Formula {
	f := &models.Formula{Name: name, Revision: revision}
	f.Versions.Stable = version
	f.Bottle.Stable.Rebuild = rebuild
	f.Bottle.Stable.RootURL = "https://ghcr.io/v2/homebrew/core"
	return f
}

func newTestReleaseAge() *ReleaseAgeService {
	return &ReleaseAgeService{
		entries: make(map[string]releaseEntry),
		pending: make(map[string]bool),
		queue:   make(chan releaseJob, 64),
		client:  http.DefaultClient,
	}
}

func TestBottleTagAndManifestURL(t *testing.T) {
	cases := []struct {
		f    *models.Formula
		want string
	}{
		{testFormula("jq", "1.8.1", 0, 0), "https://ghcr.io/v2/homebrew/core/jq/manifests/1.8.1"},
		{testFormula("openssl@3", "3.6.0", 1, 0), "https://ghcr.io/v2/homebrew/core/openssl/3/manifests/3.6.0_1"},
		{testFormula("libxml++", "5.4", 0, 2), "https://ghcr.io/v2/homebrew/core/libxmlxx/manifests/5.4-2"},
		{testFormula("a", "1", 3, 1), "https://ghcr.io/v2/homebrew/core/a/manifests/1_3-1"},
	}
	for _, c := range cases {
		if got := ghcrManifestURL(c.f); got != c.want {
			t.Errorf("got %s, want %s", got, c.want)
		}
	}
}

func TestParseOCICreated(t *testing.T) {
	want := time.Date(2026, 6, 20, 15, 46, 58, 0, time.UTC)
	got, err := parseOCICreated([]byte(`{"annotations":{"org.opencontainers.image.created":"2026-06-20T15:46:58Z"}}`))
	if err != nil || !got.Equal(want) {
		t.Errorf("index annotation: %v, %v", got, err)
	}
	got, err = parseOCICreated([]byte(`{"manifests":[
		{"annotations":{"org.opencontainers.image.created":"2026-06-19T00:00:00Z"}},
		{"annotations":{"org.opencontainers.image.created":"2026-06-20T15:46:58Z"}}]}`))
	if err != nil || !got.Equal(want) {
		t.Errorf("manifest annotations: %v, %v", got, err)
	}
	if _, err := parseOCICreated([]byte(`{}`)); err == nil {
		t.Error("want an error without annotations")
	}
}

func TestLookupable(t *testing.T) {
	cfg := config.Default().ReleaseAge
	formula := models.NewPackageFromFormula(testFormula("jq", "1", 0, 0))
	tapFormula := models.NewPackageFromFormula(&models.Formula{Name: "x"}) // no GHCR bottle
	cask := models.NewPackageFromCask(&models.Cask{Token: "c", Version: "2", URL: "https://example.com/c.dmg"})
	latest := models.NewPackageFromCask(&models.Cask{Token: "l", Version: "latest", URL: "https://example.com/l.dmg"})

	if !lookupable(formula, cfg) || lookupable(tapFormula, cfg) || !lookupable(cask, cfg) || lookupable(latest, cfg) {
		t.Error("default settings")
	}
	cfg.Casks = false
	if lookupable(cask, cfg) {
		t.Error("casks off")
	}
	cfg.Mode = "off"
	if lookupable(formula, cfg) {
		t.Error("mode off")
	}
}

func TestRequestGetAndRetry(t *testing.T) {
	config.Set(config.Default())
	s := newTestReleaseAge()
	published := time.Now().Add(-3 * time.Hour).UTC().Truncate(time.Second)
	var calls atomic.Int32
	s.formulaDate = func(context.Context, *models.Formula) (time.Time, error) {
		calls.Add(1)
		return published, nil
	}
	s.caskDate = func(context.Context, *models.Cask) (time.Time, error) {
		calls.Add(1)
		return time.Time{}, errors.New("no header")
	}
	done := make(chan struct{}, 8)
	s.SetUpdateFunc(func() { done <- struct{}{} })

	formula := models.NewPackageFromFormula(testFormula("jq", "1.8.1", 0, 0))
	cask := models.NewPackageFromCask(&models.Cask{Token: "c", Version: "2", URL: "https://example.com/c.dmg"})
	if got := s.Get(formula).State; got != models.ReleaseNotLooked {
		t.Fatalf("before: %v", got)
	}
	s.Request([]models.Package{formula, cask})
	for range 2 {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("lookups did not finish")
		}
	}

	info := s.Get(formula)
	if info.State != models.ReleaseKnown || !info.Time.Equal(published) || info.Source != "bottle" {
		t.Errorf("formula: %+v", info)
	}
	if got := s.Get(cask).State; got != models.ReleaseUnknown {
		t.Errorf("cask: %v", got)
	}
	if lvl := releaseLevel(info, config.Get().ReleaseAge, time.Now()); lvl != components.ReleaseRecent {
		t.Errorf("level: %v", lvl)
	}

	// Known and recently failed lookups are not repeated.
	s.Request([]models.Package{formula, cask})
	if s.AnyPending([]models.Package{formula, cask}) || calls.Load() != 2 {
		t.Errorf("repeated lookups: pending=%v calls=%d", s.AnyPending([]models.Package{formula, cask}), calls.Load())
	}

	// A new version (here a rebuilt bottle) is a new lookup.
	rebuilt := models.NewPackageFromFormula(testFormula("jq", "1.8.1", 0, 1))
	if got := s.Get(rebuilt).State; got != models.ReleaseNotLooked {
		t.Errorf("rebuilt: %v", got)
	}
}

func TestReleaseLevel(t *testing.T) {
	cfg := config.Default().ReleaseAge // recent 24h, fresh 1h
	now := time.Now()
	at := func(ago time.Duration) models.ReleaseInfo {
		return models.ReleaseInfo{State: models.ReleaseKnown, Time: now.Add(-ago)}
	}
	if releaseLevel(at(30*time.Minute), cfg, now) != components.ReleaseFresh ||
		releaseLevel(at(5*time.Hour), cfg, now) != components.ReleaseRecent ||
		releaseLevel(at(48*time.Hour), cfg, now) != components.ReleaseOld ||
		releaseLevel(models.ReleaseInfo{State: models.ReleaseUnknown}, cfg, now) != components.ReleaseOld {
		t.Error("levels")
	}
	cfg.RecentHours, cfg.FreshHours = 0, 0
	if releaseLevel(at(time.Minute), cfg, now) != components.ReleaseOld {
		t.Error("0 hours turns warnings off")
	}
}

func TestLastModified(t *testing.T) {
	lm := "Tue, 24 Nov 2026 14:02:39 GMT"
	headRefused := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead && headRefused {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Last-Modified", lm)
		_, _ = w.Write([]byte("x"))
	}))
	defer srv.Close()

	s := newTestReleaseAge()
	want, _ := http.ParseTime(lm)
	for _, refused := range []bool{false, true} {
		headRefused = refused
		got, err := s.lastModified(context.Background(), &models.Cask{URL: srv.URL + "/app.dmg"})
		if err != nil || !got.Equal(want) {
			t.Errorf("head refused=%v: %v, %v", refused, got, err)
		}
	}
	if _, err := s.lastModified(context.Background(), &models.Cask{URL: "ftp://x"}); err == nil {
		t.Error("want an error for a non-HTTP URL")
	}
}

func TestUpgradeArgs(t *testing.T) {
	config.Set(config.Default())
	pkgs := []models.Package{
		models.NewPackageFromFormula(testFormula("jq", "1", 0, 0)),
		models.NewPackageFromCask(&models.Cask{Token: "firefox"}),
		models.NewPackageFromCask(&models.Cask{Token: "jq"}), // clashes with the formula
	}
	if got, want := upgradeArgs(pkgs), []string{"upgrade", "jq", "firefox", "homebrew/cask/jq"}; !slices.Equal(got, want) {
		t.Errorf("got %v", got)
	}
	c := config.Default()
	c.Actions.Greedy = true
	config.Set(c)
	defer config.Set(config.Default())
	if got, want := upgradeArgs(nil), []string{"upgrade", "--greedy"}; !slices.Equal(got, want) {
		t.Errorf("got %v", got)
	}
}
