package sqlite_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/adapters/store/sqlite"
	"github.com/marcatos/cuearr/internal/domain"
)

func openTestStore(t *testing.T) *sqlite.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cuearr.db")
	store, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestJobStore_createFindUpdateList(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	created := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	job := domain.Job{
		ID:          "job-1",
		Fingerprint: "fp-album-a",
		CuePath:     "/music/a/album.cue",
		ImagePath:   "/music/a/album.flac",
		OutDir:      "/out/a",
		Status:      domain.JobQueued,
		Engine:      "shntool",
		CreatedAt:   created,
	}

	got, err := store.Create(ctx, job)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if got.ID != job.ID || got.Fingerprint != job.Fingerprint || got.Status != domain.JobQueued {
		t.Fatalf("create returned %+v", got)
	}

	byFP, err := store.FindByFingerprint(ctx, "fp-album-a")
	if err != nil {
		t.Fatalf("find by fingerprint: %v", err)
	}
	if byFP.ID != job.ID {
		t.Fatalf("find by fingerprint id=%q want %q", byFP.ID, job.ID)
	}

	started := created.Add(2 * time.Minute)
	running := byFP
	running.Status = domain.JobRunning
	running.StartedAt = started
	if err := store.Update(ctx, running); err != nil {
		t.Fatalf("update running: %v", err)
	}

	finished := started.Add(5 * time.Minute)
	completed := running
	completed.Status = domain.JobCompleted
	completed.FinishedAt = finished
	completed.Log = "split ok: 12 tracks"
	if err := store.Update(ctx, completed); err != nil {
		t.Fatalf("update completed: %v", err)
	}

	older := domain.Job{
		ID:          "job-0",
		Fingerprint: "fp-album-z",
		CuePath:     "/music/z/album.cue",
		ImagePath:   "/music/z/album.flac",
		Status:      domain.JobQueued,
		Engine:      "shntool",
		CreatedAt:   created.Add(-1 * time.Hour),
	}
	if _, err := store.Create(ctx, older); err != nil {
		t.Fatalf("create older: %v", err)
	}

	list, err := store.List(ctx, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("list len=%d want 2", len(list))
	}
	if list[0].ID != "job-1" || list[1].ID != "job-0" {
		t.Fatalf("list order: got [%s, %s] want [job-1, job-0]", list[0].ID, list[1].ID)
	}
	if list[0].Status != domain.JobCompleted || list[0].Log != completed.Log {
		t.Fatalf("newest job: %+v", list[0])
	}

	stored, err := store.Get(ctx, "job-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !stored.StartedAt.Equal(started) || !stored.FinishedAt.Equal(finished) {
		t.Fatalf("times: started=%v finished=%v", stored.StartedAt, stored.FinishedAt)
	}
}

func TestJobStore_getNotFound(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	_, err := store.Get(ctx, "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("get missing: err=%v want domain.ErrNotFound", err)
	}
}

func TestSettingsStore_putGet(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	settingsStore := store.Settings()
	defaults, err := settingsStore.Get(ctx)
	if err != nil {
		t.Fatalf("get empty: %v", err)
	}
	if len(defaults.WatchDirs) != 0 || defaults.OutDir != "" || defaults.Engine != "" {
		t.Fatalf("defaults: %+v", defaults)
	}

	want := domain.Settings{
		WatchDirs: []string{"/watch/a", "/watch/b"},
		OutDir:    "/out",
		InPlace:   true,
		Engine:    "native",
		Auth:      domain.AuthSettings{},
	}
	if err := settingsStore.Put(ctx, want); err != nil {
		t.Fatalf("put: %v", err)
	}

	got, err := settingsStore.Get(ctx)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got.WatchDirs) != 2 || got.WatchDirs[0] != "/watch/a" {
		t.Fatalf("watch dirs: %+v", got.WatchDirs)
	}
	if got.OutDir != want.OutDir || got.InPlace != want.InPlace || got.Engine != want.Engine {
		t.Fatalf("settings: %+v", got)
	}
}
