package web

import (
	"database/sql"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "poc.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	if _, err := database.Exec(`CREATE TABLE dict(value TEXT); INSERT INTO dict(value) VALUES ('seed-value-123');`); err != nil {
		t.Fatal(err)
	}
	assets := os.DirFS("../..") // repo root; New expects the assets/ prefix
	h, err := New(database, assets)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestIndexRendersSQLiteValue(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestHandler(t).index(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 200 {
		t.Fatalf("index status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, "seed-value-123") || !strings.Contains(body, "datastar.js") {
		t.Errorf("index missing db value or datastar script:\n%s", body)
	}
}

func TestFragmentUsesSSEPatch(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/frag", nil)
	req.Header.Set("Datastar-Request", "true")
	newTestHandler(t).fragment(rec, req)
	if rec.Code != 200 {
		t.Fatalf("fragment status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/event-stream") {
		t.Errorf("content type = %q", rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(body, `event: datastar-patch-elements`) ||
		!strings.Contains(body, `data: elements <div id="value-block">`) ||
		!strings.Contains(body, "seed-value-123") {
		t.Errorf("SSE patch not shape-correct:\n%s", body)
	}
}

func TestStaticServesDatastarBundle(t *testing.T) {
	h := newTestHandler(t)
	rec := httptest.NewRecorder()
	h.static(rec, httptest.NewRequest("GET", "/assets/js/datastar.js", nil))
	if rec.Code != 200 {
		t.Fatalf("datastar.js status = %d", rec.Code)
	}
	if rec.Body.Len() < 1024 {
		t.Errorf("datastar.js suspiciously small (%d bytes)", rec.Body.Len())
	}
}
