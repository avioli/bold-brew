package services

import "testing"

func TestParseVulnsJSON(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		want   int
		wantOK bool
	}{
		{"current none", `{"findings": [], "checked": 1, "complete": true}`, 0, true},
		{"current some", `{"findings": [{"formula": "x", "vulnerabilities": [{"id": "CVE-1", "severity": "HIGH"}, {"id": "CVE-2"}], "patched": [{"id": "CVE-3"}]}]}`, 2, true},
		{"legacy", `[{"name": "x", "vulnerabilities": [{"id": "CVE-1"}]}]`, 1, true},
		{"empty", ``, 0, false},
		{"garbage", `Error: nope`, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := parseVulnsJSON([]byte(c.in))
			if ok != c.wantOK || len(got) != c.want {
				t.Fatalf("got %d, %v; want %d, %v", len(got), ok, c.want, c.wantOK)
			}
			if got == nil {
				t.Fatal("nil slice: the details panel would hide the section")
			}
		})
	}
}
