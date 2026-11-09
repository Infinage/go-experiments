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

func TestCardsByIDs(t *testing.T) {
	s := mustStore(t)
	a, _ := s.CreateCard("q1", "", "web")
	b, _ := s.CreateCard("q2", "", "web")
	s.Review(a, srs.Medium, time.Now())

	cs, err := s.CardsByIDs([]int64{a, b, 999})
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 2 {
		t.Fatalf("got %d cards", len(cs))
	}
	// A single-id request must return that card; an arg/placeholder
	// mismatch here consumed ids and once returned nothing.
	single, err := s.CardsByIDs([]int64{a})
	if err != nil {
		t.Fatal(err)
	}
	if len(single) != 1 || single[0].ID != a {
		t.Fatalf("singleton request lost the card: %+v", single)
	}
	byID := map[int64]StudyCard{cs[0].ID: cs[0], cs[1].ID: cs[1]}
	if byID[a].State.Interval != 1 {
		t.Fatalf("state not loaded: %+v", byID[a])
	}
	if byID[b].New != true {
		t.Fatal("unreviewed card should be New")
	}
}

// Untagged cards must be selectable via the NoneFilter sentinel: the
// cards list, the study queue and reset all key off category = "".
func TestNoneFilterFlows(t *testing.T) {
	s := mustStore(t)
	tagged, _ := s.CreateCard("q1", "", "web")
	untagged, _ := s.CreateCard("q2", "", "")

	n, err := s.UntaggedCount()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("untagged = %d, want 1", n)
	}

	cs, err := s.Cards("", NoneFilter, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 1 || cs[0].ID != untagged {
		t.Fatalf("None-filtered cards: %+v", cs)
	}

	// Pending counts group untagged under the "" name.
	s.Review(tagged, srs.Easy, time.Now()) // 1-day interval, due later
	pending, err := s.CategoryPending()
	if err != nil {
		t.Fatal(err)
	}
	// The just-reviewed card is due tomorrow, so the "web" group
	// disappears entirely (GROUP BY lists only non-empty groups; the
	// dropdown renders the missing count as "-").
	want := map[string]int{"": 1} // untagged card was never reviewed
	if len(pending) != 1 {
		t.Fatalf("pending groups: %+v", pending)
	}
	for _, c := range pending {
		if want[c.Name] != c.Count {
			t.Fatalf("pending[%q] = %d, want %d", c.Name, c.Count, want[c.Name])
		}
	}

	// Reset with the sentinel clears only the untagged cards' state.
	if err := s.ResetProgress(NoneFilter); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.ReviewState(untagged); ok {
		t.Fatal("untagged card should have lost its review state")
	}
	if _, ok, _ := s.ReviewState(tagged); !ok {
		t.Fatal("tagged card should keep its review state")
	}
}
