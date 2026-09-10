package app_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/app"
	"github.com/marcatos/cuearr/internal/domain"
	"github.com/marcatos/cuearr/internal/ports"
)

type fakeSplitter struct {
	result ports.SplitResult
	err    error
}

func (f *fakeSplitter) Name() string { return "fake" }

func (f *fakeSplitter) Available(context.Context) error { return nil }

func (f *fakeSplitter) Split(_ context.Context, _ domain.SplitPlan, _ string) (ports.SplitResult, error) {
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
