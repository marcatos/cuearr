package domain

import (
	"testing"
	"time"
)

func TestParseCueIndex(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
	}{
		{"00:00:00", 0},
		{"00:00:03", 3 * time.Second},
		{"01:00:02", (60*75 + 2) * time.Second / 75},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseCueIndex(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("ParseCueIndex(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestExpectedTrackDurations_twoTracksSixSeconds(t *testing.T) {
	sheet := CueSheet{
		Tracks: []CueTrack{
			{Number: 1, Index01: "00:00:00"},
			{Number: 2, Index01: "00:00:03"},
		},
	}
	total := 6 * time.Second
	durs, err := ExpectedTrackDurations(sheet, total)
	if err != nil {
		t.Fatal(err)
	}
	want := []time.Duration{3 * time.Second, 3 * time.Second}
	if len(durs) != len(want) {
		t.Fatalf("len=%d want %d", len(durs), len(want))
	}
	for i := range want {
		if durs[i] != want[i] {
			t.Fatalf("track %d: got %v want %v", i+1, durs[i], want[i])
		}
	}
}
