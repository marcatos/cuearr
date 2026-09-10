package domain_test

import (
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/domain"
)

func TestApplyAttemptFailure_RequeuesUntilMaxAttempts(t *testing.T) {
	at := time.Date(2026, 9, 10, 15, 0, 0, 0, time.UTC)
	job := domain.Job{ID: "j1", Status: domain.JobFailed, Error: "split failed"}

	first := domain.ApplyAttemptFailure(job, "split failed", at, 3)
	if first.Status != domain.JobQueued || first.AttemptCount != 1 {
		t.Fatalf("first=%+v", first)
	}
	if !first.StartedAt.IsZero() || !first.FinishedAt.IsZero() {
		t.Fatalf("timestamps should clear on requeue: %+v", first)
	}
	if len(first.AttemptLog) != 1 || first.AttemptLog[0].Number != 1 || first.AttemptLog[0].Error != "split failed" {
		t.Fatalf("attempt log=%+v", first.AttemptLog)
	}

	second := domain.ApplyAttemptFailure(first, "split failed again", at.Add(time.Minute), 3)
	if second.Status != domain.JobQueued || second.AttemptCount != 2 || len(second.AttemptLog) != 2 {
		t.Fatalf("second=%+v", second)
	}

	third := domain.ApplyAttemptFailure(second, "final failure", at.Add(2*time.Minute), 3)
	if third.Status != domain.JobFailed || third.AttemptCount != 3 {
		t.Fatalf("third=%+v", third)
	}
	if !third.FinishedAt.Equal(at.Add(2 * time.Minute)) {
		t.Fatalf("finished_at=%v", third.FinishedAt)
	}
	if len(third.AttemptLog) != 3 || third.AttemptLog[2].Number != 3 {
		t.Fatalf("attempt log=%+v", third.AttemptLog)
	}
}

func TestApplyAttemptFailure_SingleAttemptWhenMaxIsOne(t *testing.T) {
	at := time.Date(2026, 9, 10, 15, 0, 0, 0, time.UTC)
	job := domain.ApplyAttemptFailure(domain.Job{Status: domain.JobFailed}, "boom", at, 1)
	if job.Status != domain.JobFailed || job.AttemptCount != 1 {
		t.Fatalf("job=%+v", job)
	}
}
