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

// TestGetCardsByUser verifies that the cards list returns every card owned by
// the user with its tags, and nothing belonging to other users.
func TestGetCardsByUser(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	id1 := seedCard(t, st, "alice", "geo")
	if err := st.CreateCard(ctx, &Card{Prompt: "p2", Answer: "a2", Username: "alice",
		Tags: []string{"math", "geo"}}); err != nil {
		t.Fatalf("CreateCard: %v", err)
	}
	seedCard(t, st, "bob", "other")

	cards, err := st.GetCardsByUser(ctx, "alice")
	if err != nil {
		t.Fatalf("GetCardsByUser: %v", err)
	}
	if len(cards) != 2 {
		t.Fatalf("got %d cards, want 2 (bob's card must be excluded)", len(cards))
	}
	if cards[0].ID != id1 || cards[0].Tags[0] != "geo" {
		t.Errorf("cards[0] = %+v, want id %d with tag geo", cards[0], id1)
	}
	if cards[1].Prompt != "p2" || len(cards[1].Tags) != 2 {
		t.Errorf("cards[1] = %+v, want p2 with 2 tags", cards[1])
	}

	// Edits must round-trip: update the first card's content and tags.
	cards[0].Prompt = "edited"
	cards[0].Answer = "a2"
	cards[0].Tags = []string{"history"}
	if err := st.UpdateCard(ctx, cards[0]); err != nil {
		t.Fatalf("UpdateCard: %v", err)
	}
	got, err := st.GetCardsByUser(ctx, "alice")
	if err != nil {
		t.Fatalf("GetCardsByUser: %v", err)
	}
	if got[0].Prompt != "edited" || len(got[0].Tags) != 1 || got[0].Tags[0] != "history" {
		t.Errorf("after update got %+v, want edited with single history tag", got[0])
	}

	// Deleting must remove stats and tags along with the card.
	if err := st.DeleteCards(ctx, []int64{got[1].ID}); err != nil {
		t.Fatalf("DeleteCards: %v", err)
	}
	got, err = st.GetCardsByUser(ctx, "alice")
	if err != nil {
		t.Fatalf("GetCardsByUser: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("got %d cards after delete, want 1", len(got))
	}
}

// TestSeedDemoData verifies seeding fills an empty database with the demo
// accounts and their cards, and is a no-op when users already exist.
func TestSeedDemoData(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	seeded, err := st.SeedDemoData(ctx)
	if err != nil || !seeded {
		t.Fatalf("first seed = %v, %v; want true, nil", seeded, err)
	}

	for _, want := range []struct {
		user  string
		cards int
	}{
		{"demo", 4}, {"reviewer", 1}, {"test", 2},
	} {
		got, err := st.GetCardsByUser(ctx, want.user)
		if err != nil || len(got) != want.cards {
			t.Fatalf("%s: cards = %d, %v; want %d", want.user, len(got), err, want.cards)
		}
	}

	// Login must work with the documented passwords.
	if _, err := st.GetUser(ctx, "demo"); err != nil {
		t.Fatalf("GetUser demo: %v", err)
	}
	c, err := st.GetUser(ctx, "demo")
	if err != nil {
		t.Fatalf("GetUser demo: %v", err)
	}
	if !checkPassword(c.Password, "demo1234") {
		t.Errorf("seeded demo password does not verify with demo1234")
	}
	if checkPassword(c.Password, "wrongpass") {
		t.Errorf("seeded demo password verifies wrong password")
	}

	// Second run must be a no-op (no duplicate users or cards).
	seeded, err = st.SeedDemoData(ctx)
	if err != nil || seeded {
		t.Errorf("second seed = %v, %v; want false, nil", seeded, err)
	}
	got, _ := st.GetCardsByUser(ctx, "demo")
	if len(got) != 4 {
		t.Errorf("demo has %d cards after re-seed, want 4", len(got))
	}
}
