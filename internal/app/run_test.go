package app_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/app"
	"github.com/marcatos/cuearr/internal/domain"
	"github.com/marcatos/cuearr/internal/ports"
)

type fakeSplitter struct {
	result  ports.SplitResult
	err     error
	outDirs []string
}

func (f *fakeSplitter) Name() string { return "fake" }

func (f *fakeSplitter) Available(context.Context) error { return nil }

func (f *fakeSplitter) Split(_ context.Context, _ domain.SplitPlan, outDir string) (ports.SplitResult, error) {
	f.outDirs = append(f.outDirs, outDir)
	return f.result, f.err
}

var _ ports.Splitter = (*fakeSplitter)(nil)

func TestRunJob_SuccessMarksCompletedAndLogsFiles(t *testing.T) {
	ctx := context.Background()
	store := &fakeJobStore{}
	job := domain.Job{
		ID:          "job-run-1",
		Fingerprint: "fp-run",
		CuePath:     writeCue(t, twoTrackCue),
		ImagePath:   "/album/album.flac",
		Status:      domain.JobQueued,
		Engine:      "fake",
	}
	if _, err := store.Create(ctx, job); err != nil {
		t.Fatal(err)
	}

	splitter := &fakeSplitter{
		result: ports.SplitResult{
			OutputFiles: []string{"/out/01.flac", "/out/02.flac"},
			Log:         "splitter stdout",
			Duration:    1500 * time.Millisecond,
		},
	}
	inspector := &fakeFLACInspector{info: map[string]ports.FLACInfo{
		job.ImagePath:  {Duration: 6 * time.Second},
		"/out/01.flac": {Duration: 3 * time.Second},
		"/out/02.flac": {Duration: 3 * time.Second},
	}}
	runner := app.JobRunner{
		Store: store, Splitter: splitter, Inspector: inspector,
		Tagger: &fakeFLACTagger{}, ReadFile: os.ReadFile,
	}

	got, err := runner.RunJob(ctx, job, "/out/album", false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.JobCompleted {
		t.Fatalf("status=%q", got.Status)
	}
	if got.Error != "" {
		t.Fatalf("error=%q", got.Error)
	}
	if !strings.Contains(got.Log, "01.flac") || !strings.Contains(got.Log, "02.flac") {
		t.Fatalf("log=%q", got.Log)
	}
	if got.FinishedAt.IsZero() {
		t.Fatal("expected finished_at")
	}
}

func TestRunJob_SplitErrorMarksFailed(t *testing.T) {
	ctx := context.Background()
	store := &fakeJobStore{}
	job := domain.Job{
		ID:          "job-run-2",
		Fingerprint: "fp-run-fail",
		CuePath:     "/album/album.cue",
		ImagePath:   "/album/album.flac",
		Status:      domain.JobQueued,
	}
	if _, err := store.Create(ctx, job); err != nil {
		t.Fatal(err)
	}

	splitter := &fakeSplitter{err: errors.New("split blew up")}
	runner := app.JobRunner{Store: store, Splitter: splitter}

	got, err := runner.RunJob(ctx, job, "/out", false)
	if err == nil {
		t.Fatal("expected error")
	}
	if got.Status != domain.JobFailed {
		t.Fatalf("status=%q", got.Status)
	}
	if got.Error == "" {
		t.Fatal("expected error message on job")
	}
}

func TestRunJob_UsesDistinctPerAlbumOutputDirectories(t *testing.T) {
	ctx := context.Background()
	store := &fakeJobStore{}
	output := "/out/01.flac"
	splitter := &fakeSplitter{result: ports.SplitResult{OutputFiles: []string{output}}}
	jobs := []domain.Job{
		{ID: "job-a", Fingerprint: "album-a", CuePath: writeCue(t, oneTrackCue), ImagePath: "/music/a/album.flac", Status: domain.JobQueued},
		{ID: "job-b", Fingerprint: "album-b", CuePath: writeCue(t, oneTrackCue), ImagePath: "/music/b/album.flac", Status: domain.JobQueued},
	}
	inspector := &fakeFLACInspector{info: map[string]ports.FLACInfo{
		jobs[0].ImagePath: {Duration: 3 * time.Second},
		jobs[1].ImagePath: {Duration: 3 * time.Second},
		output:            {Duration: 3 * time.Second},
	}}
	runner := app.JobRunner{
		Store: store, Splitter: splitter, Inspector: inspector,
		Tagger: &fakeFLACTagger{}, ReadFile: os.ReadFile,
	}
	for _, job := range jobs {
		if _, err := store.Create(ctx, job); err != nil {
			t.Fatal(err)
		}
		if _, err := runner.RunJob(ctx, job, "/out", false); err != nil {
			t.Fatal(err)
		}
	}
	if len(splitter.outDirs) != 2 {
		t.Fatalf("split calls=%d", len(splitter.outDirs))
	}
	if splitter.outDirs[0] == splitter.outDirs[1] {
		t.Fatalf("colliding output dirs: %q", splitter.outDirs[0])
	}
	if splitter.outDirs[0] != filepath.Join("/out", "album-a") {
		t.Fatalf("first output dir=%q", splitter.outDirs[0])
	}
}

func TestRunJob_EmptySplitOutputMarksFailed(t *testing.T) {
	ctx := context.Background()
	store := &fakeJobStore{}
	job := domain.Job{
		ID: "job-empty", Fingerprint: "fp-empty", CuePath: writeCue(t, oneTrackCue),
		ImagePath: "/album/image.flac", Status: domain.JobQueued,
	}
	if _, err := store.Create(ctx, job); err != nil {
		t.Fatal(err)
	}
	runner := app.JobRunner{
		Store: store, Splitter: &fakeSplitter{},
		Inspector: &fakeFLACInspector{info: map[string]ports.FLACInfo{
			job.ImagePath: {Duration: 3 * time.Second},
		}},
		Tagger: &fakeFLACTagger{}, ReadFile: os.ReadFile,
	}

	got, err := runner.RunJob(ctx, job, "/out", false)
	if err == nil {
		t.Fatal("expected empty output error")
	}
	if got.Status != domain.JobFailed {
		t.Fatalf("status=%q, want failed", got.Status)
	}
}

func TestRunJob_VerificationFailureMarksFailed(t *testing.T) {
	tests := []struct {
		name      string
		outputs   []string
		durations map[string]time.Duration
		tagErr    map[string]error
	}{
		{
			name:      "count mismatch",
			outputs:   []string{"/out/01.flac"},
			durations: map[string]time.Duration{"/out/01.flac": 3 * time.Second},
		},
		{
			name:    "duration mismatch",
			outputs: []string{"/out/01.flac", "/out/02.flac"},
			durations: map[string]time.Duration{
				"/out/01.flac": 3200 * time.Millisecond,
				"/out/02.flac": 3 * time.Second,
			},
		},
		{
			name:    "tag error",
			outputs: []string{"/out/01.flac", "/out/02.flac"},
			durations: map[string]time.Duration{
				"/out/01.flac": 3 * time.Second,
				"/out/02.flac": 3 * time.Second,
			},
			tagErr: map[string]error{"/out/02.flac": errors.New("tag failed")},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			store := &fakeJobStore{}
			job := domain.Job{
				ID: "job-" + strings.ReplaceAll(tc.name, " ", "-"), Fingerprint: tc.name,
				CuePath: writeCue(t, twoTrackCue), ImagePath: "/album/image.flac",
				Status: domain.JobQueued,
			}
			if _, err := store.Create(ctx, job); err != nil {
				t.Fatal(err)
			}
			info := map[string]ports.FLACInfo{
				job.ImagePath: {Duration: 6 * time.Second},
			}
			for path, duration := range tc.durations {
				info[path] = ports.FLACInfo{Duration: duration}
			}
			runner := app.JobRunner{
				Store:     store,
				Splitter:  &fakeSplitter{result: ports.SplitResult{OutputFiles: tc.outputs}},
				Inspector: &fakeFLACInspector{info: info},
				Tagger:    &fakeFLACTagger{err: tc.tagErr},
				ReadFile:  os.ReadFile,
			}

			got, err := runner.RunJob(ctx, job, "/out", false)
			if err == nil {
				t.Fatal("expected verification error")
			}
			if got.Status != domain.JobFailed {
				t.Fatalf("status=%q, want failed", got.Status)
			}
			stored, getErr := store.Get(ctx, job.ID)
			if getErr != nil {
				t.Fatal(getErr)
			}
			if stored.Status != domain.JobFailed || stored.Error == "" {
				t.Fatalf("stored job status=%q error=%q", stored.Status, stored.Error)
			}
		})
	}
}

const oneTrackCue = `PERFORMER "Artist"
TITLE "Album"
FILE "image.flac" WAVE
  TRACK 01 AUDIO
    TITLE "One"
    INDEX 01 00:00:00
`
