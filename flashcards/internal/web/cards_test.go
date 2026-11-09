package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/infinage/go-experiments/flashcards/internal/store"
)

// handler backed by the real templates, via a store seeded with one card.
func realHandler(t *testing.T) (http.Handler, *store.Store) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	id, err := db.CreateCard("q1", "a1", "web")
	if err != nil {
		t.Fatal(err)
	}
	_ = id
	h, err := New(db, realTemplates(t))
	if err != nil {
		t.Fatal(err)
	}
	return h, db
}

// postForm fields are sent as urlencoded form POST.
func post(h http.Handler, path string, form url.Values) *httptest.ResponseRecorder {
	return do(h, "POST", path, form)
}

// sigGet sends a datastar GET with signals in the datastar query param
// (the GET-only transport everything uses after the Android bridge change).
func sigGet(h http.Handler, path, signals string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("Datastar-Request", "true")
	req.URL.RawQuery = "datastar=" + url.QueryEscape(signals)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// del sends ids the way Datastar does: signals in the datastar query param
// on a GET (route /cards/delete).
func del(h http.Handler, path string, _ url.Values) *httptest.ResponseRecorder {
	return sigGet(h, "/cards/delete", `{"deleteIds": [1, 2]}`)
}

func do(h http.Handler, method, path string, form url.Values) *httptest.ResponseRecorder {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req := httptest.NewRequest(method, path, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestSaveAndListFlow(t *testing.T) {
	h, db := realHandler(t)
	if rec := sigGet(h, "/cards/save",
		`{"front": "Q", "back": "A", "category": "web"}`); rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	cs, _ := db.Cards("", "", "")
	if len(cs) != 2 || cs[0].Front != "Q" {
		t.Fatalf("cards after save: %+v", cs)
	}
	// Validation error path.
	rec := sigGet(h, "/cards/save", `{"front": "only front"}`)
	if rec.Code != 422 || !strings.Contains(rec.Body.String(), "required") {
		t.Fatalf("validation: %d %s", rec.Code, rec.Body.String())
	}
	// Edit updates.
	if rec := sigGet(h, "/cards/save",
		`{"id": 1, "front": "Q2", "back": "A2", "category": ""}`); rec.Code != 200 {
		t.Fatalf("edit save: %d", rec.Code)
	}
	c, _ := db.Card(1)
	if c.Front != "Q2" {
		t.Fatalf("edit not applied: %+v", c)
	}
	// Bulk delete over SSE (responds 200 with an event stream).
	rec = del(h, "/cards", nil)
	if rec.Code != 200 ||
		!strings.Contains(rec.Body.String(), "datastar-patch-elements") {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	cs, _ = db.Cards("", "", "")
	if len(cs) != 0 {
		t.Fatalf("cards after delete: %d", len(cs))
	}
}

func TestEditMissingCard404(t *testing.T) {
	h, _ := realHandler(t)
	if rec := get(h, "/cards/999/edit"); rec.Code != 404 {
		t.Fatalf("got %d", rec.Code)
	}
	if rec := get(h, "/cards/notanid/edit"); rec.Code != 400 {
		t.Fatalf("got %d", rec.Code)
	}
}
