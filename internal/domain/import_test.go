package domain_test

import (
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/domain"
)

func TestBeginImportRequestAllowsNoneAndClearsPreviousError(t *testing.T) {
	now := time.Date(2026, time.September, 11, 8, 0, 0, 0, time.UTC)
	job := domain.Job{ImportStatus: domain.ImportNone, ImportError: "old error"}

	got, err := domain.BeginImportRequest(job, now)
	if err != nil {
		t.Fatalf("BeginImportRequest() error = %v", err)
	}
	if got.ImportStatus != domain.ImportRequested {
		t.Fatalf("ImportStatus = %q, want %q", got.ImportStatus, domain.ImportRequested)
	}
	if got.ImportError != "" {
		t.Fatalf("ImportError = %q, want empty", got.ImportError)
	}
	if !got.ImportRequestedAt.Equal(now) {
		t.Fatalf("ImportRequestedAt = %v, want %v", got.ImportRequestedAt, now)
	}
}

func TestBeginImportRequestAllowsFailedRetry(t *testing.T) {
	now := time.Date(2026, time.September, 11, 8, 0, 0, 0, time.UTC)
	job := domain.Job{ImportStatus: domain.ImportFailed, ImportError: "lidarr unavailable"}

	got, err := domain.BeginImportRequest(job, now)
	if err != nil {
		t.Fatalf("BeginImportRequest() error = %v", err)
	}
	if got.ImportStatus != domain.ImportRequested {
		t.Fatalf("ImportStatus = %q, want %q", got.ImportStatus, domain.ImportRequested)
	}
}

func TestBeginImportRequestRejectsRequestedWithinCooldown(t *testing.T) {
	now := time.Date(2026, time.September, 11, 8, 0, 0, 0, time.UTC)
	job := domain.Job{
		ImportStatus:      domain.ImportRequested,
		ImportRequestedAt: now.Add(-domain.ImportRequestCooldown + time.Second),
	}

	if _, err := domain.BeginImportRequest(job, now); err == nil {
		t.Fatal("BeginImportRequest() error = nil, want cooldown error")
	}
}

func TestBeginImportRequestAllowsRequestedAfterCooldown(t *testing.T) {
	now := time.Date(2026, time.September, 11, 8, 0, 0, 0, time.UTC)
	job := domain.Job{
		ImportStatus:      domain.ImportRequested,
		ImportRequestedAt: now.Add(-domain.ImportRequestCooldown),
	}

	got, err := domain.BeginImportRequest(job, now)
	if err != nil {
		t.Fatalf("BeginImportRequest() error = %v", err)
	}
	if !got.ImportRequestedAt.Equal(now) {
		t.Fatalf("ImportRequestedAt = %v, want %v", got.ImportRequestedAt, now)
	}
}

func TestBeginImportRequestRejectsImported(t *testing.T) {
	job := domain.Job{ImportStatus: domain.ImportImported}

	if _, err := domain.BeginImportRequest(job, time.Now()); err == nil {
		t.Fatal("BeginImportRequest() error = nil, want already imported error")
	}
}

func TestCompleteImportOKMarksImported(t *testing.T) {
	now := time.Date(2026, time.September, 11, 8, 0, 0, 0, time.UTC)
	job := domain.Job{ImportStatus: domain.ImportRequested, ImportError: "old error"}

	got := domain.CompleteImportOK(job, now)

	if got.ImportStatus != domain.ImportImported {
		t.Fatalf("ImportStatus = %q, want %q", got.ImportStatus, domain.ImportImported)
	}
	if got.ImportError != "" {
		t.Fatalf("ImportError = %q, want empty", got.ImportError)
	}
	if !got.ImportFinishedAt.Equal(now) {
		t.Fatalf("ImportFinishedAt = %v, want %v", got.ImportFinishedAt, now)
	}
}

func TestCompleteImportFailRecordsFailure(t *testing.T) {
	now := time.Date(2026, time.September, 11, 8, 0, 0, 0, time.UTC)

	got := domain.CompleteImportFail(domain.Job{ImportStatus: domain.ImportRequested}, now, "request failed")

	if got.ImportStatus != domain.ImportFailed {
		t.Fatalf("ImportStatus = %q, want %q", got.ImportStatus, domain.ImportFailed)
	}
	if got.ImportError != "request failed" {
		t.Fatalf("ImportError = %q, want %q", got.ImportError, "request failed")
	}
	if !got.ImportFinishedAt.Equal(now) {
		t.Fatalf("ImportFinishedAt = %v, want %v", got.ImportFinishedAt, now)
	}
}

func TestSkipImportRecordsReason(t *testing.T) {
	got := domain.SkipImport(domain.Job{}, "connector disabled")

	if got.ImportStatus != domain.ImportSkipped {
		t.Fatalf("ImportStatus = %q, want %q", got.ImportStatus, domain.ImportSkipped)
	}
	if got.ImportError != "connector disabled" {
		t.Fatalf("ImportError = %q, want %q", got.ImportError, "connector disabled")
	}
}
