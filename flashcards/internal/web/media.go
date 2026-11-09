package web

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/starfederation/datastar-go/datastar"

	"github.com/infinage/go-experiments/flashcards/internal/srs"
	"github.com/infinage/go-experiments/flashcards/internal/store"
)

// readSignals wraps datastar.ReadSignals for short local use.
func readSignals(r *http.Request, target any) error {
	return datastar.ReadSignals(r, target)
}

// cardStatusData backs the edit page's status line fragment.
type cardStatusData struct {
	Line string
	ID   int64
}

// statusLineJSON JSON-quotes a status line for a signals patch.
func statusLineJSON(line string) string {
	b, _ := json.Marshal(line)
	return string(b)
}

func mediaPath(id int64, kind string) string {
	return "/cards/" + strconv.FormatInt(id, 10) + "/media/" + kind
}

// mediaSignals carries the media-delete request parameters.
type mediaSignals struct {
	Kind string `json:"delKind"`
}

// deleteMedia removes a card-front image or audio over SSE.
func (s *Server) deleteMedia(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r)
	if !ok {
		return
	}
	var sig mediaSignals
	if err := readSignals(r, &sig); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var err error
	switch sig.Kind {
	case "image":
		err = s.store.ClearImage(id)
	case "audio":
		err = s.store.ClearAudio(id)
	default:
		http.Error(w, "bad kind", http.StatusBadRequest)
		return
	}
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Clear the preview signals so the UI drops the media immediately,
	// plus the busy signal of the (now replaced) triggering button.
	payload := `{"imgUrl": "", "imgName": "", "busyDelImg": false}`
	if sig.Kind == "audio" {
		payload = `{"audUrl": "", "audName": "", "busyDelAud": false}`
	}
	if err := datastar.NewSSE(w, r).
		PatchSignals([]byte(payload)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// resetCardStatus clears one card's review state over SSE and re-renders
// its status line.
func (s *Server) resetCardStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r)
	if !ok {
		return
	}
	if err := s.store.ResetCardProgress(id); err != nil &&
		!errors.Is(err, store.ErrNotFound) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	line := dueLine(srs.State{}, false)
	frag := s.templateHTML("card-status", cardStatusData{Line: line, ID: id})
	sse := datastar.NewSSE(w, r)
	// Patch the line's signal as well: the span is bound with data-text
	// ($statusLine), so a bare element patch would be overwritten by the
	// stale signal value right after the swap.
	if err := sse.PatchSignals([]byte(`{"busyResetStatus": false, "statusLine": ` +
		statusLineJSON(line) + `}`)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := sse.PatchElements(frag); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Upload limits for card-front media.
const (
	maxImageBytes = 5 << 20  // 5 MB
	maxAudioBytes = 10 << 20 // 10 MB
)

// sniffed is the allow-list of accepted media types, keyed by the MIME
// prefix the sniffers report.
var sniffed = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpeg",
	"image/webp": ".webp",
	"image/gif":  ".gif",
	"audio/mpeg": ".mp3",
	"audio/mp4":  ".m4a",
	"audio/wav":  ".wav",
	"audio/ogg":  ".ogg",
	"audio/webm": ".webm",
	"audio/amr":  ".amr",
	"audio/3gpp": ".3gp",
}

// readUpload validates a multipart file field and returns its bytes and
// sniffed MIME type. kind is "image" or "audio".
func readUpload(r *http.Request, field, kind string) (store.Media, error) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"),
		"multipart/form-data") {
		return store.Media{}, nil // text-only submission
	}
	f, _, err := r.FormFile(field)
	if errors.Is(err, http.ErrMissingFile) {
		return store.Media{}, nil // optional field
	}
	if err != nil {
		return store.Media{}, err
	}
	defer f.Close()

	limit := maxImageBytes
	if kind == "audio" {
		limit = maxAudioBytes
	}
	b, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil {
		return store.Media{}, err
	}
	if len(b) > limit {
		return store.Media{}, errTooLarge(kind, limit)
	}
	if len(b) == 0 {
		return store.Media{}, nil
	}
	mime := sniffMime(b)
	if _, ok := sniffed[mime]; !ok {
		return store.Media{}, errors.New(
			"unsupported " + kind + " type " + mime)
	}
	return store.Media{Bytes: b, Mime: mime}, nil
}

func errTooLarge(kind string, limit int) error {
	return errors.New(kind + " exceeds the " +
		strconv.Itoa(limit>>20) + " MB limit")
}

// sniffMime detects the content type from the bytes themselves, never from
// the client-declared filename or Content-Type.
func sniffMime(b []byte) string {
	if m := sniffImage(b); m != "" {
		return m
	}
	if m := sniffAudio(b); m != "" {
		return m
	}
	return "application/octet-stream"
}

func sniffImage(b []byte) string {
	switch {
	case bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")):
		return "image/png"
	case bytes.HasPrefix(b, []byte("\xff\xd8\xff")):
		return "image/jpeg"
	case bytes.HasPrefix(b, []byte("GIF87a")), bytes.HasPrefix(b, []byte("GIF89a")):
		return "image/gif"
	case len(b) > 12 && bytes.Equal(b[0:4], []byte("RIFF")) &&
		bytes.Equal(b[8:12], []byte("WEBP")):
		return "image/webp"
	}
	return ""
}

func sniffAudio(b []byte) string {
	switch {
	case bytes.HasPrefix(b, []byte("ID3")), bytes.HasPrefix(b, []byte("\xff\xfb")),
		bytes.HasPrefix(b, []byte("\xff\xf3")):
		return "audio/mpeg"
	case bytes.HasPrefix(b, []byte("#!AMR")):
		return "audio/amr"
	case len(b) > 12 && bytes.Equal(b[4:8], []byte("ftyp")) &&
		(len(b) > 16 && (bytes.HasPrefix(b[8:12], []byte("3gp")) ||
			bytes.HasPrefix(b[8:12], []byte("3gr")))):
		// 3GPP container (Android voice recorders emit these).
		return "audio/3gpp"
	case len(b) > 12 && bytes.Equal(b[4:8], []byte("ftyp")):
		return "audio/mp4"
	case bytes.HasPrefix(b, []byte("RIFF")) && len(b) > 12 &&
		bytes.Equal(b[8:12], []byte("WAVE")):
		return "audio/wav"
	case bytes.HasPrefix(b, []byte("OggS")):
		return "audio/ogg"
		// Matroska container: MediaRecorder's default (Chrome and friends
		// emit WebM opus).
	case bytes.HasPrefix(b, []byte{0x1A, 0x45, 0xDF, 0xA3}):
		return "audio/webm"
	}
	return ""
}

// cardMedia serves a card-front image or audio blob, with a strong ETag
// and Range support (audio scrubbing needs it).
func (s *Server) cardMedia(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r)
	if !ok {
		return
	}
	var m store.Media
	var err error
	switch strings.TrimPrefix(r.PathValue("kind"), "") {
	case "image":
		m, err = s.store.Image(id)
	case "audio":
		m, err = s.store.Audio(id)
	default:
		http.NotFound(w, r)
		return
	}
	switch {
	case errors.Is(err, store.ErrNotFound), errors.Is(err, store.ErrNoMedia):
		http.NotFound(w, r)
		return
	case err != nil:
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sum := sha256.Sum256(m.Bytes)
	etag := `"` + hex.EncodeToString(sum[:16]) + `"`
	w.Header().Set("Content-Type", m.Mime)
	w.Header().Set("ETag", etag)
	// no-cache (not max-age): the same URL serves changed bytes after an
	// in-place media replace, so the browser must revalidate every time
	// and the ETag keeps unchanged blobs cheap (304).
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("Accept-Ranges", "bytes")

	if match := r.Header.Get("If-None-Match"); match == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	// A zero modtime keeps Last-Modification out of the equation; the
	// ETag handles revalidation.
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(m.Bytes))
}
