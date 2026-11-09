package web

// Picked-media staging. The card form saves over GET (the Android webview
// asset bridge rejects request bodies), so media bytes cannot ride the
// save request — base64 in the signal bulk overflowed the URL and large
// picks silently failed. Picks are instead uploaded once to this staging
// store and the save signals reference them by short URL.
//
// Files land under os.TempDir()/flashcards-stage/<token>, are served back
// under /cards/media/tmp/<token>, and are adopted by saveMediaSignals.

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/infinage/go-experiments/flashcards/internal/store"
)

const (
	stageSizeBytes = 12 << 20 // a little over maxAudioBytes
	stageTTL       = 6 * time.Hour
	stageSweep     = 15 * time.Minute
)

var stageTokenRe = regexp.MustCompile("^[a-f0-9]{24}$")

type staging struct {
	mu       sync.Mutex
	dir      string
	lastSwee time.Time
}

func newStaging() *staging {
	return &staging{dir: filepath.Join(os.TempDir(), "flashcards-stage")}
}

func (st *staging) init() {
	if err := os.MkdirAll(st.dir, 0o700); err != nil {
		log.Printf("staging init: %v", err)
		return
	}
	st.sweepLocked()
}

// sweepLocked drops staged files older than the TTL.
func (st *staging) sweepLocked() {
	entries, err := os.ReadDir(st.dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-stageTTL)
	for _, e := range entries {
		if info, err := e.Info(); err == nil && info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(st.dir, e.Name()))
		}
	}
	st.lastSwee = time.Now()
}

// sweepIfDue rate-limits sweeping to once per stageSweep.
func (st *staging) sweepIfDue() {
	st.mu.Lock()
	defer st.mu.Unlock()
	if time.Since(st.lastSwee) > stageSweep {
		st.sweepLocked()
	}
}

// stageMedia handles POST /cards/media/stage from mediapick.js. No
// datastar semantics: the response is plain JSON.
func (s *Server) stageMedia(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, stageSizeBytes)
	file, head, err := r.FormFile("file")
	if err != nil {
		tooBig := head == nil // missing file field vs. request-size failure
		http.Error(w, fileErr(tooBig), http.StatusBadRequest)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file) // body already capped by MaxBytesReader
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	mime := sniffMime(data)
	kind := "audio"
	if strings.HasPrefix(mime, "image/") {
		kind = "image"
	}
	if err := sniffCheck(data, kind); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tokenBytes := make([]byte, 12)
	if _, err := rand.Read(tokenBytes); err != nil {
		http.Error(w, "staging failed", http.StatusInternalServerError)
		return
	}
	token := hex.EncodeToString(tokenBytes)
	if err := os.WriteFile(filepath.Join(s.stage.dir, token), data, 0o600); err != nil {
		http.Error(w, "staging failed", http.StatusInternalServerError)
		return
	}
	s.stage.sweepIfDue()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"url":  "/cards/media/tmp/" + token,
		"mime": mime,
		"name": head.Filename,
	})
}

// fileErr explains a missing or oversized stage request.
func fileErr(missing bool) string {
	if missing {
		return "no file sent"
	}
	return fmt.Sprintf("file too large (max %d MB)", stageSizeBytes>>20)
}

// serveStaged hands staged bytes back to the browser for the live
// preview between pick and save.
func (s *Server) serveStaged(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if !stageTokenRe.MatchString(token) {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(s.stage.dir, token)
	data, err := os.ReadFile(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", sniffMime(data))
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

// adoptStaged turns a staged URL into store.Media, consuming the file.
// An empty pick gate means no attach; a pick without a staged URL means
// the upload is still in flight and the save must wait.
func (s *Server) adoptStaged(stagedURL, pick string, kind string, limit int) (store.Media, error) {
	token := strings.TrimPrefix(stagedURL, "/cards/media/tmp/")
	if stagedURL == token && pick != "" {
		return store.Media{}, fmt.Errorf(
			"the picked %s is still uploading — try saving again in a moment", kind)
	}
	if pick == "" {
		return store.Media{}, nil // cleared or untouched; defer to mediaKind
	}
	if !stageTokenRe.MatchString(token) {
		return store.Media{}, fmt.Errorf("staged %s reference is invalid — re-pick the file", kind)
	}
	path := filepath.Join(s.stage.dir, token)
	data, err := os.ReadFile(path)
	if err != nil { // staged file lost (GC or restart)
		return store.Media{}, fmt.Errorf("staged %s was lost — re-pick the file", kind)
	}
	if len(data) > limit {
		return store.Media{}, errTooLarge(kind, limit)
	}
	if err := sniffCheck(data, kind); err != nil {
		return store.Media{}, err
	}
	_ = os.Remove(path) // adopted once
	return store.Media{Bytes: data, Mime: sniffMime(data)}, nil
}
