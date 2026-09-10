package domain

import "time"

// ApplyAttemptFailure records a failed run against the job and either requeues
// it or leaves it failed when AttemptCount reaches maxAttempts.
//
// maxAttempts is the maximum total attempts (including the first). When
// AttemptCount < maxAttempts the job is set back to queued with running
// timestamps cleared while attempt history is preserved.
func ApplyAttemptFailure(job Job, errMsg string, at time.Time, maxAttempts int) Job {
	if maxAttempts < 1 {
		maxAttempts = DefaultMaxRetries
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}

	job.AttemptCount++
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
