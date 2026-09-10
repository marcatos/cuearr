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
		if err == nil && got.Status == domain.JobFailed && got.AttemptCount == 3 {
			cancel()
			<-done
			if len(splitter.outDirs) != 3 {
				t.Fatalf("splitter calls=%d want 3", len(splitter.outDirs))
			}
			if len(got.AttemptLog) != 3 {
				t.Fatalf("attempt log=%+v", got.AttemptLog)
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
