package flashcards

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := NewStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := st.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return st
}

// seedCard creates the user (needed for the cards FK), then a card with a
// single tag, and returns the card ID.
func seedCard(t *testing.T, st *Store, user, tag string) int64 {
	t.Helper()
	if err := st.CreateUser(context.Background(), User{Username: user, Password: "x"}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	c := Card{Prompt: "p", Answer: "a", Username: user, Tags: []string{tag}}
	if err := st.CreateCard(context.Background(), &c); err != nil {
		t.Fatalf("CreateCard: %v", err)
	}
	return c.ID
}

// fetchStat reads the raw stats row for (user, card).
func fetchStat(t *testing.T, st *Store, user string, card int64) (interval int, nextReview string) {
	t.Helper()
	err := st.db.QueryRow(
		`SELECT current_interval_hours, next_review FROM stats WHERE user = ? AND card = ?`,
		user, card,
	).Scan(&interval, &nextReview)
	if err != nil {
		t.Fatalf("fetch stat: %v", err)
	}
	return interval, nextReview
}

func TestUpsertCardStatIntervalProgression(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	card := seedCard(t, st, "alice", "geo")

	// First correct answer: interval 1 -> 2 (insert path).
	if err := st.UpsertCardStat(ctx, "alice", card, true); err != nil {
		t.Fatalf("first correct: %v", err)
	}
	if interval, _ := fetchStat(t, st, "alice", card); interval != 2 {
		t.Errorf("after first correct: interval = %d, want 2", interval)
	}

	// Second correct answer: interval 2 -> 4.
	if err := st.UpsertCardStat(ctx, "alice", card, true); err != nil {
		t.Fatalf("second correct: %v", err)
	}
	if interval, _ := fetchStat(t, st, "alice", card); interval != 4 {
		t.Errorf("after second correct: interval = %d, want 4", interval)
	}

	// Wrong answer resets to 1.
	if err := st.UpsertCardStat(ctx, "alice", card, false); err != nil {
		t.Fatalf("wrong: %v", err)
	}
	if interval, _ := fetchStat(t, st, "alice", card); interval != 1 {
		t.Errorf("after wrong: interval = %d, want 1", interval)
	}
}

// TestUpsertCardStatConcurrentNoLostUpdates verifies that concurrent correct
// answers do not lose updates. N correct answers must yield an interval of 2^N.
func TestUpsertCardStatConcurrentNoLostUpdates(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	card := seedCard(t, st, "alice", "geo")

	const n = 6
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Go(func() { errs[i] = st.UpsertCardStat(ctx, "alice", card, true) })
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent upsert %d: %v", i, err)
		}
	}

	interval, _ := fetchStat(t, st, "alice", card)
	if want := 1 << n; interval != want {
		t.Errorf("interval after %d concurrent correct answers = %d, want %d "+
			"(a smaller value means an update was lost)", n, interval, want)
	}
}

// TestUpsertCardStatNextReviewMatchesSQLiteClock verifies that next_review
// is stored in a format SQLite interprets consistently, independent of the
// server's local timezone.
func TestUpsertCardStatNextReviewMatchesSQLiteClock(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	card := seedCard(t, st, "alice", "geo")

	if err := st.UpsertCardStat(ctx, "alice", card, true); err != nil {
		t.Fatalf("correct: %v", err)
	}

	var deltaHours float64
	err := st.db.QueryRow(
		`SELECT (julianday(next_review) - julianday('now')) * 24 FROM stats WHERE user = ? AND card = ?`,
		"alice", card,
	).Scan(&deltaHours)
	if err != nil {
		t.Fatalf("julianday delta: %v", err)
	}
	if deltaHours < 1.95 || deltaHours > 2.05 {
		t.Errorf("next_review is %.3f hours out, want ~2 (SQLite must parse the stored datetime as UTC)", deltaHours)
	}

	// The due queries use the same clock, so the card must not be due yet.
	due, err := st.DueCardsByTag(ctx, "alice", "geo")
	if err != nil {
		t.Fatalf("DueCardsByTag: %v", err)
	}
	if len(due) != 0 {
		t.Errorf("got %d due cards right after answering, want 0", len(due))
	}
}
