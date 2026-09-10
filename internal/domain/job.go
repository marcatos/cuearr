package domain

import "time"

const (
	JobQueued    = "queued"
	JobRunning   = "running"
	JobCompleted = "completed"
	JobFailed    = "failed"
)

type Job struct {
	ID, Fingerprint, CuePath, ImagePath, OutDir, Status, Engine, Log, Error string
	CreatedAt, StartedAt, FinishedAt                                         time.Time
}
