package app_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/app"
	"github.com/marcatos/cuearr/internal/domain"
	"github.com/marcatos/cuearr/internal/ports"
)

type recoverableJobStore struct {
	fakeJobStore
	recoverCalls int
}

type attemptObservingSplitter struct {
	store    *fakeJobStore
	jobID    string
	observed chan int
}

func (s *attemptObservingSplitter) Name() string { return "attempt-observer" }

func (s *attemptObservingSplitter) Available(context.Context) error { return nil }

func (s *attemptObservingSplitter) Split(ctx context.Context, _ domain.SplitPlan, _ string) (ports.SplitResult, error) {
	job, err := s.store.Get(ctx, s.jobID)
	if err != nil {
		return ports.SplitResult{}, err
	}
	select {
	case s.observed <- job.AttemptCount:
	default:
	}
	return ports.SplitResult{}, errors.New("observed failure")
}

func (s *recoverableJobStore) RecoverRunning(context.Context) (int64, error) {
	s.recoverCalls++
	return 2, nil
}

func TestWorker_RunRecoversInterruptedJobsAtStartup(t *testing.T) {
	store := &recoverableJobStore{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	worker := &app.Worker{Store: store, Splitter: &fakeSplitter{}, Interval: time.Hour}
	if err := worker.Run(ctx); err != context.Canceled {
		t.Fatalf("Run error=%v", err)
	}
	if store.recoverCalls != 1 {
		t.Fatalf("recover calls=%d", store.recoverCalls)
	}
}

func TestWorker_ClaimProcessesQueuedJob(t *testing.T) {
	cuePath := writeCue(t, oneTrackCue)
	imagePath := "/a.flac"
	outputPath := "/out/01.flac"
	store := &fakeJobStore{byFP: map[string]domain.Job{
		"fp": {
			ID: "job-1", Fingerprint: "fp", Status: domain.JobQueued,
			CuePath: cuePath, ImagePath: imagePath, CreatedAt: time.Now().UTC(),
		},
	}}
	splitter := &fakeSplitter{result: ports.SplitResult{OutputFiles: []string{outputPath}}}
	worker := &app.Worker{
		Store:    store,
		Splitter: splitter,
		Inspector: &fakeFLACInspector{info: map[string]ports.FLACInfo{
			imagePath:  {Duration: 3 * time.Second},
			outputPath: {Duration: 3 * time.Second},
		}},
		Tagger:    &fakeFLACTagger{},
		Preflight: &fakePreflight{},
		ReadFile:  os.ReadFile,
		OutDir:    t.TempDir(),
		Interval:  20 * time.Millisecond,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx) }()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, err := store.Get(context.Background(), "job-1")
		if err == nil && job.Status == domain.JobCompleted {
			cancel()
			<-done
			if len(splitter.outDirs) < 1 {
				t.Fatalf("splitter calls=%d", len(splitter.outDirs))
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	<-done
	t.Fatal("job did not complete")
}

func TestWorker_FailedJobStopsAfterConfiguredTotalAttempts(t *testing.T) {
	cuePath := writeCue(t, oneTrackCue)
	job := domain.Job{
		ID: "job-retry", Fingerprint: "fp-retry", Status: domain.JobQueued,
		CuePath: cuePath, ImagePath: "/a.flac", CreatedAt: time.Now().UTC(),
	}
	store := &fakeJobStore{byFP: map[string]domain.Job{job.Fingerprint: job}}
	splitter := &fakeSplitter{err: errors.New("split failed")}
	runtime := app.NewRuntimeConfig(domain.Settings{MaxRetries: 3}, splitter)
	worker := &app.Worker{
		Store: store, Runtime: runtime, Preflight: &fakePreflight{},
		OutDir: t.TempDir(), Interval: 10 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx) }()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, err := store.Get(context.Background(), job.ID)
		if err == nil && got.Status == domain.JobFailed && got.AttemptCount == 3 && len(got.AttemptLog) == 3 {
			cancel()
			<-done
			if len(splitter.outDirs) != 3 {
				t.Fatalf("splitter calls=%d want 3", len(splitter.outDirs))
			}
			for i, attempt := range got.AttemptLog {
				if attempt.Number != i+1 || attempt.At.IsZero() || attempt.Error != "split failed" {
					t.Fatalf("attempt[%d]=%+v", i, attempt)
				}
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done
	t.Fatal("job did not reach permanent failure")
}

func TestWorker_ManualRequeueAfterExhaustionRunsAgain(t *testing.T) {
	at := time.Date(2026, 9, 10, 18, 0, 0, 0, time.UTC)
	failed := domain.Job{
		ID: "job-manual-retry", Fingerprint: "fp-manual-retry", Status: domain.JobFailed,
		CuePath: writeCue(t, oneTrackCue), ImagePath: "/a.flac", CreatedAt: at,
		AttemptCount: 3,
		AttemptLog: []domain.JobAttempt{
			{Number: 1, At: at, Error: "a"},
			{Number: 2, At: at, Error: "b"},
			{Number: 3, At: at, Error: "c"},
		},
		Error:      "c",
		FinishedAt: at,
	}
	requeued, err := domain.RequeueFailedJob(failed)
	if err != nil {
		t.Fatal(err)
	}
	if requeued.AttemptCount != 0 {
		t.Fatalf("manual requeue attempt count=%d, want 0", requeued.AttemptCount)
	}
	store := &fakeJobStore{byFP: map[string]domain.Job{requeued.Fingerprint: requeued}}
	observed := make(chan int, 1)
	splitter := &attemptObservingSplitter{store: store, jobID: failed.ID, observed: observed}
	worker := &app.Worker{
		Store: store, Runtime: app.NewRuntimeConfig(domain.Settings{MaxRetries: 3}, splitter),
		Preflight: &fakePreflight{}, OutDir: t.TempDir(), Interval: 10 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx) }()

	select {
	case count := <-observed:
		cancel()
		<-done
		if count != 1 {
			t.Fatalf("attempt count at split start=%d, want 1", count)
		}
	case <-time.After(2 * time.Second):
		cancel()
		<-done
		t.Fatal("manual retry did not re-run work")
	}
}

func TestWorker_PersistsAttemptBeforeSplitterStarts(t *testing.T) {
	job := domain.Job{
		ID: "job-attempt-start", Fingerprint: "fp-attempt-start", Status: domain.JobQueued,
		CuePath: writeCue(t, oneTrackCue), ImagePath: "/a.flac", CreatedAt: time.Now().UTC(),
	}
	store := &fakeJobStore{byFP: map[string]domain.Job{job.Fingerprint: job}}
	observed := make(chan int, 1)
	splitter := &attemptObservingSplitter{store: store, jobID: job.ID, observed: observed}
	worker := &app.Worker{
		Store: store, Runtime: app.NewRuntimeConfig(domain.Settings{MaxRetries: 1}, splitter),
		Preflight: &fakePreflight{}, OutDir: t.TempDir(), Interval: 10 * time.Millisecond,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx) }()

	select {
	case count := <-observed:
		cancel()
		<-done
		if count != 1 {
			t.Fatalf("attempt count at split start=%d, want 1", count)
		}
	case <-time.After(2 * time.Second):
		cancel()
		<-done
		t.Fatal("splitter was not called")
	}
}
