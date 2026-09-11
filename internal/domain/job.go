package domain

import "time"

const (
	JobQueued    = "queued"
	JobRunning   = "running"
	JobCompleted = "completed"
	JobFailed    = "failed"

	ImportNone      = "none"
	ImportRequested = "requested"
	ImportImported  = "imported"
	ImportFailed    = "failed"
	ImportSkipped   = "skipped"
)

type JobAttempt struct {
	Number int       `json:"n"`
	At     time.Time `json:"at"`
	Error  string    `json:"error"`
}

type Job struct {
	ID, Fingerprint, CuePath, ImagePath, OutDir, Status, Engine, Log, Error string
	ImportStatus, ImportError                                               string
	CreatedAt, StartedAt, FinishedAt, ImportRequestedAt, ImportFinishedAt   time.Time
	AttemptCount                                                            int
	AttemptLog                                                              []JobAttempt
}
