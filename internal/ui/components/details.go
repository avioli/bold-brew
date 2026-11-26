package components

import (
	"bbrew/internal/models"
	"bbrew/internal/ui/theme"
	"fmt"
	"regexp"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// noColorLabel matches a "[blue]• Label:[-:-:-]" bullet label.
var noColorLabel = regexp.MustCompile(`\[blue\](• [^\[]*?:)\[-:-:-\]`)

// DetailsPadding is the horizontal padding on each side of the details text.
const DetailsPadding = 1

// DetailsMinWidth is the minimum total width of the details panel: 34 columns
// of content, padding on both sides and the left border.
const DetailsMinWidth = 34 + 2*DetailsPadding + 1

type Details struct {
	view        *tview.TextView
	theme       *theme.Theme
	inlineTitle bool
	requiredBy  func(name string) []string
}

func NewDetails(theme *theme.Theme) *Details {
	details := &Details{
		view:        tview.NewTextView(),
		theme:       theme,
		inlineTitle: true,
	}

	details.view.SetDynamicColors(true)
	details.view.SetTextAlign(tview.AlignLeft)
	details.view.SetBorder(false)
	details.view.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		borderStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite)
		titleStyle := tcell.StyleDefault.Foreground(theme.TitleColor)
		if details.view.HasFocus() {
			titleStyle = titleStyle.Reverse(true)
		}
		for row := y; row < y+height; row++ {
			screen.SetContent(x, row, tview.Borders.Vertical, nil, borderStyle)
		}
		// Left border, then one column of padding on each side.
		innerX, innerWidth := x+1+DetailsPadding, width-1-2*DetailsPadding
		if !details.inlineTitle {
			// The title is drawn on the outer border by the layout.
			return innerX, y, innerWidth, height
		}
		title := " Details "
		for i, ch := range title {
			screen.SetContent(x+1+i, y, ch, nil, titleStyle)
		}
		return innerX, y + 2, innerWidth, height - 2
	})
	return details
}

// SetInlineTitle draws the "Details" title inside the panel. Turn it off when
// the layout draws the title on the outer border instead.
func (d *Details) SetInlineTitle(inline bool) {
	d.inlineTitle = inline
}

func (d *Details) SetContent(pkg *models.Package, vulns []models.Vulnerability) {
	if pkg == nil {
		d.view.SetText("")
		return
	}

	// Installation status with colors
	installedStatus := "[red]Not installed[-:-:-]"
	if pkg.LocallyInstalled {
		installedStatus = "[green]Installed[-:-:-]"
		if pkg.Outdated {
			installedStatus = "[orange]Update available[-:-:-]"
		}
	}

	// Health warning inline (shown next to status)
	healthInline := ""
	if pkg.Disabled {
		healthInline = " [red]⚠ DISABLED[-:-:-]"
	} else if pkg.Deprecated {
		healthInline = " [yellow]⚠ DEPRECATED[-:-:-]"
	}

	// Type tag with escaped brackets
	var typeTag, typeLabel string
	switch pkg.Type {
	case models.PackageTypeCask:
		typeTag = tview.Escape("[C]")
		typeLabel = "Cask"
	case models.PackageTypeFlatpak:
		typeTag = tview.Escape("[P]")
		typeLabel = "Flatpak"
	case models.PackageTypeMas:
		typeTag = tview.Escape("[M]")
		typeLabel = "Mac App Store"
	default:
		typeTag = tview.Escape("[F]")
		typeLabel = "Formula"
	}

	// Section separator
	separator := "[dim]────────────────────────[-:-:-]"

	// Basic information with status
	nameLabel := "Name"
	if pkg.Type == models.PackageTypeMas {
		nameLabel = "App ID"
	}
	version := pkg.Version
	var license, tap, extraStatus, autoUpdates string
	switch {
	case pkg.Formula != nil:
		f := pkg.Formula
		if f.Versions.Head != "" {
			version += " (HEAD available)"
		}
		license, tap = f.License, f.Tap
		if f.Pinned {
			extraStatus += ", [blue]pinned[-:-:-]"
		}
		if f.KegOnly {
			extraStatus += ", keg-only"
		}
	case pkg.Cask != nil:
		tap = pkg.Cask.Tap
		if pkg.Cask.AutoUpdates {
			autoUpdates = "Yes (the app updates itself)"
		}
	}

	basicInfo := fmt.Sprintf(
		"[yellow::b]%s[-:-:-]\n%s\n"+
			"[blue]• Type:[-:-:-] %s %s\n"+
			"[blue]• %s:[-:-:-] %s\n"+
			"[blue]• Display Name:[-:-:-] %s\n"+
			"[blue]• Version:[-:-:-] %s\n"+
			"[blue]• Status:[-:-:-] %s%s%s\n"+
			"%s%s%s"+
			"[blue]• Homepage:[-:-:-] %s\n\n"+
			"[yellow::b]Description[-:-:-]\n%s\n%s",
		tview.Escape(pkg.Label()), separator,
		typeTag, typeLabel,
		nameLabel, tview.Escape(pkg.Name),
		tview.Escape(pkg.DisplayName),
		tview.Escape(version),
		installedStatus, extraStatus, healthInline,
		bullet("License", tview.Escape(license)),
		bullet("Tap", tview.Escape(tap)),
		bullet("Auto-updates", autoUpdates),
		pkg.Homepage,
		separator,
		tview.Escape(pkg.Description),
	)

	// Sections in order of usefulness; empty ones are skipped.
	parts := []string{
		basicInfo,
		d.getHealthInfo(pkg),
		d.caveatsSection(pkg),
		d.getVulnInfo(vulns),
		d.getPackageInstallationDetails(pkg),
		d.dependenciesSection(pkg),
		d.requiredBySection(pkg),
		d.conflictsSection(pkg),
		d.aliasesSection(pkg),
		d.getAnalyticsInfo(pkg),
	}
	nonEmpty := parts[:0]
	for _, p := range parts {
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	parts = nonEmpty

	text := strings.Join(parts, "\n\n")
	if theme.NoColor() {
		// Without colour, labels ("• Version:") are bold; values stay normal.
		text = noColorLabel.ReplaceAllString(text, "[::b]$1[::-]")
	}
	d.view.SetText(text)
}

func (d *Details) getHealthInfo(pkg *models.Package) string {
	if !pkg.Deprecated && !pkg.Disabled {
		return ""
	}

	separator := "[dim]────────────────────────[-:-:-]"

	var title, reason, date, replacement string

	if pkg.Disabled {
		title = "[red::b]⚠ Package Disabled[-:-:-]"
		if pkg.Formula != nil {
			reason = interfaceToString(pkg.Formula.DisableReason)
			date = interfaceToString(pkg.Formula.DisableDate)
			replacement = interfaceToString(pkg.Formula.DisableReplacement)
		} else if pkg.Cask != nil {
			reason = interfaceToString(pkg.Cask.DisableReason)
			date = interfaceToString(pkg.Cask.DisableDate)
		}
	} else {
		title = "[yellow::b]⚠ Package Deprecated[-:-:-]"
		if pkg.Formula != nil {
			reason = interfaceToString(pkg.Formula.DeprecationReason)
			date = interfaceToString(pkg.Formula.DeprecationDate)
			replacement = interfaceToString(pkg.Formula.DeprecationReplacement)
		} else if pkg.Cask != nil {
			reason = interfaceToString(pkg.Cask.DeprecationReason)
			date = interfaceToString(pkg.Cask.DeprecationDate)
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s\n%s\n", title, separator)

	if reason != "" {
		fmt.Fprintf(&sb, "[blue]• Reason:[-:-:-] %s\n", reason)
	}
	if date != "" {
		fmt.Fprintf(&sb, "[blue]• Since:[-:-:-] %s\n", date)
	}
	if replacement != "" {
		fmt.Fprintf(&sb, "[blue]• Replacement:[-:-:-] [green]%s[-:-:-]\n", replacement)
	}

	if pkg.Disabled {
		sb.WriteString("\n[dim]This package can no longer be installed.[-:-:-]")
	} else {
		sb.WriteString("\n[dim]Consider migrating to the replacement before this package is removed.[-:-:-]")
	}

	return sb.String()
}

func interfaceToString(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

func (d *Details) getPackageInstallationDetails(pkg *models.Package) string {
	separator := "[dim]────────────────────────[-:-:-]"

	if !pkg.LocallyInstalled {
		return fmt.Sprintf("[yellow::b]Installation[-:-:-]\n%s\nNot installed", separator)
	}

	// For formulae, show detailed installation info
	if pkg.Type == models.PackageTypeFormula && pkg.Formula != nil && len(pkg.Formula.Installed) > 0 {
		packagePrefix := pkg.Formula.LocalPath

		installedOnRequest := "No"
		if pkg.Formula.Installed[0].InstalledOnRequest {
			installedOnRequest = "Yes"
		}

		installedAsDependency := "No"
		if pkg.Formula.Installed[0].InstalledAsDependency {
			installedAsDependency = "Yes"
		}

		inst := pkg.Formula.Installed[0]
		source := "Bottle"
		if !inst.PouredFromBottle {
			source = "Built from source"
		}
		extra := bullet("Installed on", formatDate(inst.Time)) + bullet("Installed from", source)
		if pkg.Formula.KegOnly {
			extra += bullet("Keg-only", tview.Escape(kegOnlyReason(pkg.Formula.KegOnlyReason)))
		}

		return fmt.Sprintf(
			"[yellow::b]Installation Details[-:-:-]\n%s\n"+
				"[blue]• Path:[-:-:-] %s\n"+
				"[blue]• Installed on request:[-:-:-] %s\n"+
				"[blue]• Installed as dependency:[-:-:-] %s\n"+
				"[blue]• Installed version:[-:-:-] %s\n%s",
			separator,
			packagePrefix,
			installedOnRequest,
			installedAsDependency,
			inst.Version,
			strings.TrimSuffix(extra, "\n"),
		)
	}

	// For casks, show simpler installation info
	if pkg.Type == models.PackageTypeCask && pkg.Cask != nil {
		installedVersion := "Unknown"
		if pkg.Cask.Installed != nil {
			installedVersion = *pkg.Cask.Installed
		}

		installedOn := ""
		if pkg.Cask.InstalledTime != nil {
			installedOn = bullet("Installed on", formatDate(*pkg.Cask.InstalledTime))
		}

		return fmt.Sprintf(
			"[yellow::b]Installation Details[-:-:-]\n%s\n"+
				"[blue]• Type:[-:-:-] Desktop Application\n"+
				"[blue]• Installed version:[-:-:-] %s%s",
			separator,
			installedVersion,
			strings.TrimSuffix("\n"+installedOn, "\n"),
		)
	}

	return fmt.Sprintf("[yellow::b]Installation[-:-:-]\n%s\nInstalled", separator)
}

func (d *Details) getAnalyticsInfo(pkg *models.Package) string {
	separator := "[dim]────────────────────────[-:-:-]"
	p := message.NewPrinter(language.English)

	return fmt.Sprintf(
		"[yellow::b]Analytics[-:-:-]\n%s\n"+
			"[blue]• 90d Global Rank:[-:-:-] %s\n"+
			"[blue]• 90d Downloads:[-:-:-] %s",
		separator,
		p.Sprintf("%d", pkg.Analytics90dRank),
		p.Sprintf("%d", pkg.Analytics90dDownloads),
	)
}

func (d *Details) getVulnInfo(vulns []models.Vulnerability) string {
	if vulns == nil {
		return ""
	}

	separator := "[dim]────────────────────────[-:-:-]"

	if len(vulns) == 0 {
		return fmt.Sprintf("[yellow::b]Security[-:-:-]\n%s\n[green]✓ No known vulnerabilities[-:-:-]", separator)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "[red::b]⚠ Security (%d vulnerabilit", len(vulns))
	if len(vulns) == 1 {
		sb.WriteString("y")
	} else {
		sb.WriteString("ies")
	}
	fmt.Fprintf(&sb, ")[-:-:-]\n%s\n", separator)

	for _, v := range vulns {
		sevColor := vulnSeverityColor(v.Severity)
		fmt.Fprintf(&sb, "  [%s][%s][-:-:-] %s\n", sevColor, v.Severity, v.ID)
		if v.Summary != "" {
			fmt.Fprintf(&sb, "    [dim]%s[-:-:-]\n", v.Summary)
		}
	}

	return sb.String()
}

func vulnSeverityColor(severity string) string {
	switch strings.ToUpper(severity) {
	case "CRITICAL":
		return "red"
	case "HIGH":
		return "red"
	case "MEDIUM":
		return "orange"
	case "LOW":
		return "yellow"
	default:
		return "white"
	}
}

func (d *Details) View() *tview.TextView {
	return d.view
}

func (d *Details) Clear() {
	d.view.Clear()
}
