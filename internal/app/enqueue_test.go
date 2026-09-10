package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/app"
	"github.com/marcatos/cuearr/internal/domain"
	"github.com/marcatos/cuearr/internal/ports"
)

type fakeJobStore struct {
	byFP map[string]domain.Job
}

func (f *fakeJobStore) Create(_ context.Context, job domain.Job) (domain.Job, error) {
	if f.byFP == nil {
		f.byFP = make(map[string]domain.Job)
	}
	f.byFP[job.Fingerprint] = job
	return job, nil
}

func (f *fakeJobStore) Get(_ context.Context, id string) (domain.Job, error) {
	for _, j := range f.byFP {
		if j.ID == id {
			return j, nil
		}
	}
	return domain.Job{}, domain.ErrNotFound
}

func (f *fakeJobStore) List(_ context.Context, _ int) ([]domain.Job, error) {
	out := make([]domain.Job, 0, len(f.byFP))
	for _, j := range f.byFP {
		out = append(out, j)
	}
	return out, nil
}

func (f *fakeJobStore) Update(_ context.Context, job domain.Job) error {
	if _, ok := f.byFP[job.Fingerprint]; !ok {
		return domain.ErrNotFound
	}
	f.byFP[job.Fingerprint] = job
	return nil
}

func (f *fakeJobStore) FindByFingerprint(_ context.Context, fp string) (domain.Job, error) {
	j, ok := f.byFP[fp]
	if !ok {
		return domain.Job{}, domain.ErrNotFound
	}
	return j, nil
}

var _ ports.JobStore = (*fakeJobStore)(nil)

func TestEnqueue_IdempotentWhenQueuedOrCompleted(t *testing.T) {
	ctx := context.Background()
	store := &fakeJobStore{}
	plan := domain.SplitPlan{
		CuePath:     "/a/album.cue",
		ImagePath:   "/a/album.flac",
		WorkDir:     "/a",
		Fingerprint: "fp-test-1",
	}

	first, created, err := app.Enqueue(ctx, store, plan, "shntool")
	if err != nil {
		t.Fatal(err)
	}
	if !created || first.Status != domain.JobQueued {
		t.Fatalf("first created=%v status=%q", created, first.Status)
	}

	second, created, err := app.Enqueue(ctx, store, plan, "shntool")
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("second enqueue should not create")
	}
	if second.ID != first.ID {
		t.Fatalf("id=%q want %q", second.ID, first.ID)
	}

	completed := first
	completed.Status = domain.JobCompleted
	completed.FinishedAt = time.Now().UTC()
	if err := store.Update(ctx, completed); err != nil {
		t.Fatal(err)
	}

	third, created, err := app.Enqueue(ctx, store, plan, "shntool")
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("enqueue after completed should not create")
	}
	if third.ID != first.ID {
		t.Fatalf("id=%q want %q", third.ID, first.ID)
	}
}

func TestEnqueue_CreatesWhenFingerprintMissing(t *testing.T) {
	ctx := context.Background()
	store := &fakeJobStore{}
	plan := domain.SplitPlan{Fingerprint: "new-fp", CuePath: "/x.cue", ImagePath: "/x.flac", WorkDir: "/"}
	_, created, err := app.Enqueue(ctx, store, plan, "shntool")
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("expected create on missing fingerprint")
	}
	got, err := store.FindByFingerprint(ctx, "new-fp")
	if err != nil {
		t.Fatal(err)
	}
	if got.Fingerprint != "new-fp" {
		t.Fatalf("fp=%q", got.Fingerprint)
	}
}
