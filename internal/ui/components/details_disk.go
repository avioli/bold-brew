package components

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rivo/tview"

	"bbrew/internal/models"
)

// SetDiskUsageFunc sets how to look up a package's measured disk usage, for
// the "Disk Usage" section. scanned is false until the first scan finished.
func (d *Details) SetDiskUsageFunc(f func(pkg *models.Package) (*models.DiskUsage, bool)) {
	d.diskUsage = f
}

// newStatus marks packages that are new since the last brew update.
func newStatus(pkg *models.Package) string {
	if pkg.New {
		return ", [green]new[-:-:-]"
	}
	return ""
}

func (d *Details) diskUsageSection(pkg *models.Package) string {
	if d.diskUsage == nil || (pkg.Type != models.PackageTypeFormula && pkg.Type != models.PackageTypeCask) {
		return ""
	}
	u, scanned := d.diskUsage(pkg)
	if u == nil {
		if !scanned && pkg.LocallyInstalled {
			return section("Disk Usage", "[dim]Measuring...[-:-:-]")
		}
		return ""
	}

	var sb strings.Builder
	if pkg.LocallyInstalled {
		installed := FormatBytes(u.Installed)
		if u.Old > 0 {
			installed += fmt.Sprintf(" (%s in old versions)", FormatBytes(u.Old))
		}
		sb.WriteString(bullet("Installed", installed))
		sb.WriteString(diskPaths(u.InstalledPaths))
	}
	if u.Cache > 0 {
		sb.WriteString(bullet("Download cache", FormatBytes(u.Cache)))
		sb.WriteString(diskPaths(u.CachePaths))
	}
	if u.AppData > 0 {
		sb.WriteString(bullet("App data (zap)", FormatBytes(u.AppData)))
		sb.WriteString(diskPaths(u.AppDataPaths))
	}

	var notes []string
	if u.PkgFiles {
		notes = append(notes, "Files installed by the .pkg installer are not counted.")
	}
	if u.Unreadable > 0 {
		notes = append(notes, fmt.Sprintf("Partial: %d item%s unreadable without more permissions.", u.Unreadable, plural(u.Unreadable)))
	}
	if n := len(u.Skipped); n > 0 {
		sb.WriteString(bullet("Not checked", fmt.Sprintf("%d protected location%s", n, plural(n))))
		sb.WriteString("[dim]" + tview.Escape("Reading these would make macOS ask for privacy access:") + "[-:-:-]\n")
		for _, p := range u.Skipped {
			sb.WriteString(tview.Escape("  "+tildePath(p)) + "\n")
		}
	}
	for _, n := range notes {
		sb.WriteString("[dim]" + tview.Escape(n) + "[-:-:-]\n")
	}
	return section("Disk Usage", strings.TrimSuffix(sb.String(), "\n"))
}

// diskPaths lists the paths, largest first, indented.
func diskPaths(paths []models.SizedPath) string {
	var sb strings.Builder
	for _, p := range paths {
		line := fmt.Sprintf("  %6s %s", FormatBytes(p.Bytes), tildePath(p.Path))
		if p.Label != "" {
			line += " (" + p.Label + ")"
		}
		sb.WriteString(tview.Escape(line) + "\n")
	}
	return sb.String()
}

// tildePath shows the home directory as ~.
func tildePath(path string) string {
	home, _ := os.UserHomeDir()
	if home != "" && strings.HasPrefix(path, home+string(filepath.Separator)) {
		return "~" + path[len(home):]
	}
	return path
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
