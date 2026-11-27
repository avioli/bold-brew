package components

import (
	"fmt"
	"time"

	"bbrew/internal/models"
)

// Release levels, see SetReleaseFunc.
const (
	ReleaseOld    = 0 // older than the recent threshold, or no date
	ReleaseRecent = 1 // younger than the recent threshold
	ReleaseFresh  = 2 // younger than the very fresh threshold
)

// SetReleaseFunc sets how to look up when a package's version was published
// and how recent that is (ReleaseOld, ReleaseRecent or ReleaseFresh), for
// the "Released" line.
func (d *Details) SetReleaseFunc(f func(pkg *models.Package) (models.ReleaseInfo, int)) {
	d.release = f
}

// releaseBullet is the "Released" line, or "" when there is nothing to say.
func (d *Details) releaseBullet(pkg *models.Package) string {
	if d.release == nil {
		return ""
	}
	info, level := d.release(pkg)
	var value string
	switch info.State {
	case models.ReleasePending:
		value = "[dim]checking...[-:-:-]"
	case models.ReleaseUnknown:
		value = "[dim]unknown[-:-:-]"
	case models.ReleaseKnown:
		local := info.Time.Local()
		value = fmt.Sprintf("%s (%s, %s)", local.Format("2006-01-02 15:04"), FormatAge(time.Since(info.Time)), info.Source)
		switch level {
		case ReleaseFresh:
			value = "[red::b]⚠ " + value + "[-:-:-]"
		case ReleaseRecent:
			value = "[yellow]⚠ " + value + "[-:-:-]"
		}
	default:
		return ""
	}
	return bullet("Released", value)
}

// FormatAge describes how long ago something happened, e.g. "3h ago".
func FormatAge(age time.Duration) string {
	switch {
	case age < time.Minute:
		return "just now"
	case age < time.Hour:
		return fmt.Sprintf("%dm ago", int(age.Minutes()))
	case age < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(age.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(age.Hours()/24))
	}
}
