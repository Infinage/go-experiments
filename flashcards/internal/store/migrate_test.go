package store

import (
	"path/filepath"
	"testing"
)

// TestUpgradesPreCategoryDatabase opens a database that only has the first
// migration applied (user_version = 1, no category column) and verifies it
// upgrades cleanly.
func TestUpgradesPreCategoryDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	// Recreate a "version 1" database: roll back to the original schema.
	s.db.Exec(`DROP TABLE review_log; DROP TABLE review_state; DROP TABLE cards;`)
	s.db.Exec(`CREATE TABLE cards (
		id INTEGER PRIMARY KEY,
		front_text TEXT NOT NULL,
		back_text TEXT NOT NULL,
		front_image BLOB,
		front_image_mime TEXT,
		front_audio BLOB,
		front_audio_mime TEXT,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	);`)
	s.db.Exec(`PRAGMA user_version = 1;`)
	s.Close()

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen old db: %v", err)
	}
	defer s2.Close()
	var got int
	s2.db.QueryRow("PRAGMA user_version").Scan(&got)
	if got != len(migrations) {
		t.Fatalf("user_version = %d, want %d", got, len(migrations))
	}
	if _, err := s2.CreateCard("q", "a", "web"); err != nil {
		t.Fatalf("category column missing: %v", err)
	}
}
