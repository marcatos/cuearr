package main

import (
	"context"
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/app"
	"github.com/marcatos/cuearr/internal/domain"
)

func TestWaitForImportPollUsesLiveIntervalAndDisable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runtimeCfg := app.NewRuntimeConfig(domain.Settings{LidarrPollIntervalSec: 10}, nil)
	intervalRead := make(chan int, 3)
	type result struct {
		settings domain.Settings
		ready    bool
	}
	resultCh := make(chan result, 1)
	go func() {
		settings, ready := waitForImportPoll(ctx, runtimeCfg, func(seconds int) time.Duration {
			intervalRead <- seconds
			if seconds == 10 {
				return time.Second
			}
			return 5 * time.Millisecond
		})
		resultCh <- result{settings: settings, ready: ready}
	}()

	if got := <-intervalRead; got != 10 {
		t.Fatalf("initial interval=%d, want 10", got)
	}
	runtimeCfg.Apply(domain.Settings{}, nil)
	select {
	case got := <-resultCh:
		t.Fatalf("poll fired while disabled: %+v", got)
	case <-time.After(25 * time.Millisecond):
	}

	runtimeCfg.Apply(domain.Settings{LidarrPollIntervalSec: 3}, nil)
	select {
	case got := <-intervalRead:
		if got != 3 {
			t.Fatalf("updated interval=%d, want 3", got)
		}
	case <-time.After(time.Second):
		t.Fatal("updated interval was not read")
	}
	select {
	case got := <-resultCh:
		if !got.ready || got.settings.LidarrPollIntervalSec != 3 {
			t.Fatalf("poll result=%+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("poll did not use updated interval")
	}
}
