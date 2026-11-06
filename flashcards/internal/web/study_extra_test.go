package web

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGradeResetsFlipSignal(t *testing.T) {
	h, _ := realHandler(t)
	req := httptest.NewRequest("POST", "/study/1/grade",
		strings.NewReader(`{"grade": "good", "idx": 0}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Datastar-Request", "true")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), `"revealed": false`) {
		t.Fatalf("revealed not reset: %s", rec.Body.String())
	}
}

func TestResetProgressEndpoint(t *testing.T) {
	h, db := realHandler(t)
	req := httptest.NewRequest("POST", "/study/reset",
		strings.NewReader(`{"category": ""}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Datastar-Request", "true")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("reset: %d", rec.Code)
	}
	// No review state existed, so nothing changes; endpoint must not fail.
	if _, err := db.Card(1); err != nil {
		t.Fatal(err)
	}
}

func TestStudyPageRendersDeckStats(t *testing.T) {
	h, _ := realHandler(t)
	rec := get(h, "/")
	body := rec.Body.String()
	if !strings.Contains(body, "Study Statistics") {
		t.Fatal("missing stats sidebar")
	}
	if !strings.Contains(body, "Not Started") {
		t.Fatal("missing not-started stat")
	}
}
