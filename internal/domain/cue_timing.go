package domain

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const cueFramesPerSecond = 75

// ParseCueIndex converts a CUE INDEX timestamp (MM:SS:FF) to a duration from the
// start of the image. FF is Red Book CD frames at 75/s; SS must be 0–59, FF 0–74.
func ParseCueIndex(mmssff string) (time.Duration, error) {
	mmssff = strings.TrimSpace(mmssff)
	parts := strings.Split(mmssff, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid cue index %q: want MM:SS:FF", mmssff)
	}
	minutes, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid cue index minutes in %q: %w", mmssff, err)
	}
	seconds, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid cue index seconds in %q: %w", mmssff, err)
	}
	frames, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, fmt.Errorf("invalid cue index frames in %q: %w", mmssff, err)
	}
	if minutes < 0 || seconds < 0 || seconds >= 60 {
		return 0, fmt.Errorf("invalid cue index %q", mmssff)
	}
	if frames < 0 || frames >= cueFramesPerSecond {
		return 0, fmt.Errorf("invalid cue index frames in %q", mmssff)
	}
	totalFrames := (minutes*60+seconds)*cueFramesPerSecond + frames
	return time.Duration(totalFrames) * time.Second / cueFramesPerSecond, nil
}

// ExpectedTrackDurations returns per-track lengths from successive INDEX 01 values.
// The last track spans from its INDEX 01 to totalAudio.
func ExpectedTrackDurations(sheet CueSheet, totalAudio time.Duration) ([]time.Duration, error) {
	if len(sheet.Tracks) == 0 {
		return nil, fmt.Errorf("cue sheet has no tracks")
	}
	if totalAudio < 0 {
		return nil, fmt.Errorf("total audio duration must be non-negative")
	}

	starts := make([]time.Duration, len(sheet.Tracks))
	for i, tr := range sheet.Tracks {
		if tr.Index01 == "" {
			return nil, fmt.Errorf("track %d missing INDEX 01", tr.Number)
		}
		start, err := ParseCueIndex(tr.Index01)
		if err != nil {
			return nil, fmt.Errorf("track %d INDEX 01: %w", tr.Number, err)
		}
		starts[i] = start
	}

	for i := 1; i < len(starts); i++ {
		if starts[i] < starts[i-1] {
			return nil, fmt.Errorf("track INDEX 01 times must be non-decreasing")
		}
	}
	last := starts[len(starts)-1]
	if last > totalAudio {
		return nil, fmt.Errorf("last INDEX 01 (%v) after total audio (%v)", last, totalAudio)
	}

	out := make([]time.Duration, len(starts))
	for i := 0; i < len(starts)-1; i++ {
		out[i] = starts[i+1] - starts[i]
	}
	out[len(starts)-1] = totalAudio - last
	return out, nil
}
