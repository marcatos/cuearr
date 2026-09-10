package sqlite

import "database/sql"

const schema = `
CREATE TABLE IF NOT EXISTS jobs (
	id TEXT PRIMARY KEY,
	fingerprint TEXT NOT NULL UNIQUE,
	cue_path TEXT NOT NULL,
	image_path TEXT NOT NULL,
	out_dir TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL,
	engine TEXT NOT NULL DEFAULT '',
	log_text TEXT NOT NULL DEFAULT '',
	error_text TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	started_at TEXT,
	finished_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_jobs_created_at ON jobs(created_at DESC);

CREATE TABLE IF NOT EXISTS settings (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	payload TEXT NOT NULL DEFAULT '{}'
);
`

func migrate(db *sql.DB) error {
	_, err := db.Exec(schema)
	return err
}
