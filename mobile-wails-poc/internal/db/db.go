// Package db opens the POC SQLite database, seeds one row and reads it back.
// Pure-Go driver (modernc.org/sqlite) so the same code compiles for android/arm64.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Open opens (creating elsewhere if needed) the database and makes sure the
// `dict` table exists with one seed row. Path "" picks the first writable
// fallback location: $POC_DB_PATH (runner's job) → user cache dir → tmp dir → cwd.
func Open(path string) (*sql.DB, error) {
	if path == "" {
		var err error
		path, err = defaultPath()
		if err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("db dir: %w", err)
	}

	handle, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	if _, err := handle.Exec(`CREATE TABLE IF NOT EXISTS dict(value TEXT)`); err != nil {
		handle.Close()
		return nil, fmt.Errorf("create table: %w", err)
	}

	var n int
	if err := handle.QueryRow(`SELECT COUNT(*) FROM dict`).Scan(&n); err != nil {
		handle.Close()
		return nil, err
	}
	if n == 0 {
		_, err := handle.Exec(`INSERT INTO dict(value) VALUES (?)`,
			"hello from SQLite inside the Wails webview")
		if err != nil {
			handle.Close()
			return nil, fmt.Errorf("seed: %w", err)
		}
	}
	return handle, nil
}

// ReadFirst returns the seeded row's value — the same read is served both
// server-side on first render and over the datastar SSE fragment endpoint.
func ReadFirst(ctx context.Context, handle *sql.DB) (string, error) {
	var v string
	err := handle.QueryRowContext(ctx, `SELECT value FROM dict LIMIT 1`).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errors.New("dict table is empty")
	}
	return v, err
}

func defaultPath() (string, error) {
	candidates := []string{}
	if dir, err := os.UserCacheDir(); err == nil {
		candidates = append(candidates, filepath.Join(dir, "mobile-wails-poc", "poc.db"))
	}
	if dir := os.TempDir(); dir != "" {
		candidates = append(candidates, filepath.Join(dir, "mobile-wails-poc", "poc.db"))
	}
	candidates = append(candidates, "poc.db")
	for _, c := range candidates {
		if err := os.MkdirAll(filepath.Dir(c), 0o755); err != nil {
			continue
		}
		return c, nil
	}
	return "", errors.New("no writable location found for the SQLite file")
}
