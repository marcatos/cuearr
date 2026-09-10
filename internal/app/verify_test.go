package app_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/app"
	"github.com/marcatos/cuearr/internal/ports"
)

type fakeFLACInspector struct {
	info  map[string]ports.FLACInfo
	err   map[string]error
	calls []string
}

func (f *fakeFLACInspector) Inspect(_ context.Context, path string) (ports.FLACInfo, error) {
	f.calls = append(f.calls, path)
	if err := f.err[path]; err != nil {
		return ports.FLACInfo{}, err
	}
	info, ok := f.info[path]
	if !ok {
		return ports.FLACInfo{}, fmt.Errorf("unexpected inspect %q", path)
	}
	return info, nil
}

type appliedTags struct {
	path string
	tags ports.TrackTags
}

type fakeFLACTagger struct {
	err      map[string]error
	calls    []appliedTags
	calledAt time.Time
}

func (f *fakeFLACTagger) ApplyTags(_ context.Context, path string, tags ports.TrackTags) error {
	f.calls = append(f.calls, appliedTags{path: path, tags: tags})
	f.calledAt = time.Now().UTC()
	return f.err[path]
}

func TestVerifySplit_AcceptsToleranceAndAppliesCueTags(t *testing.T) {
	cuePath := writeCue(t, `PERFORMER "Album Artist"
TITLE "Album"
FILE "image.flac" WAVE
  TRACK 01 AUDIO
    TITLE "One"
    INDEX 01 00:00:00
  TRACK 02 AUDIO
    TITLE "Two"
    PERFORMER "Guest"
    INDEX 01 00:03:00
`)
	files := []string{"/out/01.flac", "/out/02.flac"}
	inspector := &fakeFLACInspector{info: map[string]ports.FLACInfo{
		files[0]: {Duration: 3100 * time.Millisecond},
		files[1]: {Duration: 2900 * time.Millisecond},
	}}
	tagger := &fakeFLACTagger{}
	runner := app.JobRunner{
		Inspector: inspector,
		Tagger:    tagger,
		ReadFile:  os.ReadFile,
	}

	if err := runner.VerifySplit(context.Background(), cuePath, files, 6*time.Second); err != nil {
		t.Fatal(err)
	}
	if len(inspector.calls) != len(files) {
		t.Fatalf("inspect calls=%d, want %d", len(inspector.calls), len(files))
	}
	for i, path := range files {
		if inspector.calls[i] != path {
			t.Errorf("inspect call %d=%q, want %q", i, inspector.calls[i], path)
		}
	}

	want := []appliedTags{
		{path: files[0], tags: ports.TrackTags{Title: "One", Artist: "Album Artist", Album: "Album", TrackNumber: "1"}},
		{path: files[1], tags: ports.TrackTags{Title: "Two", Artist: "Guest", Album: "Album", TrackNumber: "2"}},
	}
	if len(tagger.calls) != len(want) {
		t.Fatalf("tag calls=%d, want %d", len(tagger.calls), len(want))
	}
	for i := range want {
		if tagger.calls[i] != want[i] {
			t.Errorf("tag call %d=%+v, want %+v", i, tagger.calls[i], want[i])
		}
	}
}

func TestVerifySplit_RejectsTrackCountMismatch(t *testing.T) {
	cuePath := writeCue(t, twoTrackCue)
	inspector := &fakeFLACInspector{}
	runner := app.JobRunner{
		Inspector: inspector,
		Tagger:    &fakeFLACTagger{},
		ReadFile:  os.ReadFile,
	}

	err := runner.VerifySplit(context.Background(), cuePath, []string{"/out/01.flac"}, 6*time.Second)
	if err == nil || !errors.Is(err, app.ErrOutputCountMismatch) {
		t.Fatalf("error=%v, want output count mismatch", err)
	}
	if len(inspector.calls) != 0 {
		t.Fatalf("inspect calls=%d, want 0", len(inspector.calls))
	}
}

func TestVerifySplit_RejectsDurationOutsideTolerance(t *testing.T) {
	cuePath := writeCue(t, twoTrackCue)
	files := []string{"/out/01.flac", "/out/02.flac"}
	inspector := &fakeFLACInspector{info: map[string]ports.FLACInfo{
		files[0]: {Duration: 3100*time.Millisecond + time.Nanosecond},
		files[1]: {Duration: 3 * time.Second},
	}}
	tagger := &fakeFLACTagger{}
	runner := app.JobRunner{Inspector: inspector, Tagger: tagger, ReadFile: os.ReadFile}

	err := runner.VerifySplit(context.Background(), cuePath, files, 6*time.Second)
	if err == nil || !errors.Is(err, app.ErrDurationMismatch) {
		t.Fatalf("error=%v, want duration mismatch", err)
	}
	if len(tagger.calls) != 0 {
		t.Fatalf("tag calls=%d, want 0", len(tagger.calls))
	}
}

func TestVerifySplit_PropagatesTagError(t *testing.T) {
	cuePath := writeCue(t, twoTrackCue)
	files := []string{"/out/01.flac", "/out/02.flac"}
	tagErr := errors.New("tag failed")
	runner := app.JobRunner{
		Inspector: &fakeFLACInspector{info: map[string]ports.FLACInfo{
			files[0]: {Duration: 3 * time.Second},
			files[1]: {Duration: 3 * time.Second},
		}},
		Tagger:   &fakeFLACTagger{err: map[string]error{files[1]: tagErr}},
		ReadFile: os.ReadFile,
	}

	err := runner.VerifySplit(context.Background(), cuePath, files, 6*time.Second)
	if !errors.Is(err, tagErr) {
		t.Fatalf("error=%v, want wrapped tag error", err)
	}
}

const twoTrackCue = `PERFORMER "Artist"
TITLE "Album"
FILE "image.flac" WAVE
  TRACK 01 AUDIO
    TITLE "One"
    INDEX 01 00:00:00
  TRACK 02 AUDIO
    TITLE "Two"
    INDEX 01 00:03:00
`

func writeCue(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "album.cue")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
