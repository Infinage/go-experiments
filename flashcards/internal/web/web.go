// Package web holds HTTP handlers and templates.
package web

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/infinage/go-experiments/flashcards/internal/store"
)

type Server struct {
	store *store.Store
	tmpl  *template.Template
	stage *staging
}

// New builds the HTTP handler. assets must contain html/, css/ and js/.
func New(db *store.Store, assets fs.FS) (http.Handler, error) {
	funcs := template.FuncMap{
		// sub/add are integer arithmetic for templates.
		"sub": func(a, b int) int { return a - b },
		"add": func(a, b int) int { return a + b },
		// cardStatusData wraps the status-line fields for the fragment.
		"cardStatusData": func(line string, id int64) cardStatusData {
			return cardStatusData{Line: line, ID: id}
		},
		// gradeChoices lists the study grade buttons with their hints.
		"gradeChoices": func() []gradeChoice {
			return []gradeChoice{
				{"forgot", "Forgot", "Didn't recall — one retry at the end of this pass"},
				{"hard", "Hard", "Recalled with effort — due tomorrow"},
				{"medium", "Medium", "Recalled — due in a few days"},
				{"easy", "Easy", "Recalled easily — mastered sooner"},
			}
		},
		// percent guards against division by zero in progress bars.
		"percent": func(part, total int) int {
			if total == 0 {
				return 0
			}
			return part * 100 / total
		},
		// Used by the new/edit page wrappers to pass the title down.
		"cardPageData": func(title string, d cardFormData) cardPageData {
			return cardPageData{Title: title, Data: d}
		},
		// JSON-quotes a string for embedding in a data-signals expression.
		"toSignal": func(s string) string {
			b, _ := json.Marshal(s)
			return string(b)
		},
		// mediaType describes a card's front media for the grid label.
		"mediaType": func(c store.Card) string {
			switch {
			case c.HasImage:
				return "IMAGE"
			case c.HasAudio:
				return "AUDIO"
			default:
				return "TEXT"
			}
		},
		// statusColor colors a card's header strip by review progress.
		// Plain hex values: html/template's CSS filter rejects var().
		"statusColor": func(status string) string {
			switch status {
			case "mastered":
				return "#4FDDC7"
			case "learning":
				return "#94A9F0"
			default:
				return "#FF8AE2"
			}
		},
		"statusLabel": func(status string) string {
			switch status {
			case "mastered":
				return "Mastered"
			case "learning":
				return "In progress"
			default:
				return "Not started"
			}
		},
		// mediaInit JSON-quotes the cards page's initial media filter.
		"mediaInit": func(d cardsData) string {
			b, _ := json.Marshal(d.Media)
			return string(b)
		},
		// existingMediaURL is the media endpoint for an attached file, or
		// an empty JSON string for new cards.
		"existingMediaURL": func(c store.Card, kind string) string {
			var u string
			if c.ID != 0 {
				switch kind {
				case "image":
					if c.HasImage {
						u = mediaPath(c.ID, "image")
					}
				case "audio":
					if c.HasAudio {
						u = mediaPath(c.ID, "audio")
					}
				}
			}
			b, _ := json.Marshal(u)
			return string(b)
		},
	}
	tmpl, err := template.New("").Funcs(funcs).ParseFS(assets, "html/*.html")
	if err != nil {
		return nil, err
	}
	s := &Server{store: db, tmpl: tmpl, stage: newStaging()}
	s.stage.init()

	mux := http.NewServeMux()
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServerFS(assets)))
	// The Wails asset server requests "/index.html" for the webview URL "/" —
	// the {$}-anchored studyPage route only matches the bare "/" path.
	mux.HandleFunc("GET /index.html", s.studyPage)
	mux.HandleFunc("GET /{$}", s.studyPage)
	// Everything runs over GET: the Android webview asset bridge rejects
	// request bodies, so one code path serves desktop and mobile alike —
	// datastar signals ride the query string.
	mux.HandleFunc("GET /study/{id}/grade", s.gradeCard)
	mux.HandleFunc("GET /study/reset", s.resetProgress)
	mux.HandleFunc("GET /seed", s.seedCards)
	mux.HandleFunc("GET /cards", s.cardsPage)
	mux.HandleFunc("GET /cards/new", s.newCard)
	mux.HandleFunc("GET /cards/save", s.saveCard)
	mux.HandleFunc("GET /cards/{id}/edit", s.editCard)
	mux.HandleFunc("POST /cards/media/stage", s.stageMedia)
	mux.HandleFunc("GET /cards/media/tmp/{token}", s.serveStaged)
	mux.HandleFunc("GET /cards/{id}/media/{kind}", s.cardMedia)
	mux.HandleFunc("GET /cards/{id}/media/delete", s.deleteMedia)
	mux.HandleFunc("GET /cards/{id}/status/reset", s.resetCardStatus)
	mux.HandleFunc("GET /cards/delete", s.deleteCards)
	// GET now carries mutations, so nothing dynamic may be served from any
	// intermediary or browser cache. Static assets keep their own headers.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/assets/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		mux.ServeHTTP(w, r)
	}), nil
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("render %s: %v", name, err)
	}
}

// templateHTML executes a named template to a string, for Datastar
// fragments. On error it logs and returns an empty string (the client
// keeps the current DOM, which beats a half-rendered fragment).
func (s *Server) templateHTML(name string, data any) string {
	var buf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		log.Printf("render %s fragment: %v", name, err)
		return ""
	}
	return buf.String()
}
