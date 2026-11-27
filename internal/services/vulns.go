package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"bbrew/internal/config"
	"bbrew/internal/models"
)

// vulnResult holds the raw JSON scan result for a single formula in the
// legacy (brew-vulns tap) format: a top-level array of these.
type vulnResult struct {
	Name            string                 `json:"name"`
	Version         string                 `json:"version"`
	Vulnerabilities []models.Vulnerability `json:"vulnerabilities"`
}

// vulnReport is the `brew vulns --json` output of the built-in command
// (Homebrew 6+): an object listing findings per formula.
type vulnReport struct {
	Findings []struct {
		Formula         string                 `json:"formula"`
		Vulnerabilities []models.Vulnerability `json:"vulnerabilities"`
	} `json:"findings"`
	Complete bool `json:"complete"`
}

// parseVulnsJSON extracts the open vulnerabilities from `brew vulns --json`
// output, in either the current object format or the legacy array format.
// ok is false when the output could not be parsed.
func parseVulnsJSON(data []byte) (vulns []models.Vulnerability, ok bool) {
	vulns = make([]models.Vulnerability, 0)
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return vulns, false
	}
	if data[0] == '[' {
		var results []vulnResult
		if err := json.Unmarshal(data, &results); err != nil {
			return vulns, false
		}
		for _, r := range results {
			vulns = append(vulns, r.Vulnerabilities...)
		}
		return vulns, true
	}
	var report vulnReport
	if err := json.Unmarshal(data, &report); err != nil {
		return vulns, false
	}
	for _, f := range report.Findings {
		vulns = append(vulns, f.Vulnerabilities...)
	}
	return vulns, true
}

// VulnsServiceInterface defines the contract for vulnerability scanning.
type VulnsServiceInterface interface {
	IsAvailable() bool
	ScanPackage(name string, output io.Writer) ([]models.Vulnerability, error)
	GetCachedVulns(name string) ([]models.Vulnerability, bool)
}

// VulnsService wraps the `brew vulns` command for vulnerability scanning.
type VulnsService struct {
	mu        sync.RWMutex
	available *bool // nil = not checked yet
	cache     map[string][]models.Vulnerability
}

// errVulnsUnavailable is returned when `brew vulns` does not exist.
var errVulnsUnavailable = errors.New("brew vulns is not available: it is built into Homebrew 6+, run `brew update`")

var NewVulnsService = func() VulnsServiceInterface {
	return &VulnsService{
		cache: make(map[string][]models.Vulnerability),
	}
}

// IsAvailable checks if the `brew vulns` command exists (it is built into
// Homebrew since 6.0).
func (s *VulnsService) IsAvailable() bool {
	s.mu.RLock()
	if s.available != nil {
		avail := *s.available
		s.mu.RUnlock()
		return avail
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.available != nil {
		return *s.available
	}

	cmd := brewCommand("vulns", "--help")
	err := cmd.Run()
	avail := err == nil
	s.available = &avail
	return avail
}

// ScanPackage runs `brew vulns <name>` streaming real-time output to the panel,
// then fetches JSON for structured caching. This avoids blocking the UI
// while the OSV database is queried.
func (s *VulnsService) ScanPackage(name string, output io.Writer) ([]models.Vulnerability, error) {
	if !s.IsAvailable() {
		return nil, errVulnsUnavailable
	}

	// Phase 1: Stream human-readable output in real-time so the user sees progress
	sevArgs := severityArgs()
	cmd := brewCommand(append([]string{"vulns", name, "-m", "0"}, sevArgs...)...) // #nosec G204
	scanErr := ExecuteCommand(cmd, output)
	// Exit code 1 = vulnerabilities found (not a failure for us)
	// Exit code 2 = scan error. We still continue to try JSON caching.
	if scanErr != nil {
		fmt.Fprintf(output, "\n")
	}

	// Phase 2: Fetch JSON for structured caching (fast — brew caches the OSV response)
	jsonCmd := brewCommand(append([]string{"vulns", name, "--json"}, sevArgs...)...) // #nosec G204
	var jsonBuf bytes.Buffer
	jsonCmd.Stdout = &jsonBuf
	_ = jsonCmd.Run() // best-effort; if this fails we just skip caching

	vulns, ok := parseVulnsJSON(jsonBuf.Bytes())
	if !ok {
		// Unknown result: do not cache "no vulnerabilities".
		return nil, fmt.Errorf("could not read the brew vulns results")
	}

	s.mu.Lock()
	s.cache[name] = vulns
	s.mu.Unlock()

	return vulns, nil
}

// GetCachedVulns returns cached vulnerability data for a package, if available.
func (s *VulnsService) GetCachedVulns(name string) ([]models.Vulnerability, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	vulns, ok := s.cache[name]
	return vulns, ok
}

// severityArgs passes the vulns.min_severity setting to brew vulns.
func severityArgs() []string {
	if sev := config.Get().Vulns.MinSeverity; sev != "" && sev != "any" {
		return []string{"--severity=" + sev}
	}
	return nil
}

func pluralY(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}
