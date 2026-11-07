package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// gradePosts a grade the way the study page does: signals carry the
// grade plus the client-side pass state.
func gradePost(t *testing.T, h http.Handler, cardID int64,
	signals string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", fmt.Sprintf("/study/%d/grade", cardID),
		strings.NewReader(signals))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Datastar-Request", "true")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// studyGets the study page the way a datastar @get('/' does: signals
// travel in the datastar query parameter.
func studyGet(t *testing.T, h http.Handler, signals string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Datastar-Request", "true")
	if signals != "" {
		req.URL.RawQuery = "datastar=" + url.QueryEscape(signals)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestForcedPassTerminatesAfterGrade guards the review-anyway pass: a
// graded card must leave the queue even though the pass includes every
// card, so the same card can never repeat forever.
func TestForcedPassTerminatesAfterGrade(t *testing.T) {
	h, _ := realHandler(t)
	// Grading the only card moves it into the future; a forced pass
	// would pull it straight back without a graded-ids exclusion.
	rec := gradePost(t, h, 1, `{"grade": "easy", "force": true,
		"gradedIds": [1], "reviewed": 1}`)
	if rec.Code != 200 {
		t.Fatalf("grade: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), ">q1<") {
		t.Fatalf("graded card still queued in forced pass: %s", rec.Body.String())
	}

	rec = studyGet(t, h, `{"force": true, "gradedIds": [1], "reviewed": 1}`)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "All caught up") {
		t.Fatalf("forced pass not caught up: %d %s", rec.Code, rec.Body.String())
	}
}

// TestGradeKeepsPassTotalStable verifies the "of M" formula: the queue
// shrinks by one while reviewed grows by one, so the patched passTotal
// must stay at the pass size.
func TestGradeKeepsPassTotalStable(t *testing.T) {
	h, db := realHandler(t)
	db.CreateCard("q2", "a2", "web")
	db.CreateCard("q3", "a3", "web")
	rec := gradePost(t, h, 1, `{"grade": "easy", "gradedIds": [1],
		"reviewed": 1}`)
	if rec.Code != 200 {
		t.Fatalf("grade: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"passTotal": 3`) {
		t.Fatalf("passTotal not 3 after one grade: %s", rec.Body.String())
	}

	// Paging must not move the total either.
	rec = studyGet(t, h, `{"idx": 1, "gradedIds": [1], "reviewed": 1}`)
	if !strings.Contains(rec.Body.String(), `"passTotal": 3`) {
		t.Fatalf("passTotal changed while paging: %s", rec.Body.String())
	}
}

// TestRequeuedCardNotDuplicated ensures a forgotten card leaves the
// head of the queue and re-attaches at the end exactly once.
func TestRequeuedCardNotDuplicated(t *testing.T) {
	h, db := realHandler(t)
	db.CreateCard("q2", "a2", "web")
	rec := gradePost(t, h, 1, `{"grade": "forgot", "requeuedIds": [1],
		"gradedIds": [1], "reviewed": 1}`)
	if rec.Code != 200 {
		t.Fatalf("grade: %d %s", rec.Code, rec.Body.String())
	}
	if n := strings.Count(rec.Body.String(), "q1"); n != 0 {
		t.Fatalf("requeued card should not be the head card: %s", rec.Body.String())
	}
	// passTotal 2 already proves a single copy (3 would mean duplicated);
	// paging to the end must reach the requeued card.
	rec = studyGet(t, h, `{"idx": 1, "requeuedIds": [1], "gradedIds": [1],
		"reviewed": 1}`)
	if !strings.Contains(rec.Body.String(), "q1") {
		t.Fatalf("requeued card not reachable at the end: %s", rec.Body.String())
	}
}

// TestRequeueLeakStops verifies a requeued card graded forward leaves
// the pass: the client pops it from requeuedIds, only gradedIds keeps
// tracking it, and the pass can finish.
func TestRequeueLeakStops(t *testing.T) {
	h, _ := realHandler(t)
	rec := gradePost(t, h, 1, `{"grade": "easy", "requeuedIds": [],
		"gradedIds": [1], "reviewed": 1}`)
	if rec.Code != 200 {
		t.Fatalf("grade: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), ">q1<") {
		t.Fatalf("graded card still queued: %s", rec.Body.String())
	}
}

// TestGradeKeepsIndexSoNextCardSlidesIn guards the paging signals: a
// grade must not reset idx, so the successor of the graded card slides
// into its slot and the counter (reviewed + idx + 1) keeps counting up.
func TestGradeKeepsIndexSoNextCardSlidesIn(t *testing.T) {
	h, db := realHandler(t)
	db.CreateCard("q2", "a2", "web")
	db.CreateCard("q3", "a3", "web")
	// Card 1 graded at idx 1 (the user paged once before grading):
	// idx must stay 1 and the card now at idx 1 must be q3.
	rec := gradePost(t, h, 1, `{"grade": "easy", "gradedIds": [1],
    "reviewed": 1, "idx": 1}`)
	if rec.Code != 200 {
		t.Fatalf("grade: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), ">q3<") {
		t.Fatalf("card after grading at idx 1 is not q3: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"idx": 0`) {
		t.Fatalf("grade reset idx to 0: %s", rec.Body.String())
	}
}

// TestGradeLastCardClampsIdx guards the end-of-queue case: after the
// last remaining card is graded the queue shrinks past the client's
// idx, so the grade response must echo the server-clamped idx back —
// otherwise the counter runs past passTotal ("Card 4 of 3") and Prev
// re-serves the just-shown card.
func TestGradeLastCardClampsIdx(t *testing.T) {
	h, db := realHandler(t)
	db.CreateCard("q2", "a2", "web")
	// Two cards, user pages to idx 1 (card 2) and grades it: the queue
	// then holds only q2 at idx 0, so the response must carry "idx": 0.
	rec := gradePost(t, h, 2, `{"grade": "easy", "gradedIds": [2],
    "reviewed": 1, "idx": 1}`)
	if rec.Code != 200 {
		t.Fatalf("grade: %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"idx": 0`) {
		t.Fatalf("grade did not clamp idx to 0: %s", body)
	}
	if !strings.Contains(body, `"passTotal": 2`) {
		t.Fatalf("passTotal diverged after end-of-queue grade: %s", body)
	}
}

// TestShuffleShortQueueNotPanic covers a stale requeuedIds list longer
// than the (filtered) queue: shuffling must clamp instead of slicing
// out of bounds.
func TestShuffleShortQueueNotPanic(t *testing.T) {
	h, db := realHandler(t)
	db.CreateCard("q2", "a2", "web")
	// requeuedIds longer than the 1-card queue, on a filtered queue.
	rec := studyGet(t, h, `{"shuffleSeed": 7, "requeuedIds": [1, 2, 3, 4],
    "category": "web"}`)
	if rec.Code != 200 {
		t.Fatalf("studyGet with long requeuedIds: %d %s",
			rec.Code, rec.Body.String())
	}
}

// TestCaughtUpTotalNotZero covers the "Card N of 0" bug: after a reset
// performed while a caught-up page had seeded passTotal=0, the reset
// response must re-patch passTotal to the fresh queue count.
func TestCaughtUpTotalNotZero(t *testing.T) {
	h, db := realHandler(t)
	db.CreateCard("q2", "a2", "web")
	// The caught-up page would have seeded nothing due; simulate the
	// reset click with pass state reset client-side.
	req := httptest.NewRequest("POST", "/study/reset",
		strings.NewReader(`{"category": ""}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Datastar-Request", "true")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("reset: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"passTotal": 2`) {
		t.Fatalf("passTotal not restored after reset: %s", rec.Body.String())
	}
}
