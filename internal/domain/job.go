package domain

import "time"

const (
	JobQueued    = "queued"
	JobRunning   = "running"
	JobCompleted = "completed"
	JobFailed    = "failed"
)

type JobAttempt struct {
	Number int       `json:"n"`
	At     time.Time `json:"at"`
	Error  string    `json:"error"`
}

type Job struct {
	ID, Fingerprint, CuePath, ImagePath, OutDir, Status, Engine, Log, Error string
	CreatedAt, StartedAt, FinishedAt                                        time.Time
	AttemptCount                                                            int
	AttemptLog                                                              []JobAttempt
}
