package flashcards

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Store provides database operations for flashcards, users, and stats.
type Store struct {
	db *sql.DB
}

// Card represents a flashcard item.
type Card struct {
	ID       int64
	Prompt   string
	Answer   string
	Tags     []string
	Username string
}

// User represents a registered system user.
type User struct {
	Username string
	Password string
}

// CardStat represents spaced-repetition statistics for a user's flashcard.
type CardStat struct {
	User                 string
	Card                 int64
	NextReview           time.Time
	CurrentIntervalHours uint64
}

// TagCount is a tag with its total cards and how many are due for a user.
type TagCount struct {
	Name  string
	Total int
	Due   int
}

// NewStore initializes a Store instance given a SQLite database file path.
func NewStore(dbpath string) (*Store, error) {
	// FK enforcement and a 5s busy timeout are enabled: with a connection pool,
	// concurrent writers would otherwise fail immediately with SQLITE_BUSY instead
	// of queueing for the write lock.
	db, err := sql.Open("sqlite", dbpath+"?_fk=1&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("store init: %w", err)
	}

	return &Store{db}, nil
}
