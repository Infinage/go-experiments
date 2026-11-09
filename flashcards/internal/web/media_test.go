package web

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"

	"github.com/infinage/go-experiments/flashcards/internal/store"
	"testing"
)

// pngBytes renders a valid 1x1 PNG for sniffing tests.
func pngBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// sigSave posts a card save with base64 media, the way formmedia.js
// delivers it (signals on the query string).
func sigSave(h http.Handler, fields map[string]string, files map[string][]byte) *httptest.ResponseRecorder {
	sig := map[string]any{}
	for k, v := range fields {
		sig[k] = v
	}
	for k, data := range files {
		sig[k] = []fileSignal{{
			Name:     "upload",
			Contents: base64.StdEncoding.EncodeToString(data),
		}}
	}
	raw, err := json.Marshal(sig)
	if err != nil {
		panic(err)
	}
	req := httptest.NewRequest("GET", "/cards/save", nil)
	req.Header.Set("Datastar-Request", "true")
	req.URL.RawQuery = "datastar=" + url.QueryEscape(string(raw))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestUploadImageSaves(t *testing.T) {
	h, db := realHandler(t)
	rec := sigSave(h, map[string]string{
		"front": "q", "back": "a",
	}, map[string][]byte{"imgData": pngBytes(t)})
	if rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	cs, _ := db.Cards("", "", "")
	if len(cs) != 2 || !cs[0].HasImage {
		t.Fatalf("cards: %+v", cs)
	}
	m, err := db.Image(cs[0].ID)
	if err != nil || m.Mime != "image/png" {
		t.Fatalf("stored: %v, %v", m, err)
	}

	// The media endpoint serves it with ETag and Range support.
	rec = get(h, "/cards/"+itoa(cs[0].ID)+"/media/image")
	if rec.Code != 200 || !strings.HasPrefix(
		rec.Header().Get("Content-Type"), "image/png") {
		t.Fatalf("media: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("ETag") == "" ||
		rec.Header().Get("Accept-Ranges") != "bytes" {
		t.Fatal("missing ETag or Accept-Ranges")
	}
	// Conditional request.
	etag := rec.Header().Get("ETag")
	req := httptest.NewRequest("GET",
		"/cards/"+itoa(cs[0].ID)+"/media/image", nil)
	req.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req)
	if rec2.Code != 304 {
		t.Fatalf("If-None-Match: %d", rec2.Code)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func TestUploadRejectsBadContent(t *testing.T) {
	h, _ := realHandler(t)
	cases := []struct {
		name  string
		files map[string][]byte
		want  string
	}{
		{"text as image", map[string][]byte{"imgData": []byte("hello")}, "unsupported"},
		{"text as audio", map[string][]byte{"audData": []byte("hello")}, "unsupported"},
	}
	for _, tc := range cases {
		rec := sigSave(h, map[string]string{
			"front": "q", "back": "a",
		}, tc.files)
		if rec.Code != 400 || !strings.Contains(rec.Body.String(), tc.want) {
			t.Errorf("%s: %d %s", tc.name, rec.Code, rec.Body.String())
		}
	}
	// Oversized image: just over the limit.
	big := bytes.Repeat(pngBytes(t), 80000) // > 5 MB of valid PNG
	rec := sigSave(h, map[string]string{
		"front": "q", "back": "a",
	}, map[string][]byte{"imgData": big})
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "limit") {
		t.Errorf("oversize: %d %s", rec.Code, rec.Body.String())
	}
}

func TestMediaMissing(t *testing.T) {
	h, _ := realHandler(t)
	if rec := get(h, "/cards/9999/media/image"); rec.Code != 404 {
		t.Fatalf("missing card: %d", rec.Code)
	}
	if rec := get(h, "/cards/1/media/audio"); rec.Code != 404 {
		t.Fatalf("no audio: %d", rec.Code)
	}
	if rec := get(h, "/cards/1/media/video"); rec.Code != 404 {
		t.Fatalf("bad kind: %d", rec.Code)
	}
}

func TestMediaRangeRequest(t *testing.T) {
	h, db := realHandler(t)
	// Attach known audio bytes to the seeded card (id 1).
	if err := db.SetAudio(1, storeMedia([]byte("0123456789"))); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/cards/1/media/audio", nil)
	req.Header.Set("Range", "bytes=2-5")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 206 || rec.Body.String() != "2345" {
		t.Fatalf("range: %d %q", rec.Code, rec.Body.String())
	}
}

// storeMedia adapts raw bytes for the store API.
func storeMedia(b []byte) store.Media {
	return store.Media{Bytes: b, Mime: "audio/mpeg"}
}
