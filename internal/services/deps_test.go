package services

import (
	"reflect"
	"testing"

	"bbrew/internal/models"
)

func formula(name string, installed bool, deps ...string) models.Package {
	return models.Package{Name: name, LocallyInstalled: installed, Type: models.PackageTypeFormula,
		Formula: &models.Formula{Name: name, Dependencies: deps}}
}

func TestDepIndex(t *testing.T) {
	ix := newDepIndex([]models.Package{
		formula("wget", true, "openssl@3", "libidn2"),
		formula("curl", false, "openssl@3"),
		formula("openssl@3", true, "ca-certificates"),
		formula("libidn2", true, "libunistring"),
		formula("ca-certificates", true),
		formula("libunistring", true),
	})

	if got, want := ix.dependencies("wget"), map[string]bool{
		"openssl@3": true, "libidn2": true, "ca-certificates": true, "libunistring": true,
	}; !reflect.DeepEqual(got, want) {
		t.Errorf("dependencies(wget) = %v, want %v", got, want)
	}
	if got, want := ix.dependents("openssl@3"), map[string]bool{"wget": true, "curl": true}; !reflect.DeepEqual(got, want) {
		t.Errorf("dependents(openssl@3) = %v, want %v", got, want)
	}
	if got, want := ix.installedDependents("openssl@3"), []string{"wget"}; !reflect.DeepEqual(got, want) {
		t.Errorf("installedDependents(openssl@3) = %v, want %v", got, want)
	}
	if got := ix.dependencies("ca-certificates"); len(got) != 0 {
		t.Errorf("dependencies(ca-certificates) = %v, want none", got)
	}
}
