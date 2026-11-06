// Package store owns the SQLite database.
package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

var errNoRows = sql.ErrNoRows

// migrations are applied in order; the index+1 is the schema version
// recorded in PRAGMA user_version.
var migrations = []string{
	`CREATE TABLE cards (
		id               INTEGER PRIMARY KEY,
		front_text       TEXT NOT NULL,
		back_text        TEXT NOT NULL,
		front_image      BLOB,
		front_image_mime TEXT,
		front_audio      BLOB,
		front_audio_mime TEXT,
		created_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	);
	CREATE TABLE review_state (
		card_id       INTEGER PRIMARY KEY REFERENCES cards(id) ON DELETE CASCADE,
		due_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		interval_days INTEGER NOT NULL DEFAULT 0,
		ease          REAL NOT NULL DEFAULT 2.5,
		repetitions   INTEGER NOT NULL DEFAULT 0,
		lapses        INTEGER NOT NULL DEFAULT 0
	);
	CREATE TABLE review_log (
		id          INTEGER PRIMARY KEY,
		card_id     INTEGER NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
		reviewed_at TEXT NOT NULL,
		grade       INTEGER NOT NULL
	);`,
	`ALTER TABLE cards ADD COLUMN category TEXT;`,
}

type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the database at path and migrates it.
// Use ":memory:" for tests.
func Open(path string) (*Store, error) {
	dsn := path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// A single connection keeps ":memory:" databases coherent and avoids
	// writer contention for a single-user app.
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	for i := version; i < len(migrations); i++ {
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(migrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
