package flashcards

import (
	"bytes"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/starfederation/datastar-go/datastar"
)

const maxBodySize = 8 << 20

type session struct {
	mu      sync.Mutex
	user    string
	expires time.Time
	review  *review
}

// Server wires the Store to HTTP handlers.
type Server struct {
	store    *Store
	tmpl     *template.Template
	static   fs.FS
	mu       sync.Mutex
	sessions map[string]*session
}

// NewServer parses the embedded templates and returns a ready Server.
// staticFS is rooted at assets/ and served under /assets/.
func NewServer(st *Store, tmplFS fs.FS, staticFS fs.FS) (*Server, error) {
	tmpl, err := template.ParseFS(tmplFS, "*.html")
	if err != nil {
		return nil, err
	}
	return &Server{store: st, tmpl: tmpl, static: staticFS,
		sessions: map[string]*session{}}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("POST /login", s.handleLogin)
	mux.HandleFunc("POST /signup", s.handleSignup)
	mux.HandleFunc("POST /logout", s.authed(s.handleLogout))
	mux.HandleFunc("GET /home", s.authed(s.handleHome))
	mux.HandleFunc("GET /cards", s.authed(s.handleCardsList))
	mux.HandleFunc("GET /cards/new", s.authed(s.handleNewForm))
	mux.HandleFunc("POST /cards", s.authed(s.handleCreateCard))
	mux.HandleFunc("GET /cards/import", s.authed(s.handleImportForm))
	mux.HandleFunc("POST /cards/import", s.authed(s.handleImport))
	mux.HandleFunc("GET /cards/{id}/edit", s.authed(s.handleEditForm))
	mux.HandleFunc("POST /cards/{id}/edit", s.authed(s.handleEditCard))
	mux.HandleFunc("DELETE /cards/{id}", s.authed(s.handleDeleteCard))
	mux.HandleFunc("DELETE /cards", s.authed(s.handleDeleteCards))
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(s.static))))
	mux.HandleFunc("POST /review/start", s.authed(s.handleReviewStart))
	mux.HandleFunc("POST /review/reveal", s.authed(s.handleReveal))
	mux.HandleFunc("POST /review/answer", s.authed(s.handleAnswer))
	return s.csrf(mux)
}

// ---------- middleware ----------

// csrf requires the Datastar-Request header on anything that isn't a plain GET.
// Cross-site forms can't set custom headers, so this plus SameSite=Lax is enough here.
func (s *Server) csrf(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead &&
			r.Header.Get("Datastar-Request") != "true" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ---------- SSE + templates ----------

// patchMain renders a template and sends it as a Datastar element patch via
// the go-datastar SDK. Every fragment's root element is <section id="main">,
// so Datastar morphs it in place.
func (s *Server) patchMain(w http.ResponseWriter, r *http.Request, name string, data any) {
	var b bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&b, name, data); err != nil {
		log.Printf("render %s: %v", name, err)
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
	sse := datastar.NewSSE(w, r)
	sse.PatchElements(b.String(), datastar.WithSelector("#main"))
}

// readSignals decodes the request body (which Datastar sends as JSON signals)
// into dst, capped at maxBodySize. Delegates to the SDK's reader.
func readSignals(r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, maxBodySize)
	return datastar.ReadSignals(r, dst)
}

func fail(w http.ResponseWriter, err error) {
	log.Printf("error: %v", err)
	http.Error(w, "something went wrong", http.StatusInternalServerError)
}
