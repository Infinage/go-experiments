package web

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/infinage/go-experiments/flashcards/internal/store"
)

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// stageUpload mimics mediapick.js: POST the picked file to the staging
// store and keep the returned URL for the save signals.
func stageUpload(t testing.TB, h http.Handler, kind string, data []byte) string {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "upload."+map[string]string{
		"image": "png", "audio": "bin", "bad": "bin"}[kind])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/cards/media/stage", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("stage: %d %s", rec.Code, rec.Body.String())
	}
	var out struct{ URL string }
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.URL == "" {
		t.Fatal("stage returned no url")
	}
	return out.URL
}

// sigSave posts a card save with staged media URLs on the query string,
// the way the datastar GET save delivers it.
func sigSave(h http.Handler, fields map[string]string) *httptest.ResponseRecorder {
	return sigQuery(h, fields)
}

// sigQuery posts raw signal fields to a datastar GET endpoint.
func sigQuery(h http.Handler, fields map[string]string) *httptest.ResponseRecorder {
	raw, err := json.Marshal(fields)
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
	staged := stageUpload(t, h, "image", pngBytes(t))
	rec := sigSave(h, map[string]string{
		"front": "q", "back": "a",
		"imgPick": "upload.png", "imgStaged": staged,
	})
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

func TestStageRejectsBadContent(t *testing.T) {
	h, _ := realHandler(t)
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"text", []byte("hello"), "unsupported"},
	}
	for _, tc := range cases {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, _ := mw.CreateFormFile("file", "upload.bin")
		fw.Write(tc.data)
		mw.Close()
		req := httptest.NewRequest("POST", "/cards/media/stage", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != 400 || !strings.Contains(rec.Body.String(), tc.want) {
			t.Errorf("%s: %d %s", tc.name, rec.Code, rec.Body.String())
		}
	}
}

func TestSaveRejectsOversizedStagedImage(t *testing.T) {
	h, _ := realHandler(t)
	// A valid PNG just over the 5 MB image limit: staging accepts it
	// (stage cap 12 MB covers audio), the save rejects it.
	big := bytes.Repeat(pngBytes(t), 80000)
	st := stageUpload(t, h, "image", big)
	rec := sigSave(h, map[string]string{
		"front": "q", "back": "a",
		"imgPick": "big.png", "imgStaged": st,
	})
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "limit") {
		t.Errorf("oversize: %d %s", rec.Code, rec.Body.String())
	}
	// Datastar saves surface errors as executed toast(...) calls, not
	// bare status codes (silent 400s had no visible feedback).
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
		t.Errorf("oversize content-type: %q", ct)
	}
}

func TestSavePicklessStagedURLIgnored(t *testing.T) {
	h, db := realHandler(t)
	// Staged data with no pick opt-in: save succeeds, nothing attached.
	staged := stageUpload(t, h, "image", pngBytes(t))
	rec := sigSave(h, map[string]string{
		"front": "q", "back": "a", "imgStaged": staged,
	})
	if rec.Code != 200 {
		t.Fatalf("pick-less save: %d %s", rec.Code, rec.Body.String())
	}
	cs, _ := db.Cards("", "", "")
	if len(cs) != 2 || cs[0].HasImage {
		t.Fatalf("pick-less save attached media: %+v", cs)
	}

	// A pick with no staged URL yet means the upload is still in
	// flight: the save must say so instead of silently dropping media.
	rec = sigQuery(h, map[string]string{
		"front": "q", "back": "a", "imgPick": "busy.png",
	})
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "still uploading") {
		t.Errorf("staging-in-flight: %d %s", rec.Code, rec.Body.String())
	}
}

// A stale staged URL (GC'd or from a previous server run) re-prompt
// instead of failing the whole save with an opaque error.
func TestSaveStagedURLLost(t *testing.T) {
	h, _ := realHandler(t)
	rec := sigQuery(h, map[string]string{
		"front": "q", "back": "a",
		"imgPick":   "x.png",
		"imgStaged": "/cards/media/tmp/deadbeefdeadbeefdeadbeef",
	})
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "re-pick") {
		t.Fatalf("lost staged: %d %s", rec.Code, rec.Body.String())
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
