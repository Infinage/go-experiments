// Package web holds HTTP handlers and templates.
package web

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io/fs"
	"log"
	"net/http"

	"github.com/infinage/go-experiments/flashcards/internal/store"
)

type Server struct {
	store *store.Store
	tmpl  *template.Template
}

// New builds the HTTP handler. assets must contain html/, css/ and js/.
func New(db *store.Store, assets fs.FS) (http.Handler, error) {
	funcs := template.FuncMap{
		// sub is integer subtraction for template arithmetic.
		"sub": func(a, b int) int { return a - b },
		// gradeChoices lists the study grade buttons.
		"gradeChoices": func() []gradeChoice {
			return []gradeChoice{
				{"again", "Again"}, {"hard", "Hard"},
				{"good", "Good"}, {"easy", "Easy"},
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
		// mediaType/mediaColor describe a card's front media for the
		// cards-grid header strip.
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
		"mediaColor": func(c store.Card) string {
			switch {
			case c.HasImage:
				return "var(--color-mint)"
			case c.HasAudio:
				return "var(--color-sky)"
			default:
				return "var(--color-pink)"
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
	s := &Server{store: db, tmpl: tmpl}

	mux := http.NewServeMux()
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServerFS(assets)))
	mux.HandleFunc("GET /{$}", s.studyPage)
	mux.HandleFunc("POST /study/{id}/grade", s.gradeCard)
	mux.HandleFunc("POST /study/reset", s.resetProgress)
	mux.HandleFunc("POST /seed", s.seedCards)
	mux.HandleFunc("GET /cards", s.cardsPage)
	mux.HandleFunc("GET /cards/new", s.newCard)
	mux.HandleFunc("POST /cards/save", s.saveCard)
	mux.HandleFunc("GET /cards/{id}/edit", s.editCard)
	mux.HandleFunc("GET /cards/{id}/media/{kind}", s.cardMedia)
	mux.HandleFunc("POST /cards/{id}/media/delete", s.deleteMedia)
	mux.HandleFunc("DELETE /cards", s.deleteCards)
	return mux, nil
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("render %s: %v", name, err)
	}
}

// templateHTML executes a named template to a string, for Datastar
// fragments. On error it returns an empty string; render() logs errors for
// full-page renders, and here the error is swallowed the same way.
func (s *Server) templateHTML(name string, data any) string {
	var buf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return ""
	}
	return buf.String()
}
