package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/adapters/store/sqlite"
	"github.com/marcatos/cuearr/internal/domain"
)

func TestReview_SettingsRollbackPreservesConcurrentSuccessfulUpdate(t *testing.T) {
	store, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	if err := store.Settings().Put(ctx, domain.Settings{Engine: "shntool", OutDir: "/original"}); err != nil {
		t.Fatal(err)
	}
	firstApplying := make(chan struct{})
	releaseFirst := make(chan struct{})
	srv := New(Deps{
		Jobs: store, Settings: store.Settings(),
		ApplySettings: func(_ context.Context, next domain.Settings) error {
			if next.OutDir == "/invalid" {
				close(firstApplying)
				<-releaseFirst
				return errors.New("cannot apply first settings")
			}
			return nil
		},
	})
	put := func(out string, done chan<- int) {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/settings",
			strings.NewReader(`{"engine":"shntool","out_dir":"`+out+`"}`))
		rec := httptest.NewRecorder()
		srv.handlePutSettings(rec, req)
		done <- rec.Code
	}
	firstDone := make(chan int, 1)
	secondDone := make(chan int, 1)
	go put("/invalid", firstDone)
	select {
	case <-firstApplying:
	case <-time.After(time.Second):
		close(releaseFirst)
		t.Fatal("first settings request did not reach apply")
	}
	go put("/accepted", secondDone)
	// Let the second request finish while the first is in flight. A correct
	// serialized implementation may instead finish it after first is released.
	var secondStatus int
	select {
	case secondStatus = <-secondDone:
	case <-time.After(100 * time.Millisecond):
	}
	close(releaseFirst)
	select {
	case status := <-firstDone:
		if status != http.StatusBadRequest {
			t.Fatalf("first status=%d, want 400", status)
		}
	case <-time.After(time.Second):
		t.Fatal("first settings request did not finish")
	}
	if secondStatus == 0 {
		select {
		case secondStatus = <-secondDone:
		case <-time.After(time.Second):
			t.Fatal("second settings request did not finish")
		}
	}
	if secondStatus != http.StatusOK {
		t.Fatalf("second status=%d, want 200", secondStatus)
	}
	got, err := store.Settings().Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.OutDir != "/accepted" {
		t.Fatalf("rollback erased successful concurrent settings: out_dir=%q, want /accepted", got.OutDir)
	}
}

func TestBackgroundScanStopsWithServerContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopped := make(chan struct{})
	srv := New(Deps{Context: ctx, WatchDirs: []string{"/watch"}, ScanWatch: func(ctx context.Context) error {
		<-ctx.Done()
		close(stopped)
		return ctx.Err()
	}})
	rec := httptest.NewRecorder()
	srv.handleScanJobs(rec, httptest.NewRequest(http.MethodPost, "/api/v1/jobs/scan", nil))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status=%d", rec.Code)
	}
	cancel()
	done := make(chan struct{})
	go func() { srv.WaitScans(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("background scan did not stop")
	}
	select {
	case <-stopped:
	default:
		t.Fatal("server did not wait for background scan")
	}
}
