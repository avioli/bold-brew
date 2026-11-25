package services

import "bbrew/internal/models"

// StatusFilter narrows packages by their installation status.
type StatusFilter int

const (
	StatusAll StatusFilter = iota
	StatusInstalled
	StatusOutdated
	StatusLeaves
)

// KindFilter narrows packages by their kind.
type KindFilter int

const (
	KindAll KindFilter = iota
	KindFormulae
	KindCasks
)

// PackageFilter combines a status and a kind filter, e.g. Installed+Casks.
type PackageFilter struct {
	Status StatusFilter
	Kind   KindFilter
}

// IsNone reports whether no filter is active.
func (f PackageFilter) IsNone() bool {
	return f.Status == StatusAll && f.Kind == KindAll
}

// Matches reports whether a package passes the filter.
func (f PackageFilter) Matches(pkg models.Package) bool {
	switch f.Status {
	case StatusInstalled:
		if !pkg.LocallyInstalled {
			return false
		}
	case StatusOutdated:
		if !pkg.LocallyInstalled || !pkg.Outdated {
			return false
		}
	case StatusLeaves:
		if !pkg.LocallyInstalled || !pkg.InstalledOnRequest {
			return false
		}
	}

	switch f.Kind {
	case KindFormulae:
		return pkg.Type == models.PackageTypeFormula
	case KindCasks:
		return pkg.Type == models.PackageTypeCask
	}
	return true
}

// Label describes the filter, e.g. "Installed+Casks", or "" when none.
func (f PackageFilter) Label() string {
	status := map[StatusFilter]string{
		StatusInstalled: "Installed", StatusOutdated: "Outdated", StatusLeaves: "Leaves",
	}[f.Status]
	kind := map[KindFilter]string{KindFormulae: "Formulae", KindCasks: "Casks"}[f.Kind]

	switch {
	case status != "" && kind != "":
		return status + "+" + kind
	case status != "":
		return status
	default:
		return kind
	}
}

// ToggleStatus sets the status filter, or clears it if it is already set.
func (f *PackageFilter) ToggleStatus(status StatusFilter) {
	if f.Status == status {
		f.Status = StatusAll
	} else {
		f.Status = status
	}
}

// ToggleKind sets the kind filter, or clears it if it is already set.
func (f *PackageFilter) ToggleKind(kind KindFilter) {
	if f.Kind == kind {
		f.Kind = KindAll
	} else {
		f.Kind = kind
	}
}
