package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/infinage/go-experiments/flashcards/internal/store"
)

// realTemplates parses the actual assets/html templates from disk.
func realTemplates(t *testing.T) fstest.MapFS {
	t.Helper()
	out := fstest.MapFS{}
	entries, err := os.ReadDir("../../assets/html")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".html") {
			continue
		}
		data, err := os.ReadFile("../../assets/html/" + e.Name())
		if err != nil {
			t.Fatal(err)
		}
		out["html/"+e.Name()] = &fstest.MapFile{Data: data}
	}
	out["css/index.css"] = &fstest.MapFile{Data: []byte("body{}")}
	return out
}

func newHandlerWith(t *testing.T, assets fstest.MapFS) http.Handler {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	h, err := New(db, assets)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func newTestHandler(t *testing.T) http.Handler {
	return newHandlerWith(t, fstest.MapFS{
		"html/index.html": {Data: []byte(`{{define "head"}}{{end}}{{define "nav"}}{{end}}{{define "Study"}}<h1>study</h1>{{end}}`)},
		"css/index.css":   {Data: []byte("body{}")},
	})
}

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

func TestStudyPage(t *testing.T) {
	rec := get(newTestHandler(t), "/")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "study") {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestStaticAssets(t *testing.T) {
	if rec := get(newTestHandler(t), "/assets/css/index.css"); rec.Code != 200 {
		t.Fatalf("got %d", rec.Code)
	}
}

func fstestAssets() fstest.MapFS {
	return fstest.MapFS{
		"html/index.html":     {Data: []byte(`{{define "head"}}{{end}}{{define "nav"}}{{end}}{{define "Home"}}<h1>home</h1>{{end}}`)},
		"html/cards.html":     {Data: []byte(`{{define "Cards"}}cards{{end}}{{define "cards-list"}}list{{end}}`)},
		"html/card_form.html": {Data: []byte(`{{define "NewCard"}}new{{end}}{{define "EditCard"}}edit{{end}}{{define "cardPage"}}page{{end}}{{define "cardForm"}}form{{end}}`)},
		"css/index.css":       {Data: []byte("body{}")},
	}
}

func TestCardsRoutes(t *testing.T) {
	h := func() http.Handler { return newHandlerWith(t, fstestAssets()) }
	if rec := get(h(), "/cards"); rec.Code != 200 || !strings.Contains(rec.Body.String(), "cards") {
		t.Fatalf("/cards: %d %q", rec.Code, rec.Body.String())
	}
	if rec := get(h(), "/cards/new"); rec.Code != 200 || !strings.Contains(rec.Body.String(), "new") {
		t.Fatalf("/cards/new: %d %q", rec.Code, rec.Body.String())
	}
	if rec := get(h(), "/cards/1/edit"); rec.Code != 200 && rec.Code != 404 {
		t.Fatalf("/cards/1/edit: %d", rec.Code)
	}
}
