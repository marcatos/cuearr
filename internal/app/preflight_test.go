package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/app"
	"github.com/marcatos/cuearr/internal/ports"
)

type fakeSafetyProbe struct {
	sizes       []int64
	statCalls   int
	free        uint64
	writableErr error
	freeErr     error
}

func (f *fakeSafetyProbe) StatSize(string) (int64, error) {
	size := f.sizes[f.statCalls]
	f.statCalls++
	return size, nil
}

func (f *fakeSafetyProbe) EnsureWritable(string) error {
	return f.writableErr
}

func (f *fakeSafetyProbe) FreeSpace(string) (uint64, error) {
	return f.free, f.freeErr
}

func TestPreflight_RejectsImageWhoseSizeChanges(t *testing.T) {
	probe := &fakeSafetyProbe{sizes: []int64{100, 101}, free: 1 << 30}
	var waited time.Duration
	checker := app.Preflight{
		Probe:        probe,
		StabilityGap: 500 * time.Millisecond,
		Wait: func(_ context.Context, duration time.Duration) error {
			waited = duration
			return nil
		},
	}

	err := checker.Check(context.Background(), "/music/album.flac", "/output")
	if !errors.Is(err, app.ErrImageUnstable) {
		t.Fatalf("got %v, want ErrImageUnstable", err)
	}
	if waited < 500*time.Millisecond {
		t.Fatalf("waited %v, want at least 500ms", waited)
	}
}

func TestPreflight_RejectsInsufficientFreeSpace(t *testing.T) {
	const imageSize = int64(1024)
	required := uint64(imageSize) + 64*1024*1024
	probe := &fakeSafetyProbe{sizes: []int64{imageSize, imageSize}, free: required - 1}
	checker := instantPreflight(probe)

	err := checker.Check(context.Background(), "/music/album.flac", "/output")
	if !errors.Is(err, app.ErrInsufficientSpace) {
		t.Fatalf("got %v, want ErrInsufficientSpace", err)
	}
}

func TestPreflight_RequiresWritableOutput(t *testing.T) {
	probe := &fakeSafetyProbe{
		sizes:       []int64{1024},
		free:        1 << 30,
		writableErr: errors.New("permission denied"),
	}
	checker := instantPreflight(probe)

	err := checker.Check(context.Background(), "/music/album.flac", "/read-only")
	if !errors.Is(err, app.ErrOutputNotWritable) {
		t.Fatalf("got %v, want ErrOutputNotWritable", err)
	}
}

func TestPreflight_SucceedsWhenStableWritableAndLargeEnough(t *testing.T) {
	const imageSize = int64(128 * 1024 * 1024)
	probe := &fakeSafetyProbe{
		sizes: []int64{imageSize, imageSize},
		free:  uint64(imageSize * 2),
	}
	checker := instantPreflight(probe)

	if err := checker.Check(context.Background(), "/music/album.flac", "/output"); err != nil {
		t.Fatal(err)
	}
	if probe.statCalls != 2 {
		t.Fatalf("stat calls=%d, want 2", probe.statCalls)
	}
}

func instantPreflight(probe *fakeSafetyProbe) app.Preflight {
	return app.Preflight{
		Probe:        probe,
		StabilityGap: 500 * time.Millisecond,
		Wait:         func(context.Context, time.Duration) error { return nil },
	}
}

var _ ports.FileSafetyProbe = (*fakeSafetyProbe)(nil)
