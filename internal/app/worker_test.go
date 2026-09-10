package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/app"
	"github.com/marcatos/cuearr/internal/domain"
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
	store := &fakeJobStore{byFP: map[string]domain.Job{
		"fp": {
			ID: "job-1", Fingerprint: "fp", Status: domain.JobQueued,
			CuePath: "/a.cue", ImagePath: "/a.flac", CreatedAt: time.Now().UTC(),
		},
	}}
	splitter := &fakeSplitter{}
	worker := &app.Worker{
		Store:    store,
		Splitter: splitter,
		OutDir:   t.TempDir(),
		Interval: 20 * time.Millisecond,
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
