package domain

import "time"

// BeginAttempt records that a run has started and counts toward MaxRetries.
// Crash recovery after this point therefore consumes an attempt.
func BeginAttempt(job Job, at time.Time) Job {
	if at.IsZero() {
		at = time.Now().UTC()
	}
	job.AttemptCount++
	job.Status = JobRunning
	job.StartedAt = at.UTC()
	job.FinishedAt = time.Time{}
	job.Error = ""
	return job
}

// ApplyAttemptFailure records a failed run against the job and either requeues
// it or leaves it failed when AttemptCount reaches maxAttempts.
//
// AttemptCount must already have been incremented by BeginAttempt when work
// started. maxAttempts is the maximum total attempts (including the first).
// When AttemptCount < maxAttempts the job is set back to queued with running
// timestamps cleared while attempt history is preserved.
func ApplyAttemptFailure(job Job, errMsg string, at time.Time, maxAttempts int) Job {
	if maxAttempts < 1 {
		maxAttempts = DefaultMaxRetries
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	if job.AttemptCount < 1 {
		job.AttemptCount = 1
	}

	job.Error = errMsg
	job.AttemptLog = append(append([]JobAttempt(nil), job.AttemptLog...), JobAttempt{
		Number: job.AttemptCount,
		At:     at.UTC(),
		Error:  errMsg,
	})

	if job.AttemptCount < maxAttempts {
		job.Status = JobQueued
		job.StartedAt = time.Time{}
		job.FinishedAt = time.Time{}
		return job
	}

	job.Status = JobFailed
	job.FinishedAt = at.UTC()
	return job
}
