package main

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/adapters/config"
	"github.com/marcatos/cuearr/internal/adapters/store/sqlite"
	"github.com/marcatos/cuearr/internal/app"
	"github.com/marcatos/cuearr/internal/domain"
)

func TestServeReturnsWorkerRecoveryFailure(t *testing.T) {
	cfg := config.Config{DataDir: t.TempDir(), HTTPAddr: "127.0.0.1:0", Engine: "shntool"}
	store, err := sqlite.Open(filepath.Join(cfg.DataDir, "cuearr.db"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Create(context.Background(), domain.Job{ID: "broken-publication", Fingerprint: "broken-publication", Status: domain.JobPublishing})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err = serve(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), app.ScanAll)
	if err == nil || !strings.Contains(err.Error(), "worker:") {
		t.Fatalf("serve error=%v, want worker failure", err)
	}
	if ctx.Err() != nil {
		t.Fatal("server waited for deadline instead of stopping after worker failure")
	}
}

func TestServeRespondsWhileStartupScanIsBlocked(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	scanStarted := make(chan struct{})
	releaseScan := make(chan struct{})
	serveDone := make(chan error, 1)
	cfg := config.Config{
		HTTPAddr:  addr,
		DataDir:   t.TempDir(),
		WatchDirs: []string{t.TempDir()},
		Engine:    "shntool",
	}
	go func() {
		serveDone <- serve(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), func(context.Context, []string, int, func(string) error) error {
			close(scanStarted)
			<-releaseScan
			return nil
		})
	}()
	t.Cleanup(func() {
		cancel()
		close(releaseScan)
		select {
		case err := <-serveDone:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(5 * time.Second):
			t.Error("server did not shut down")
		}
	})
	select {
	case <-scanStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("startup scan did not start")
	}
	client := &http.Client{Timeout: 250 * time.Millisecond}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		res, err := client.Get("http://" + addr + "/api/v1/health")
		if err == nil {
			res.Body.Close()
			if res.StatusCode != http.StatusOK {
				t.Fatalf("health status=%d", res.StatusCode)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("HTTP server was unavailable while startup scan was blocked")
}

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
