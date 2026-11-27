package components

import "fmt"

// FormatBytes formats a size compactly in binary units, e.g. "512B",
// "3.4K", "12M", "1.2G" (at most 5 characters below 1000T).
func FormatBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%dB", n)
	}
	v := float64(n)
	for _, unit := range []string{"K", "M", "G", "T"} {
		v /= 1024
		if v < 10 {
			return fmt.Sprintf("%.1f%s", v, unit)
		}
		if v < 1000 || unit == "T" {
			return fmt.Sprintf("%.0f%s", v, unit)
		}
	}
	return fmt.Sprintf("%.0fT", v)
}
