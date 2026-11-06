package store

import (
	"testing"
	"time"

	"github.com/infinage/go-experiments/flashcards/internal/srs"
)

func TestQueueIncludeAllIgnoresDue(t *testing.T) {
	s := mustStore(t)
	id, _ := s.CreateCard("q", "a", "")
	if _, err := s.Review(id, srs.Medium, time.Now()); err != nil {
		t.Fatal(err)
	}
	// Not due -> empty queue normally.
	if q, _ := s.Queue(false, false); len(q) != 0 {
		t.Fatalf("due queue: %+v", q)
	}
	// force includes it.
	q, err := s.Queue(false, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(q) != 1 {
		t.Fatalf("forced queue: %+v", q)
	}
}

func TestCategoryProgresses(t *testing.T) {
	s := mustStore(t)
	a, _ := s.CreateCard("q1", "", "web")
	s.CreateCard("q2", "", "web")
	s.CreateCard("q3", "", "") // uncategorised: excluded
	// Make one web card mastered.
	st := srs.New()
	st.Interval = 30
	st.DueAt = time.Now().Add(-time.Hour)
	if err := s.applyState(a, st); err != nil {
		t.Fatal(err)
	}
	cps, err := s.CategoryProgresses()
	if err != nil {
		t.Fatal(err)
	}
	if len(cps) != 1 || cps[0].Name != "web" || cps[0].Total != 2 ||
		cps[0].Mastered != 1 {
		t.Fatalf("progress: %+v", cps)
	}
}

func TestResetProgress(t *testing.T) {
	s := mustStore(t)
	id, _ := s.CreateCard("q", "a", "web")
	other, _ := s.CreateCard("w", "b", "langs")
	if _, err := s.Review(id, srs.Medium, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Review(other, srs.Medium, time.Now()); err != nil {
		t.Fatal(err)
	}
	// Category-scoped reset: the web card is new again, the langs card
	// keeps its schedule.
	if err := s.ResetProgress("web"); err != nil {
		t.Fatal(err)
	}
	q, _ := s.Queue(false, false)
	if len(q) != 1 || q[0].Category != "web" {
		t.Fatalf("web card should be due again: %+v", q)
	}
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM review_state WHERE card_id = ?`,
		other).Scan(&n)
	if n != 1 {
		t.Fatalf("langs schedule should survive: %d rows", n)
	}
	// Full reset.
	if err := s.ResetProgress(""); err != nil {
		t.Fatal(err)
	}
	q, _ = s.Queue(false, false)
	if len(q) != 2 {
		t.Fatalf("everything should be due again: %+v", q)
	}
	s.db.QueryRow(`SELECT COUNT(*) FROM review_log`).Scan(&n)
	if n != 2 {
		t.Fatalf("review_log rows: %d", n)
	}
}
