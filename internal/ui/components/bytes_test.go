package components

import "testing"

func TestFormatBytes(t *testing.T) {
	for n, want := range map[int64]string{
		0: "0B", 1023: "1023B", 1024: "1.0K", 10 * 1024: "10K", 999 * 1024: "999K",
		1000 * 1024: "1.0M", 5 << 30: "5.0G", 123 << 30: "123G", 2 << 40: "2.0T",
	} {
		if got := FormatBytes(n); got != want {
			t.Errorf("FormatBytes(%d) = %q, want %q", n, got, want)
		}
	}
}
