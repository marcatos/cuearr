package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/app"
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
