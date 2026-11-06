package web

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/infinage/go-experiments/flashcards/internal/srs"
)

var _ = srs.Medium

func TestStudyPageShowsDueCard(t *testing.T) {
	h, db := realHandler(t)
	// Seeded card 1 has no review state, so it's due.
	rec := get(h, "/")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "q1") {
		t.Fatalf("study page: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Tap to reveal answer") {
		t.Fatal("missing reveal button")
	}
	// Empty database state: seed link.
	db.DeleteCard(1)
	rec = get(h, "/")
	if !strings.Contains(rec.Body.String(), "All caught up") ||
		!strings.Contains(rec.Body.String(), "/seed") {
		t.Fatalf("caught-up: %d %s", rec.Code, rec.Body.String())
	}
}

func TestGradeCardFlow(t *testing.T) {
	h, db := realHandler(t)
	// The frontend posts JSON signals with the grade merged in.
	req := httptest.NewRequest("POST", "/study/1/grade",
		strings.NewReader(`{"grade": "good"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Datastar-Request", "true")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 ||
		!strings.Contains(rec.Body.String(), "datastar-patch-elements") {
		t.Fatalf("grade: %d %s", rec.Code, rec.Body.String())
	}
	// The card now has state and is no longer due.
	if _, err := db.Card(1); err != nil {
		t.Fatal(err)
	}
	// The card now has state and is no longer due.
	q, _ := db.Queue(false, false)
	if len(q) != 0 {
		t.Fatalf("card should not be due after Good: %+v", q)
	}

	// Invalid grade.
	req = httptest.NewRequest("POST", "/study/1/grade",
		strings.NewReader(`{"grade": "zzz"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Fatalf("bad grade: %d", rec.Code)
	}
	_ = srs.Medium
}

func TestSeedOnlyWhenEmpty(t *testing.T) {
	h, db := realHandler(t)
	// DB already has a card: seeding must be a no-op redirect.
	rec := post(h, "/seed", url.Values{})
	if rec.Code != 303 {
		t.Fatalf("seed non-empty: %d", rec.Code)
	}
	cs, _ := db.Cards("", "", "")
	if len(cs) != 1 {
		t.Fatalf("cards after refused seed: %d", len(cs))
	}
	// Empty DB: seeding adds the sample set.
	db.DeleteCard(1)
	rec = post(h, "/seed", url.Values{})
	if rec.Code != 303 {
		t.Fatalf("seed: %d", rec.Code)
	}
	cs, _ = db.Cards("", "", "")
	if len(cs) != 58 {
		t.Fatalf("cards after seed: %d", len(cs))
	}
}
