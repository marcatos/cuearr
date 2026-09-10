package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/marcatos/cuearr/internal/domain"
	"github.com/marcatos/cuearr/internal/ports"
	_ "modernc.org/sqlite"
)

var (
	_ ports.JobStore      = (*Store)(nil)
	_ ports.SettingsStore = (*SettingsStore)(nil)
)

type Store struct {
	db *sql.DB
}

// SettingsStore implements ports.SettingsStore against the same database file.
type SettingsStore struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Settings() *SettingsStore {
	return &SettingsStore{db: s.db}
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Create(ctx context.Context, job domain.Job) (domain.Job, error) {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO jobs (
	id, fingerprint, cue_path, image_path, out_dir, status, engine, log_text, error_text,
	created_at, started_at, finished_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.Fingerprint, job.CuePath, job.ImagePath, job.OutDir, job.Status, job.Engine,
		job.Log, job.Error, formatTime(job.CreatedAt), nullableTime(job.StartedAt), nullableTime(job.FinishedAt),
	)
	if err != nil {
		if isFingerprintUniqueViolation(err) {
			return domain.Job{}, domain.ErrConflict
		}
		return domain.Job{}, fmt.Errorf("insert job: %w", err)
	}
	return job, nil
}

func isFingerprintUniqueViolation(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") && strings.Contains(msg, "fingerprint")
}

func (s *Store) Get(ctx context.Context, id string) (domain.Job, error) {
	return s.scanJob(s.db.QueryRowContext(ctx, jobSelect+" WHERE id = ?", id))
}

func (s *Store) FindByFingerprint(ctx context.Context, fp string) (domain.Job, error) {
	return s.scanJob(s.db.QueryRowContext(ctx, jobSelect+" WHERE fingerprint = ?", fp))
}

func (s *Store) List(ctx context.Context, limit int) ([]domain.Job, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, jobSelect+" ORDER BY created_at DESC LIMIT ?", limit)
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []domain.Job
	for rows.Next() {
		job, err := scanJobRow(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (s *Store) Update(ctx context.Context, job domain.Job) error {
	res, err := s.db.ExecContext(ctx, `
UPDATE jobs SET
	fingerprint = ?, cue_path = ?, image_path = ?, out_dir = ?, status = ?, engine = ?,
	log_text = ?, error_text = ?, created_at = ?, started_at = ?, finished_at = ?
WHERE id = ?`,
		job.Fingerprint, job.CuePath, job.ImagePath, job.OutDir, job.Status, job.Engine,
		job.Log, job.Error, formatTime(job.CreatedAt), nullableTime(job.StartedAt), nullableTime(job.FinishedAt),
		job.ID,
	)
	if err != nil {
		return fmt.Errorf("update job: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update job rows: %w", err)
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) ClaimNextQueued(ctx context.Context, startedAt time.Time) (domain.Job, error) {
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	row := s.db.QueryRowContext(ctx, `
UPDATE jobs
SET status = ?, started_at = ?, finished_at = NULL, error_text = ''
WHERE id = (
	SELECT id FROM jobs WHERE status = ? ORDER BY created_at ASC LIMIT 1
)
RETURNING id, fingerprint, cue_path, image_path, out_dir, status, engine, log_text, error_text,
	created_at, started_at, finished_at`,
		domain.JobRunning, formatTime(startedAt), domain.JobQueued,
	)
	job, err := scanJobRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Job{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Job{}, fmt.Errorf("claim next queued: %w", err)
	}
	return job, nil
}

func (s *Store) RecoverRunning(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
UPDATE jobs
SET status = ?, started_at = NULL, finished_at = NULL,
	error_text = '', log_text = ''
WHERE status = ?`, domain.JobQueued, domain.JobRunning)
	if err != nil {
		return 0, fmt.Errorf("recover running jobs: %w", err)
	}
	count, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("recover running job count: %w", err)
	}
	return count, nil
}

func (s *SettingsStore) Get(ctx context.Context) (domain.Settings, error) {
	var payload string
	err := s.db.QueryRowContext(ctx, `SELECT payload FROM settings WHERE id = 1`).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Settings{}, nil
	}
	if err != nil {
		return domain.Settings{}, fmt.Errorf("get settings: %w", err)
	}
	var settings domain.Settings
	if payload == "" || payload == "{}" {
		return domain.Settings{}, nil
	}
	if err := json.Unmarshal([]byte(payload), &settings); err != nil {
		return domain.Settings{}, fmt.Errorf("decode settings: %w", err)
	}
	return settings, nil
}

func (s *SettingsStore) Put(ctx context.Context, settings domain.Settings) error {
	payload, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO settings (id, payload) VALUES (1, ?)
ON CONFLICT(id) DO UPDATE SET payload = excluded.payload`, string(payload))
	if err != nil {
		return fmt.Errorf("put settings: %w", err)
	}
	return nil
}

const jobSelect = `
SELECT id, fingerprint, cue_path, image_path, out_dir, status, engine, log_text, error_text,
	created_at, started_at, finished_at FROM jobs`

func (s *Store) scanJob(row *sql.Row) (domain.Job, error) {
	job, err := scanJobRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Job{}, domain.ErrNotFound
	}
	return job, err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanJobRow(row rowScanner) (domain.Job, error) {
	var (
		job                        domain.Job
		created, started, finished sql.NullString
	)
	err := row.Scan(
		&job.ID, &job.Fingerprint, &job.CuePath, &job.ImagePath, &job.OutDir, &job.Status, &job.Engine,
		&job.Log, &job.Error, &created, &started, &finished,
	)
	if err != nil {
		return domain.Job{}, err
	}
	var parseErr error
	job.CreatedAt, parseErr = parseTime(created.String)
	if parseErr != nil {
		return domain.Job{}, fmt.Errorf("parse created_at: %w", parseErr)
	}
	if started.Valid {
		job.StartedAt, parseErr = parseTime(started.String)
		if parseErr != nil {
			return domain.Job{}, fmt.Errorf("parse started_at: %w", parseErr)
		}
	}
	if finished.Valid {
		job.FinishedAt, parseErr = parseTime(finished.String)
		if parseErr != nil {
			return domain.Job{}, fmt.Errorf("parse finished_at: %w", parseErr)
		}
	}
	return job, nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, s)
}

func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return formatTime(t)
}
