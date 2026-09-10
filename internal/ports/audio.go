package ports

import (
	"context"
	"time"
)

type FLACInfo struct {
	SampleRate   int
	TotalSamples int64
	Duration     time.Duration // TotalSamples / SampleRate
}

type FLACInspector interface {
	Inspect(ctx context.Context, path string) (FLACInfo, error)
}

type TrackTags struct {
	Title, Artist, Album, TrackNumber string
}

type FLACTagger interface {
	ApplyTags(ctx context.Context, path string, tags TrackTags) error
}
