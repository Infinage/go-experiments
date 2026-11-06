package store

import (
	"testing"
	"time"

	"github.com/infinage/go-experiments/flashcards/internal/srs"
)

func TestQueueIncludesNewCards(t *testing.T) {
	s := mustStore(t)
	s.CreateCard("a", "", "")
	s.CreateCard("b", "", "")
	q, err := s.Queue(false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(q) != 2 || !q[0].New || !q[1].New {
		t.Fatalf("queue: %+v", q)
	}
	st, _ := s.Stats()
	if st.Total != 2 || st.Due != 2 || st.Mastered != 0 {
		t.Fatalf("stats: %+v", st)
	}
}

func TestReviewSchedules(t *testing.T) {
	s := mustStore(t)
	id, _ := s.CreateCard("q", "a", "")

	// New cards are due; after a Good review they are not.
	next, err := s.Review(id, srs.Good, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if next.Interval != 1 {
		t.Fatalf("interval: %d", next.Interval)
	}
	q, _ := s.Queue(false, false)
	if len(q) != 0 {
		t.Fatalf("card should not be due: %+v", q)
	}
	st, _ := s.Stats()
	if st.Due != 0 || st.NextDue.IsZero() {
		t.Fatalf("stats: %+v", st)
	}

	// A logged review exists.
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM review_log WHERE card_id = ?`, id).
		Scan(&n)
	if n != 1 {
		t.Fatalf("review_log rows: %d", n)
	}
}

func TestMasteredCountAndHideMastered(t *testing.T) {
	s := mustStore(t)
	id, _ := s.CreateCard("q", "a", "")
	// Drive the card past the mastered threshold.
	state := srs.New()
	now := time.Now()
	for range 5 {
		state = srs.Review(state, srs.Good, now)
		if state.Interval < 21 {
			now = state.DueAt
		}
	}
	// Force-apply a mature state, due in the past so it sits in the queue.
	state.Interval = 30
	state.DueAt = time.Now().Add(-time.Hour)
	if err := s.applyState(id, state); err != nil {
		t.Fatal(err)
	}
	st, _ := s.Stats()
	if st.Mastered != 1 {
		t.Fatalf("mastered: %d", st.Mastered)
	}
	hidden, _ := s.Queue(true, false)
	if len(hidden) != 0 {
		t.Fatalf("hide-mastered should empty the queue: %+v", hidden)
	}
	shown, _ := s.Queue(false, false)
	if len(shown) != 1 {
		t.Fatalf("queue without filter: %+v", shown)
	}
}

// applyState writes a scheduling state directly (test helper).
func (s *Store) applyState(id int64, st srs.State) error {
	_, err := s.db.Exec(`INSERT INTO review_state
		(card_id, due_at, interval_days, ease, repetitions, lapses)
		VALUES (?,?,?,?,?,?)
		ON CONFLICT(card_id) DO UPDATE SET due_at=excluded.due_at,
			interval_days=excluded.interval_days, ease=excluded.ease,
			repetitions=excluded.repetitions, lapses=excluded.lapses`,
		id, st.DueAt.UTC().Format(time.RFC3339Nano), st.Interval, st.Ease,
		st.Repetitions, st.Lapses)
	return err
}

func TestReviewNewCardStartsFreshState(t *testing.T) {
	s := mustStore(t)
	id, _ := s.CreateCard("q", "a", "")
	// Reviewing twice without the card ever being due: second review
	// builds on the first state.
	next, err := s.Review(id, srs.Good, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	next2, err := s.Review(id, srs.Good, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if next2.Interval != 6 || next2.Interval <= next.Interval {
		t.Fatalf("second review interval: %d", next2.Interval)
	}
}
