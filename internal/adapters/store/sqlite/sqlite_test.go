package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/adapters/store/sqlite"
	"github.com/marcatos/cuearr/internal/domain"
	_ "modernc.org/sqlite"
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
	completed.AttemptCount = 1
	completed.AttemptLog = []domain.JobAttempt{{
		Number: 1,
		At:     finished,
		Error:  "first failure",
	}}
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
	if stored.AttemptCount != 1 || len(stored.AttemptLog) != 1 ||
		stored.AttemptLog[0].Number != 1 || stored.AttemptLog[0].Error != "first failure" ||
		!stored.AttemptLog[0].At.Equal(finished) {
		t.Fatalf("attempt history: %+v", stored)
	}
}

func TestOpen_MigratesLegacyJobsWithAttemptColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
CREATE TABLE jobs (
	id TEXT PRIMARY KEY,
	fingerprint TEXT NOT NULL UNIQUE,
	cue_path TEXT NOT NULL,
	image_path TEXT NOT NULL,
	out_dir TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL,
	engine TEXT NOT NULL DEFAULT '',
	log_text TEXT NOT NULL DEFAULT '',
	error_text TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	started_at TEXT,
	finished_at TEXT
);
CREATE TABLE settings (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	payload TEXT NOT NULL DEFAULT '{}'
);`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("open migrated store: %v", err)
	}
	defer store.Close()
	job := domain.Job{
		ID: "migrated", Fingerprint: "legacy-fp", CuePath: "/a.cue", ImagePath: "/a.flac",
		Status: domain.JobQueued, CreatedAt: time.Now().UTC(),
	}
	if _, err := store.Create(context.Background(), job); err != nil {
		t.Fatalf("create after migration: %v", err)
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

func TestJobStore_RecoverRunningRequeuesInterruptedJobs(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	job := domain.Job{
		ID:          "interrupted",
		Fingerprint: "fp-interrupted",
		CuePath:     "/music/album.cue",
		ImagePath:   "/music/album.flac",
		Status:      domain.JobRunning,
		CreatedAt:   time.Now().UTC(),
		StartedAt:   time.Now().UTC(),
	}
	if _, err := store.Create(ctx, job); err != nil {
		t.Fatal(err)
	}
	recovered, err := store.RecoverRunning(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if recovered != 1 {
		t.Fatalf("recovered=%d", recovered)
	}
	got, err := store.Get(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.JobQueued || !got.StartedAt.IsZero() {
		t.Fatalf("recovered job=%+v", got)
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
	if defaults.MaxRetries != domain.DefaultMaxRetries {
		t.Fatalf("default max_retries=%d want %d", defaults.MaxRetries, domain.DefaultMaxRetries)
	}

	want := domain.Settings{
		WatchDirs:  []string{"/watch/a", "/watch/b"},
		OutDir:     "/out",
		InPlace:    true,
		Engine:     "native",
		MaxRetries: 5,
		Auth:       domain.AuthSettings{},
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
	if got.OutDir != want.OutDir || got.InPlace != want.InPlace || got.Engine != want.Engine ||
		got.MaxRetries != want.MaxRetries {
		t.Fatalf("settings: %+v", got)
	}
}

func TestJobStore_ClaimNextQueued_AtomicOldest(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	older := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	_, err := store.Create(ctx, domain.Job{
		ID: "q1", Fingerprint: "fp1", CuePath: "/a.cue", ImagePath: "/a.flac",
		Status: domain.JobQueued, Engine: "shntool", CreatedAt: older,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Create(ctx, domain.Job{
		ID: "q2", Fingerprint: "fp2", CuePath: "/b.cue", ImagePath: "/b.flac",
		Status: domain.JobQueued, Engine: "shntool", CreatedAt: newer,
	})
	if err != nil {
		t.Fatal(err)
	}

	started := newer.Add(time.Minute)
	first, err := store.ClaimNextQueued(ctx, started)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != "q1" || first.Status != domain.JobRunning {
		t.Fatalf("first claim=%+v", first)
	}
	second, err := store.ClaimNextQueued(ctx, started.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != "q2" {
		t.Fatalf("second claim id=%s", second.ID)
	}
	_, err = store.ClaimNextQueued(ctx, started)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("empty queue err=%v", err)
	}
}
