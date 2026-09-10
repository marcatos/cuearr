package app_test

import (
	"context"
	"errors"
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
		CuePath:     "/album/album.cue",
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

	got, err := app.RunJob(ctx, store, splitter, job, "/out/album", false)
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

	got, err := app.RunJob(ctx, store, splitter, job, "/out", false)
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
	splitter := &fakeSplitter{}
	jobs := []domain.Job{
		{ID: "job-a", Fingerprint: "album-a", CuePath: "/music/a/album.cue", ImagePath: "/music/a/album.flac", Status: domain.JobQueued},
		{ID: "job-b", Fingerprint: "album-b", CuePath: "/music/b/album.cue", ImagePath: "/music/b/album.flac", Status: domain.JobQueued},
	}
	for _, job := range jobs {
		if _, err := store.Create(ctx, job); err != nil {
			t.Fatal(err)
		}
		if _, err := app.RunJob(ctx, store, splitter, job, "/out", false); err != nil {
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
