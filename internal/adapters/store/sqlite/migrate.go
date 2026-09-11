package sqlite

import (
	"database/sql"
	"fmt"
	"strings"
)

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
	import_status TEXT NOT NULL DEFAULT 'none',
	import_error TEXT NOT NULL DEFAULT '',
	import_requested_at TEXT,
	import_finished_at TEXT,
	attempt_count INTEGER NOT NULL DEFAULT 0,
	attempt_log TEXT NOT NULL DEFAULT '[]',
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
	if _, err := db.Exec(schema); err != nil {
		return err
	}
	for _, migration := range []string{
		`ALTER TABLE jobs ADD COLUMN attempt_count INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE jobs ADD COLUMN attempt_log TEXT NOT NULL DEFAULT '[]'`,
		`ALTER TABLE jobs ADD COLUMN import_status TEXT NOT NULL DEFAULT 'none'`,
		`ALTER TABLE jobs ADD COLUMN import_error TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE jobs ADD COLUMN import_requested_at TEXT`,
		`ALTER TABLE jobs ADD COLUMN import_finished_at TEXT`,
	} {
		if _, err := db.Exec(migration); err != nil && !isDuplicateColumn(err) {
			return fmt.Errorf("apply migration %q: %w", migration, err)
		}
	}
	return nil
}

func isDuplicateColumn(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "duplicate column name")
}
