package web

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/infinage/go-experiments/flashcards/internal/store"
)

func mediaPath(id int64, kind string) string {
	return "/cards/" + strconv.FormatInt(id, 10) + "/media/" + kind
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
	case len(b) > 12 && bytes.Equal(b[4:8], []byte("ftyp")):
		return "audio/mp4"
	case bytes.HasPrefix(b, []byte("RIFF")) && len(b) > 12 &&
		bytes.Equal(b[8:12], []byte("WAVE")):
		return "audio/wav"
	case bytes.HasPrefix(b, []byte("OggS")):
		return "audio/ogg"
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
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Header().Set("Accept-Ranges", "bytes")

	if match := r.Header.Get("If-None-Match"); match == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	// A zero modtime keeps Last-Modification out of the equation; the
	// ETag handles revalidation.
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(m.Bytes))
}
