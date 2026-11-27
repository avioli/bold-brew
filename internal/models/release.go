package models

import "time"

// ReleaseState is how far a release date lookup got.
type ReleaseState int

const (
	ReleaseNotLooked ReleaseState = iota // not requested (or lookups are off)
	ReleasePending                       // lookup running
	ReleaseKnown                         // Time is set
	ReleaseUnknown                       // lookup done, no date available
)

// ReleaseInfo is when a package version was published.
type ReleaseInfo struct {
	State   ReleaseState
	Version string    // the version the date is for
	Time    time.Time // when it was published (State == ReleaseKnown)
	// Source says where Time comes from: "bottle" (the GHCR bottle was
	// created) or "download" (the cask download's Last-Modified header).
	Source string
}

// Age is how long ago the version was published.
func (r ReleaseInfo) Age(now time.Time) time.Duration { return now.Sub(r.Time) }
