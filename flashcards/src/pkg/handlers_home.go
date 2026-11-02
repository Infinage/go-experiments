package flashcards

import (
	"bytes"
	"html/template"
	"net/http"
	"net/url"
)

// handleIndex is called when a user hits the site root (GET /). It decides
// between rendering the login page and the home dashboard based on the
// session cookie.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	var body bytes.Buffer
	if sess := s.lookup(r); sess == nil {
		_ = s.tmpl.ExecuteTemplate(&body, "login", map[string]any{})
	} else {
		hd, err := s.buildHome(r, sess.user, "")
		if err != nil {
			fail(w, err)
			return
		}
		_ = s.tmpl.ExecuteTemplate(&body, "home", hd)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.tmpl.ExecuteTemplate(w, "page", template.HTML(body.String()))
}

func (s *Server) buildHome(r *http.Request, user, flash string) (map[string]any, error) {
	tags, err := s.store.TagCounts(r.Context(), user)
	if err != nil {
		return nil, err
	}
	views := make([]map[string]any, 0, len(tags))
	for _, t := range tags {
		views = append(views, map[string]any{
			"Name": t.Name, 
			"Due": t.Due, 
			"Total": t.Total, 
			"Q": url.QueryEscape(t.Name),
		})
	}

	return map[string]any{"User": user, "Flash": flash, "Tags": views}, nil
}

func (s *Server) renderHome(w http.ResponseWriter, r *http.Request, user, flash string) {
	hd, err := s.buildHome(r, user, flash)
	if err != nil {
		fail(w, err)
		return
	}
	s.patchMain(w, r, "home", hd)
}

// handleHome is called when the user navigates to GET /home. It re-renders
// the home dashboard with the user's tag summary.
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request, sess *session) {
	s.renderHome(w, r, sess.user, "")
}
