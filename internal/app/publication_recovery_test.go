package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/adapters/store/sqlite"
	"github.com/marcatos/cuearr/internal/domain"
)

func TestPublicationRecoveryAfterRenameAndBeforeCompletion(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	root := t.TempDir()
	job := domain.Job{ID: "last-attempt", Fingerprint: "last-attempt", Status: domain.JobPublishing,
		OutDir: filepath.Join(root, "album"), AttemptCount: 3}
	if _, err := store.Create(ctx, job); err != nil {
		t.Fatal(err)
	}
	staging, err := PrepareStaging(root, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "01.flac"), []byte("verified"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := PublishTracks(staging, job.OutDir, job.ID, []string{"01.flac"}, PublishReplaceDir); err != nil {
		t.Fatal(err)
	}
	// Simulate a crash after the rename, with the completed update still pending.
	w := Worker{Store: store}
	if err := w.recoverPublications(ctx, false); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.JobCompleted || got.AttemptCount != 3 {
		t.Fatalf("recovered=%+v", got)
	}
	if err := w.recoverPublications(ctx, false); err != nil {
		t.Fatal(err)
	}
}

func TestPublicationRecoveryRestoresInterruptedSwap(t *testing.T) {
	root := t.TempDir()
	job := domain.Job{ID: "interrupted", OutDir: filepath.Join(root, "album")}
	aside := filepath.Join(root, ".album.aside-"+job.ID)
	if err := os.MkdirAll(aside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(aside, "original.flac"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	complete, err := recoverPublication(job)
	if err != nil {
		t.Fatal(err)
	}
	if complete {
		t.Fatal("unfinished publication considered complete")
	}
	if _, err := os.Stat(filepath.Join(job.OutDir, "original.flac")); err != nil {
		t.Fatal(err)
	}
}

type rejectCompletionStore struct {
	*sqlite.Store
	reject bool
}

func (s *rejectCompletionStore) Update(ctx context.Context, job domain.Job) error {
	if s.reject && job.Status == domain.JobCompleted {
		return errors.New("database unavailable")
	}
	return s.Store.Update(ctx, job)
}

func TestWorkerRetriesPublicationCompletionWithoutSplitting(t *testing.T) {
	ctx := context.Background()
	base, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer base.Close()
	store := &rejectCompletionStore{Store: base, reject: true}
	root := t.TempDir()
	job := domain.Job{ID: "retry-completion", Fingerprint: "retry-completion", Status: domain.JobPublishing,
		OutDir: filepath.Join(root, "album"), AttemptCount: 3}
	if _, err := store.Create(ctx, job); err != nil {
		t.Fatal(err)
	}
	staging, err := PrepareStaging(root, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "01.flac"), []byte("verified"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := PublishTracks(staging, job.OutDir, job.ID, []string{"01.flac"}, PublishReplaceDir); err != nil {
		t.Fatal(err)
	}
	w := Worker{Store: store, Interval: time.Millisecond}
	if err := w.runOnce(ctx); err == nil {
		t.Fatal("expected completion persistence failure")
	}
	got, err := store.Get(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.JobPublishing {
		t.Fatalf("status=%s", got.Status)
	}
	store.reject = false
	if err := w.runOnce(ctx); err != nil {
		t.Fatal(err)
	}
	got, err = store.Get(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.JobCompleted {
		t.Fatalf("status=%s", got.Status)
	}
}

func TestWorkerTickClaimsQueuedJobWhenPublicationRecoveryFails(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	root := t.TempDir()
	broken := domain.Job{ID: "broken", Fingerprint: "broken", Status: domain.JobPublishing,
		OutDir: filepath.Join(root, "broken")}
	if err := os.MkdirAll(broken.OutDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken.OutDir, publicationFile), []byte("{torn"), 0o644); err != nil {
		t.Fatal(err)
	}
	// An exhausted retry budget lets the tick settle the claimed job without splitting.
	queued := domain.Job{ID: "queued", Fingerprint: "queued", Status: domain.JobQueued,
		AttemptCount: (domain.Settings{}).MaxAttempts()}
	for _, job := range []domain.Job{broken, queued} {
		if _, err := store.Create(ctx, job); err != nil {
			t.Fatal(err)
		}
	}
	w := Worker{Store: store}
	if err := w.runOnce(ctx); err != nil {
		t.Fatalf("runOnce error=%v", err)
	}
	got, err := store.Get(ctx, broken.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.JobFailed || got.Error == "" {
		t.Fatalf("broken=%+v, want failed with error", got)
	}
	got, err = store.Get(ctx, queued.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.JobFailed || got.Error != "retry budget exhausted during crash recovery" {
		t.Fatalf("queued=%+v, want claimed", got)
	}
}
