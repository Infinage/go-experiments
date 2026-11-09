// Package web serves the POC UI: an index page rendered server-side with the
// SQLite value, a static datastar bundle, and an SSE fragment endpoint that
// re-reads the same row and patches it into the DOM via datastar.
package web

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"net/http"
	"time"

	"github.com/starfederation/datastar-go/datastar"
)

type Handler struct {
	database *sql.DB
	assets   fs.FS
	tpl      *pageTemplates
}

// New takes a project-root FS that contains the assets/ directory (embed FS
// or repo root). Assets stay served and parsed under their assets/ prefix.
func New(database *sql.DB, assets fs.FS) (*Handler, error) {
	tpl, err := parseTemplates(assets)
	if err != nil {
		return nil, err
	}
	return &Handler{database: database, assets: assets, tpl: tpl}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/frag":
		h.fragment(w, r)
	case "/", "/index.html":
		h.index(w, r)
	default:
		h.static(w, r)
	}
}

// index is the first server-side render; the same db read that /frag exercises.
func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	value, err := readFirst(r.Context(), h.database)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := h.tpl.execute(w, "index.html", view{Value: value}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// fragment handles the datastar @get: render the block fresh, patch via SSE.
func (h *Handler) fragment(w http.ResponseWriter, r *http.Request) {
	value, err := readFirst(r.Context(), h.database)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := h.tpl.execute(&buf, "fragment.html", view{
		Value: value,
		Time:  time.Now().Format("15:04:05"),
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sse := datastar.NewSSE(w, r)
	if err := sse.PatchElements(buf.String()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) static(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Path
	if len(name) > 1 && name[0] == '/' {
		name = name[1:]
	}
	// The FS root (embed.FS or repo root) already contains the assets/ prefix.
	if _, err := h.assets.Open(name); err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFileFS(w, r, h.assets, name)
}

// readFirst returns the seeded row's value — the identical read is served both
// server-side on first render and over the datastar SSE fragment endpoint.
func readFirst(ctx context.Context, database *sql.DB) (string, error) {
	var v string
	err := database.QueryRowContext(ctx, `SELECT value FROM dict LIMIT 1`).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errors.New("dict table is empty")
	}
	return v, err
}

type view struct {
	Value string
	Time  string
}
