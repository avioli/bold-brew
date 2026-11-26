package models

import (
	"encoding/json"
	"path"
)

// StringList is a list of strings that also accepts a single string in JSON.
// Unexpected shapes decode to an empty list rather than failing, so a change
// in Homebrew's API cannot break loading packages.
type StringList []string

func (l *StringList) UnmarshalJSON(b []byte) error {
	var many []string
	if json.Unmarshal(b, &many) == nil {
		*l = many
		return nil
	}
	var one string
	if json.Unmarshal(b, &one) == nil && one != "" {
		*l = StringList{one}
	}
	return nil
}

// CaskRelations is the formula and cask part of a cask's depends_on or
// conflicts_with. Other keys (macos, arch, ...) are ignored.
type CaskRelations struct {
	Formula StringList `json:"formula,omitempty"`
	Cask    StringList `json:"cask,omitempty"`
}

func (r *CaskRelations) UnmarshalJSON(b []byte) error {
	var m map[string]json.RawMessage
	if json.Unmarshal(b, &m) != nil {
		return nil
	}
	if raw, ok := m["formula"]; ok {
		_ = r.Formula.UnmarshalJSON(raw)
	}
	if raw, ok := m["cask"]; ok {
		_ = r.Cask.UnmarshalJSON(raw)
	}
	return nil
}

// Dependencies returns the names of the package's direct runtime
// dependencies, without tap prefixes.
func (p *Package) Dependencies() []string {
	var deps []string
	switch {
	case p.Formula != nil:
		deps = p.Formula.Dependencies
	case p.Cask != nil:
		deps = append(append(deps, p.Cask.DependsOn.Formula...), p.Cask.DependsOn.Cask...)
	}
	names := make([]string, 0, len(deps))
	for _, d := range deps {
		names = append(names, path.Base(d)) // "user/tap/name" -> "name"
	}
	return names
}
