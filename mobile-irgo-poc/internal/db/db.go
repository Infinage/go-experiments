// Package db opens/reads the POC SQLite database. Throwaway probe code.
package db

import (
	"database/sql"
	"fmt"
	"os"

	_ "modernc.org/sqlite"
)

// overrideDbPath, when non-empty, takes priority over Path()'s rules.
// Set via SetPath (gomobile-exported on the mobile wrapper) because
// Android-only env vars cannot reach the Go runtime.
var overrideDbPath string

// SetPath overrides the db file location (used by the mobile wrapper,
// which receives the app-writable path from native code).
func SetPath(p string) {
	overrideDbPath = p
}

// Path returns the db file path: set override, else env IRGO_DB_FILE, else
// irgo.db in cwd (desktop). On Android the cwd is NOT writable — recorded
// in RESULTS.md; native code must call SetPath before Initialize.
func Path() string {
	if overrideDbPath != "" {
		return overrideDbPath
	}
	if p := os.Getenv("IRGO_DB_FILE"); p != "" {
		return p
	}
	return "irgo.db"
}

// Open opens (creating if needed), migrates and seeds the dict table.
func Open() (*sql.DB, error) {
	d, err := sql.Open("sqlite", Path()+"?_pragma=journal_mode=WAL")
	if err != nil {
		return nil, err
	}
	if _, err := d.Exec(`CREATE TABLE IF NOT EXISTS dict (value TEXT NOT NULL)`); err != nil {
		d.Close()
		return nil, fmt.Errorf("create table: %w", err)
	}
	var n int
	if err := d.QueryRow(`SELECT COUNT(*) FROM dict`).Scan(&n); err != nil {
		d.Close()
		return nil, fmt.Errorf("count: %w", err)
	}
	if n == 0 {
		if _, err := d.Exec(`INSERT INTO dict (value) VALUES ('sqlite-roundtrip-ok')`); err != nil {
			d.Close()
			return nil, fmt.Errorf("seed: %w", err)
		}
	}
	return d, nil
}

// ReadSeed returns SELECT value FROM dict LIMIT 1.
func ReadSeed(d *sql.DB) (string, error) {
	var v string
	if err := d.QueryRow(`SELECT value FROM dict LIMIT 1`).Scan(&v); err != nil {
		return "", fmt.Errorf("read: %w", err)
	}
	return v, nil
}
