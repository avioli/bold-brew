package components

import (
	"reflect"
	"testing"
)

// Typical content: long names (capped), 15+ char versions, 7 digit downloads.
// Minimums are Type 1, Name 20, Version 10, Description 11, Downloads 7, so
// Type+Name take 23 columns including separators.
var typicalInput = fitInput{name: 40, version: 18, downloads: 7, sortColumn: -1}

func TestFitColumnsWindows(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		offset    int
		want      []int
		maxOffset int
	}{
		{"comfortable (147 screen with details, 106 without): all columns", downloadsComfortWidth, 0,
			[]int{ColType, ColName, ColVersion, ColDescription, ColDownloads}, 0},
		{"wide: scrolling right is a no-op", 120, 1,
			[]int{ColType, ColName, ColVersion, ColDescription, ColDownloads}, 0},
		{"just below comfortable: Downloads hidden at first", downloadsComfortWidth - 1, 0,
			[]int{ColType, ColName, ColVersion, ColDescription}, 1},
		{"just below comfortable: scrolled right shows Downloads", downloadsComfortWidth - 1, 1,
			[]int{ColType, ColName, ColDescription, ColDownloads}, 1},
		{"medium: offset clamps at the rightmost position", 90, 5,
			[]int{ColType, ColName, ColDescription, ColDownloads}, 1},
		{"small: Description too narrow to share with Downloads", 50, 0,
			[]int{ColType, ColName, ColVersion, ColDescription}, 2},
		{"small: description alone", 50, 1, []int{ColType, ColName, ColDescription}, 2},
		{"small: downloads alone", 50, 2, []int{ColType, ColName, ColDownloads}, 2},
		{"narrow: version only", 36, 0, []int{ColType, ColName, ColVersion}, 2},
		{"narrow: description only", 36, 1, []int{ColType, ColName, ColDescription}, 2},
		{"narrow: downloads only", 36, 2, []int{ColType, ColName, ColDownloads}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := fitColumns(tt.width, typicalInput, tt.offset)
			if !reflect.DeepEqual(r.columns, tt.want) {
				t.Errorf("columns = %v, want %v", r.columns, tt.want)
			}
			if r.maxOffset != tt.maxOffset {
				t.Errorf("maxOffset = %d, want %d", r.maxOffset, tt.maxOffset)
			}
		})
	}
}

func TestFitColumnsNeverOverflows(t *testing.T) {
	for width := 23; width <= 200; width++ {
		for offset := 0; offset < 3; offset++ {
			r := fitColumns(width, typicalInput, offset)
			used := 0
			for _, col := range r.columns {
				used += r.widths[col] + 1
			}
			if used > width {
				t.Fatalf("width %d offset %d: columns use %d", width, offset, used)
			}
			if r.columns[0] != ColType || r.columns[1] != ColName {
				t.Fatalf("width %d offset %d: Type and Name must be frozen, got %v", width, offset, r.columns)
			}
			shows := map[int]bool{}
			for _, col := range r.columns {
				shows[col] = true
			}
			if shows[ColDownloads] && shows[ColDescription] && r.widths[ColDescription] < readDescWidth {
				t.Fatalf("width %d offset %d: Description %d next to Downloads", width, offset, r.widths[ColDescription])
			}
			if r.widths[ColName] < clipNameWidth {
				t.Fatalf("width %d: Name clipped below %d", width, clipNameWidth)
			}
		}
	}
}

func TestFitColumnsCompactHeaders(t *testing.T) {
	wide := fitColumns(120, typicalInput, 0)
	if wide.compact || wide.headers[ColType] != "Type" || wide.headers[ColDownloads] != "Downloads" {
		t.Errorf("wide: compact=%v headers=%q/%q", wide.compact, wide.headers[ColType], wide.headers[ColDownloads])
	}
	if wide.widths[ColVersion] != maxVersionWidth {
		t.Errorf("wide: Version width = %d, want %d", wide.widths[ColVersion], maxVersionWidth)
	}

	// Scrolled right, with just enough for Description (20) and Downloads (7).
	tight := fitColumns(23+21+8, typicalInput, 1)
	if !tight.compact || tight.headers[ColType] != "T" || tight.headers[ColDownloads] != "Dl" {
		t.Errorf("tight: compact=%v headers=%q/%q", tight.compact, tight.headers[ColType], tight.headers[ColDownloads])
	}
}
