package domain_test

import (
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/domain"
)

func TestApplyAttemptFailure_RequeuesUntilMaxAttempts(t *testing.T) {
	at := time.Date(2026, 9, 10, 15, 0, 0, 0, time.UTC)
	job := domain.BeginAttempt(domain.Job{ID: "j1", Status: domain.JobQueued}, at)

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

	secondAttempt := domain.BeginAttempt(first, at.Add(time.Minute))
	second := domain.ApplyAttemptFailure(secondAttempt, "split failed again", at.Add(time.Minute), 3)
	if second.Status != domain.JobQueued || second.AttemptCount != 2 || len(second.AttemptLog) != 2 {
		t.Fatalf("second=%+v", second)
	}

	thirdAttempt := domain.BeginAttempt(second, at.Add(2*time.Minute))
	third := domain.ApplyAttemptFailure(thirdAttempt, "final failure", at.Add(2*time.Minute), 3)
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
	job := domain.ApplyAttemptFailure(domain.BeginAttempt(domain.Job{Status: domain.JobQueued}, at), "boom", at, 1)
	if job.Status != domain.JobFailed || job.AttemptCount != 1 {
		t.Fatalf("job=%+v", job)
	}
}

func TestRequeueFailedJob_OnlyFailed(t *testing.T) {
	at := time.Date(2026, 9, 10, 16, 0, 0, 0, time.UTC)
	failed := domain.Job{
		ID:           "j1",
		Status:       domain.JobFailed,
		Error:        "split failed",
		Log:          "stderr",
		AttemptCount: 3,
		AttemptLog: []domain.JobAttempt{
			{Number: 1, At: at, Error: "a"},
			{Number: 2, At: at, Error: "b"},
			{Number: 3, At: at, Error: "c"},
		},
		StartedAt:  at,
		FinishedAt: at,
	}
	requeued, err := domain.RequeueFailedJob(failed)
	if err != nil {
		t.Fatal(err)
	}
	if requeued.Status != domain.JobQueued || requeued.Error != "" || requeued.Log != "" {
		t.Fatalf("requeued=%+v", requeued)
	}
	if requeued.AttemptCount != 0 || len(requeued.AttemptLog) != 3 {
		t.Fatalf("attempt history=%+v", requeued)
	}
	if !requeued.StartedAt.IsZero() || !requeued.FinishedAt.IsZero() {
		t.Fatalf("timestamps=%+v %+v", requeued.StartedAt, requeued.FinishedAt)
	}

	if _, err := domain.RequeueFailedJob(domain.Job{ID: "j2", Status: domain.JobQueued}); err == nil {
		t.Fatal("expected conflict for queued job")
	}
}

func TestBeginAttempt_IncrementsBeforeWork(t *testing.T) {
	at := time.Date(2026, 9, 10, 15, 0, 0, 0, time.UTC)
	job := domain.BeginAttempt(domain.Job{ID: "j1", Status: domain.JobQueued}, at)
	if job.Status != domain.JobRunning || job.AttemptCount != 1 || !job.StartedAt.Equal(at) {
		t.Fatalf("job=%+v", job)
	}
}
