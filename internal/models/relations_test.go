package models

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestCaskRelationsLenientDecoding(t *testing.T) {
	tests := []struct {
		json string
		want CaskRelations
	}{
		{`{"depends_on":{"macos":{">=":["12"]},"formula":["ffmpeg"],"cask":["xquartz"]}}`,
			CaskRelations{Formula: StringList{"ffmpeg"}, Cask: StringList{"xquartz"}}},
		{`{"depends_on":{"formula":"ffmpeg"}}`, CaskRelations{Formula: StringList{"ffmpeg"}}},
		{`{"depends_on":{}}`, CaskRelations{}},
		{`{"depends_on":null}`, CaskRelations{}},
		{`{"depends_on":["unexpected"]}`, CaskRelations{}},
		{`{"depends_on":{"formula":42}}`, CaskRelations{}},
	}
	for _, tt := range tests {
		var c Cask
		if err := json.Unmarshal([]byte(tt.json), &c); err != nil {
			t.Fatalf("%s: %v", tt.json, err)
		}
		if !reflect.DeepEqual(c.DependsOn, tt.want) {
			t.Errorf("%s: got %+v, want %+v", tt.json, c.DependsOn, tt.want)
		}
	}
}

func TestPackageDependencies(t *testing.T) {
	f := Package{Formula: &Formula{Dependencies: []string{"openssl@3", "user/tap/thing"}}}
	if got := f.Dependencies(); !reflect.DeepEqual(got, []string{"openssl@3", "thing"}) {
		t.Errorf("formula deps = %v", got)
	}
	c := Package{Cask: &Cask{DependsOn: CaskRelations{Formula: StringList{"ffmpeg"}, Cask: StringList{"xquartz"}}}}
	if got := c.Dependencies(); !reflect.DeepEqual(got, []string{"ffmpeg", "xquartz"}) {
		t.Errorf("cask deps = %v", got)
	}
}
