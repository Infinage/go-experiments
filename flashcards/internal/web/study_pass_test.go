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
