package flashcards

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type Card struct {
	ID int64
	Prompt string
	Answer string
	Tags []string
	Username string
}

type User struct {
	Username string
	Password string
}

type CardStat struct {
	User string
	Card int64
	NextReview time.Time
	CurrentIntervalHours uint64
}

func NewStore(dbpath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbpath+"?_fk=1")
	if err != nil {
		return nil, fmt.Errorf("store init: %w", err)
	}

	return &Store{db}, nil
}
