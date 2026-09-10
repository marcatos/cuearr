package app_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/app"
	"github.com/marcatos/cuearr/internal/domain"
	"github.com/marcatos/cuearr/internal/ports"
)

type fakeJobStore struct {
	byFP map[string]domain.Job
	mu   sync.Mutex
}

func (f *fakeJobStore) Create(_ context.Context, job domain.Job) (domain.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.byFP == nil {
		f.byFP = make(map[string]domain.Job)
	}
	f.byFP[job.Fingerprint] = job
	return job, nil
}

func (f *fakeJobStore) Get(_ context.Context, id string) (domain.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, j := range f.byFP {
		if j.ID == id {
			return j, nil
		}
	}
	return domain.Job{}, domain.ErrNotFound
}

func (f *fakeJobStore) List(_ context.Context, _ int) ([]domain.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]domain.Job, 0, len(f.byFP))
	for _, j := range f.byFP {
		out = append(out, j)
	}
	return out, nil
}

func (f *fakeJobStore) Update(_ context.Context, job domain.Job) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byFP[job.Fingerprint]; !ok {
		return domain.ErrNotFound
	}
	f.byFP[job.Fingerprint] = job
	return nil
}

func (f *fakeJobStore) FindByFingerprint(_ context.Context, fp string) (domain.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	j, ok := f.byFP[fp]
	if !ok {
		return domain.Job{}, domain.ErrNotFound
	}
	return j, nil
}

func (f *fakeJobStore) ClaimNextQueued(_ context.Context, startedAt time.Time) (domain.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var (
		found domain.Job
		ok    bool
	)
	for _, j := range f.byFP {
		if j.Status != domain.JobQueued {
			continue
		}
		if !ok || j.CreatedAt.Before(found.CreatedAt) {
			found = j
			ok = true
		}
	}
	if !ok {
		return domain.Job{}, domain.ErrNotFound
	}
	found.Status = domain.JobRunning
	found.StartedAt = startedAt
	f.byFP[found.Fingerprint] = found
	return found, nil
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

// toctouJobStore simulates two enqueues racing: first Find misses, Create loses UNIQUE, second Find wins.
type toctouJobStore struct {
	job       domain.Job
	findCalls int
}

func (s *toctouJobStore) Create(_ context.Context, _ domain.Job) (domain.Job, error) {
	return domain.Job{}, domain.ErrConflict
}

func (s *toctouJobStore) Get(_ context.Context, id string) (domain.Job, error) {
	if s.job.ID == id {
		return s.job, nil
	}
	return domain.Job{}, domain.ErrNotFound
}

func (s *toctouJobStore) List(_ context.Context, _ int) ([]domain.Job, error) {
	if s.job.ID == "" {
		return nil, nil
	}
	return []domain.Job{s.job}, nil
}

func (s *toctouJobStore) Update(_ context.Context, job domain.Job) error {
	s.job = job
	return nil
}

func (s *toctouJobStore) FindByFingerprint(_ context.Context, fp string) (domain.Job, error) {
	s.findCalls++
	if s.findCalls == 1 {
		return domain.Job{}, domain.ErrNotFound
	}
	if s.job.Fingerprint == fp {
		return s.job, nil
	}
	return domain.Job{}, domain.ErrNotFound
}

func (s *toctouJobStore) ClaimNextQueued(_ context.Context, _ time.Time) (domain.Job, error) {
	return domain.Job{}, domain.ErrNotFound
}

var _ ports.JobStore = (*toctouJobStore)(nil)

func TestEnqueue_IdempotentOnCreateFingerprintConflict(t *testing.T) {
	ctx := context.Background()
	winner := domain.Job{
		ID:          "winner-id",
		Fingerprint: "fp-race",
		CuePath:     "/a/album.cue",
		ImagePath:   "/a/album.flac",
		Status:      domain.JobQueued,
		Engine:      "shntool",
		CreatedAt:   time.Now().UTC(),
	}
	store := &toctouJobStore{job: winner}
	plan := domain.SplitPlan{
		CuePath:     "/a/album.cue",
		ImagePath:   "/a/album.flac",
		WorkDir:     "/a",
		Fingerprint: "fp-race",
	}

	got, created, err := app.Enqueue(ctx, store, plan, "shntool")
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("expected created=false after fingerprint conflict")
	}
	if got.ID != winner.ID {
		t.Fatalf("id=%q want %q", got.ID, winner.ID)
	}
	if store.findCalls < 2 {
		t.Fatalf("findCalls=%d want >=2", store.findCalls)
	}
}

func TestEnqueue_CreateConflictWithoutExistingJobReturnsConflict(t *testing.T) {
	ctx := context.Background()
	store := &conflictOnlyStore{}
	plan := domain.SplitPlan{Fingerprint: "orphan-fp", CuePath: "/x.cue", ImagePath: "/x.flac", WorkDir: "/"}

	_, _, err := app.Enqueue(ctx, store, plan, "shntool")
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("err=%v want domain.ErrConflict", err)
	}
}

type conflictOnlyStore struct{}

func (conflictOnlyStore) Create(_ context.Context, _ domain.Job) (domain.Job, error) {
	return domain.Job{}, domain.ErrConflict
}

func (conflictOnlyStore) Get(_ context.Context, _ string) (domain.Job, error) {
	return domain.Job{}, domain.ErrNotFound
}

func (conflictOnlyStore) List(_ context.Context, _ int) ([]domain.Job, error) {
	return nil, nil
}

func (conflictOnlyStore) Update(_ context.Context, _ domain.Job) error {
	return domain.ErrNotFound
}

func (conflictOnlyStore) FindByFingerprint(_ context.Context, _ string) (domain.Job, error) {
	return domain.Job{}, domain.ErrNotFound
}

func (conflictOnlyStore) ClaimNextQueued(_ context.Context, _ time.Time) (domain.Job, error) {
	return domain.Job{}, domain.ErrNotFound
}

var _ ports.JobStore = (*conflictOnlyStore)(nil)
