package app_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/app"
	"github.com/marcatos/cuearr/internal/domain"
)

type fakeLidarrClient struct {
	err   error
	paths []string
}

func (f *fakeLidarrClient) Ping(context.Context) error { return nil }

func (f *fakeLidarrClient) RequestImport(_ context.Context, albumPath string) error {
	f.paths = append(f.paths, albumPath)
	return f.err
}

func TestImportService_AfterSplitCompleteRequestsEnabledImport(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.September, 11, 7, 0, 0, 0, time.UTC)
	job := completedImportJob("import-success", domain.ImportNone)
	store := newImportStore(t, job)
	client := &fakeLidarrClient{}
	service := app.ImportService{Store: store, Client: client, Clock: func() time.Time { return now }}

	got, err := service.AfterSplitComplete(ctx, job, enabledImportSettings())
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.JobCompleted || got.ImportStatus != domain.ImportImported {
		t.Fatalf("status=%q import_status=%q", got.Status, got.ImportStatus)
	}
	if len(client.paths) != 1 || client.paths[0] != job.OutDir {
		t.Fatalf("import paths=%v, want [%q]", client.paths, job.OutDir)
	}
	if !got.ImportRequestedAt.Equal(now) || !got.ImportFinishedAt.Equal(now) {
		t.Fatalf("import timestamps requested=%v finished=%v", got.ImportRequestedAt, got.ImportFinishedAt)
	}
	assertStoredJob(t, store, got)
}

func TestImportService_ImportFailureKeepsSplitCompleted(t *testing.T) {
	ctx := context.Background()
	requestErr := errors.New("Lidarr unavailable")
	job := completedImportJob("import-failure", domain.ImportNone)
	store := newImportStore(t, job)
	service := app.ImportService{
		Store: store, Client: &fakeLidarrClient{err: requestErr},
		Clock: func() time.Time { return time.Date(2026, 9, 11, 7, 1, 0, 0, time.UTC) },
	}

	got, err := service.AfterSplitComplete(ctx, job, enabledImportSettings())
	if !errors.Is(err, requestErr) {
		t.Fatalf("error=%v, want %v", err, requestErr)
	}
	if got.Status != domain.JobCompleted {
		t.Fatalf("status=%q, want completed", got.Status)
	}
	if got.ImportStatus != domain.ImportFailed || got.ImportError != requestErr.Error() {
		t.Fatalf("import status=%q error=%q", got.ImportStatus, got.ImportError)
	}
	assertStoredJob(t, store, got)
}

func TestImportService_DisabledImportIsSkipped(t *testing.T) {
	ctx := context.Background()
	job := completedImportJob("import-disabled", domain.ImportNone)
	store := newImportStore(t, job)
	client := &fakeLidarrClient{}
	service := app.ImportService{Store: store, Client: client}

	got, err := service.AfterSplitComplete(ctx, job, domain.Settings{})
	if err != nil {
		t.Fatal(err)
	}
	if got.ImportStatus != domain.ImportSkipped || got.ImportError == "" {
		t.Fatalf("import status=%q error=%q", got.ImportStatus, got.ImportError)
	}
	if len(client.paths) != 0 {
		t.Fatalf("unexpected imports=%v", client.paths)
	}
	assertStoredJob(t, store, got)
}

func TestImportService_InPlaceImportIsSkippedWithReason(t *testing.T) {
	ctx := context.Background()
	job := completedImportJob("import-in-place", domain.ImportNone)
	store := newImportStore(t, job)
	settings := enabledImportSettings()
	settings.InPlace = true

	got, err := (app.ImportService{Store: store, Client: &fakeLidarrClient{}}).
		AfterSplitComplete(ctx, job, settings)
	if err != nil {
		t.Fatal(err)
	}
	if got.ImportStatus != domain.ImportSkipped || got.ImportError != "in-place output" {
		t.Fatalf("import status=%q error=%q", got.ImportStatus, got.ImportError)
	}
}

func TestImportService_RequestedWithinCooldownIsIdempotent(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 11, 7, 2, 0, 0, time.UTC)
	job := completedImportJob("import-cooldown", domain.ImportRequested)
	job.ImportRequestedAt = now.Add(-time.Minute)
	store := newImportStore(t, job)
	client := &fakeLidarrClient{}
	service := app.ImportService{Store: store, Client: client, Clock: func() time.Time { return now }}

	got, err := service.AfterSplitComplete(ctx, job, enabledImportSettings())
	if err == nil {
		t.Fatal("error=nil, want cooldown error")
	}
	if !reflect.DeepEqual(got, job) {
		t.Fatalf("job changed during cooldown: got=%+v want=%+v", got, job)
	}
	if len(client.paths) != 0 {
		t.Fatalf("unexpected imports=%v", client.paths)
	}
}

func TestImportService_RequestImportForJobRequiresCompletedJob(t *testing.T) {
	ctx := context.Background()
	job := completedImportJob("manual-running", domain.ImportNone)
	job.Status = domain.JobRunning
	store := newImportStore(t, job)
	service := app.ImportService{Store: store, Client: &fakeLidarrClient{}}

	if _, err := service.RequestImportForJob(ctx, job.ID, enabledImportSettings()); err == nil {
		t.Fatal("error=nil, want incomplete job error")
	}
}

func TestImportService_PollPendingImportsRetriesEligibleJobsOnly(t *testing.T) {
	ctx := context.Background()
	none := completedImportJob("poll-none", domain.ImportNone)
	failed := completedImportJob("poll-failed", domain.ImportFailed)
	imported := completedImportJob("poll-imported", domain.ImportImported)
	running := completedImportJob("poll-running", domain.ImportNone)
	running.Status = domain.JobRunning
	store := newImportStore(t, none, failed, imported, running)
	client := &fakeLidarrClient{}
	service := app.ImportService{Store: store, Client: client}
	settings := enabledImportSettings()
	settings.LidarrPollIntervalSec = 30

	n, err := service.PollPendingImports(ctx, settings)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 || len(client.paths) != 2 {
		t.Fatalf("processed=%d import paths=%v", n, client.paths)
	}
	for _, job := range []domain.Job{none, failed} {
		got, getErr := store.Get(ctx, job.ID)
		if getErr != nil {
			t.Fatal(getErr)
		}
		if got.ImportStatus != domain.ImportImported {
			t.Fatalf("job %q import status=%q", job.ID, got.ImportStatus)
		}
	}
}

func TestImportService_PollDisabledDoesNotListOrImport(t *testing.T) {
	service := app.ImportService{Store: nil, Client: &fakeLidarrClient{}}
	if n, err := service.PollPendingImports(context.Background(), enabledImportSettings()); err != nil || n != 0 {
		t.Fatalf("processed=%d error=%v", n, err)
	}
}

func completedImportJob(id, importStatus string) domain.Job {
	return domain.Job{
		ID: id, Fingerprint: "fp-" + id, Status: domain.JobCompleted,
		OutDir: "/output/" + id, ImportStatus: importStatus,
	}
}

func enabledImportSettings() domain.Settings {
	return domain.Settings{
		LidarrImportEnabled: true,
		LidarrURL:           "http://lidarr:8686",
		LidarrAPIKey:        "secret",
	}
}

func newImportStore(t *testing.T, jobs ...domain.Job) *fakeJobStore {
	t.Helper()
	store := &fakeJobStore{}
	for _, job := range jobs {
		if _, err := store.Create(context.Background(), job); err != nil {
			t.Fatal(err)
		}
	}
	return store
}

func assertStoredJob(t *testing.T, store *fakeJobStore, want domain.Job) {
	t.Helper()
	got, err := store.Get(context.Background(), want.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stored job=%+v, want %+v", got, want)
	}
}
