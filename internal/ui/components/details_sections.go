package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/rivo/tview"

	"bbrew/internal/models"
)

const detailsSeparator = "[dim]────────────────────────[-:-:-]"

// SetRequiredByFunc sets how to find the installed packages that depend on a
// package, for the "Required By" section.
func (d *Details) SetRequiredByFunc(f func(name string) []string) {
	d.requiredBy = f
}

// section formats a titled section, or "" when body is empty.
func section(title, body string) string {
	if strings.TrimSpace(body) == "" {
		return ""
	}
	return fmt.Sprintf("[yellow::b]%s[-:-:-]\n%s\n%s", title, detailsSeparator, body)
}

// bullet formats a "• Label: value" line, or "" when value is empty.
func bullet(label, value string) string {
	if value == "" {
		return ""
	}
	return fmt.Sprintf("[blue]• %s:[-:-:-] %s\n", label, value)
}

// strings converts a JSON list of strings (decoded as []interface{}).
func toStrings(list []interface{}) []string {
	out := make([]string, 0, len(list))
	for _, v := range list {
		if s := interfaceToString(v); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// joinNames joins names with commas, three per line, escaped for tview.
func joinNames(names []string) string {
	var sb strings.Builder
	for i, n := range names {
		sb.WriteString(tview.Escape(n))
		if i < len(names)-1 {
			if (i+1)%3 == 0 {
				sb.WriteString("\n")
			} else {
				sb.WriteString(", ")
			}
		}
	}
	return sb.String()
}

func formatDate(unix int64) string {
	if unix <= 0 {
		return ""
	}
	return time.Unix(unix, 0).Format("2006-01-02")
}

// kegOnlyReason extracts the explanation from keg_only_reason, which is an
// object like {"reason": ":provided_by_macos", "explanation": "..."}.
func kegOnlyReason(v interface{}) string {
	m, ok := v.(map[string]interface{})
	if !ok {
		return interfaceToString(v)
	}
	if e := interfaceToString(m["explanation"]); e != "" {
		return e
	}
	return strings.TrimPrefix(interfaceToString(m["reason"]), ":")
}

// caveatsSection shows post-install notes, often the most useful part of
// brew info.
func (d *Details) caveatsSection(pkg *models.Package) string {
	var caveats string
	switch {
	case pkg.Formula != nil:
		caveats = interfaceToString(pkg.Formula.Caveats)
	case pkg.Cask != nil:
		caveats = interfaceToString(pkg.Cask.Caveats)
	}
	return section("Caveats", tview.Escape(strings.TrimSpace(caveats)))
}

// dependenciesSection lists runtime, recommended, optional and build
// dependencies (formulae), or the formulae and casks a cask depends on.
func (d *Details) dependenciesSection(pkg *models.Package) string {
	var sb strings.Builder
	add := func(label string, names []string) {
		if len(names) > 0 {
			fmt.Fprintf(&sb, "[blue]• %s:[-:-:-]\n%s\n", label, joinNames(names))
		}
	}

	switch {
	case pkg.Formula != nil:
		f := pkg.Formula
		add("Runtime", f.Dependencies)
		add("Recommended", toStrings(f.RecommendedDependencies))
		add("Optional", toStrings(f.OptionalDependencies))
		add("Build", f.BuildDependencies)
	case pkg.Cask != nil:
		add("Formulae", pkg.Cask.DependsOn.Formula)
		add("Casks", pkg.Cask.DependsOn.Cask)
	default:
		return ""
	}

	if sb.Len() == 0 {
		return section("Dependencies", "No dependencies")
	}
	return section("Dependencies", strings.TrimSuffix(sb.String(), "\n"))
}

// requiredBySection lists installed packages that depend on this one.
func (d *Details) requiredBySection(pkg *models.Package) string {
	if d.requiredBy == nil {
		return ""
	}
	return section("Required By (installed)", joinNames(d.requiredBy(pkg.Name)))
}

// conflictsSection lists packages this one conflicts with.
func (d *Details) conflictsSection(pkg *models.Package) string {
	var names []string
	switch {
	case pkg.Formula != nil:
		names = toStrings(pkg.Formula.ConflictsWith)
		reasons := toStrings(pkg.Formula.ConflictsWithReasons)
		for i := range names {
			if i < len(reasons) && reasons[i] != "" {
				names[i] += " (" + reasons[i] + ")"
			}
		}
	case pkg.Cask != nil:
		names = append(append(names, pkg.Cask.ConflictsWith.Formula...), pkg.Cask.ConflictsWith.Cask...)
	}
	if len(names) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, n := range names {
		fmt.Fprintf(&sb, "%s\n", tview.Escape(n))
	}
	return section("Conflicts With", strings.TrimSuffix(sb.String(), "\n"))
}

// aliasesSection lists other names the package is known by.
func (d *Details) aliasesSection(pkg *models.Package) string {
	var aliases, oldNames []string
	switch {
	case pkg.Formula != nil:
		aliases, oldNames = pkg.Formula.Aliases, pkg.Formula.OldNames
	case pkg.Cask != nil:
		oldNames = pkg.Cask.OldTokens
	}
	body := bullet("Aliases", joinNames(aliases)) + bullet("Old names", joinNames(oldNames))
	return section("Also Known As", strings.TrimSuffix(body, "\n"))
}
