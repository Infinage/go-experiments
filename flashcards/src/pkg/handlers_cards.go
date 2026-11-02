package flashcards

import (
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// parseTags splits a raw comma- or semicolon-separated tag string into unique,
// lowercased tags. Form input uses ","; CSV import uses ";".
func parseTags(raw, sep string) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range strings.Split(raw, sep) {
		t = strings.ToLower(strings.TrimSpace(t))
		if t != "" && !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

// handleNewForm is called when the user clicks "Add card" on the home page
// (GET /cards/new). It patches the card creation form into view.
func (s *Server) handleNewForm(w http.ResponseWriter, r *http.Request, _ *session) {
	s.patchMain(w, r, "cardform", map[string]any{})
}

// handleCreateCard is called when the user submits the add-card form
// (POST /cards). It validates input, creates the card, and either returns to
// the form (with ?more=1 to allow another) or the home page.
func (s *Server) handleCreateCard(w http.ResponseWriter, r *http.Request, sess *session) {
	var sg struct{ Prompt, Answer, Tags string }
	if err := readSignals(r, &sg); err != nil {
		s.patchMain(w, r, "cardform", map[string]any{"Flash": "Bad request."})
		return
	}
	if strings.TrimSpace(sg.Prompt) == "" || strings.TrimSpace(sg.Answer) == "" {
		s.patchMain(w, r, "cardform", map[string]any{"Flash": "Prompt and answer are required."})
		return
	}
	card := Card{
		Prompt: strings.TrimSpace(sg.Prompt), Answer: strings.TrimSpace(sg.Answer),
		Tags: parseTags(sg.Tags, ","), Username: sess.user,
	}
	if err := s.store.CreateCard(r.Context(), &card); err != nil {
		fail(w, err)
		return
	}
	if r.URL.Query().Get("more") == "1" {
		s.patchMain(w, r, "cardform", map[string]any{"Flash": "Saved. Add another:"})
		return
	}
	s.renderHome(w, r, sess.user, "Card saved.")
}

// handleImportForm is called when the user clicks "Import CSV" on the home
// page (GET /cards/import). It patches the CSV import form into view.
func (s *Server) handleImportForm(w http.ResponseWriter, r *http.Request, _ *session) {
	s.patchMain(w, r, "import", map[string]any{})
}

func parseCSV(rd io.Reader, user string) ([]Card, []string) {
	cr := csv.NewReader(rd)
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true

	header, err := cr.Read()
	if err != nil {
		return nil, []string{"Couldn't read the header row."}
	}

	col := map[string]int{}
	for i, h := range header {
		// \ufeff is the UTF-8 BOM that many editors prepend to files saved as
		// "UTF-8 with BOM"; it must be stripped from the first header cell.
		h = strings.TrimPrefix(h, "\ufeff")
		h = strings.TrimSpace(h)
		h = strings.ToLower(h)
		col[h] = i
	}
	pi, ok1 := col["prompt"]
	ai, ok2 := col["answer"]
	if !ok1 || !ok2 {
		return nil, []string{"Header must include 'prompt' and 'answer' columns."}
	}
	ti, hasTags := col["tags"]

	var cards []Card
	var errs []string
	for row := 2; ; row++ {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			errs = append(errs, fmt.Sprintf("Row %d: %v", row, err))
			break // fail fast: show the first error and stop
		}
		get := func(i int) string {
			if i < len(rec) {
				return strings.TrimSpace(rec[i])
			}
			return ""
		}
		p, a := get(pi), get(ai)
		if p == "" || a == "" {
			errs = append(errs, fmt.Sprintf("Row %d: prompt and answer are required.", row))
			break // fail fast: show the first error and stop
		}
		c := Card{Prompt: p, Answer: a, Username: user}
		if hasTags {
			c.Tags = parseTags(get(ti), ";")
		}
		cards = append(cards, c)
	}
	return cards, errs
}

// handleImport is called when the user submits a CSV file (POST /cards/import).
// It decodes the base64 payload, parses the CSV, and either reports row-level
// errors back to the import form or creates the cards and returns home.
func (s *Server) handleImport(w http.ResponseWriter, r *http.Request, sess *session) {
	var sg struct{ CSV []string }
	if err := readSignals(r, &sg); err != nil || len(sg.CSV) == 0 || sg.CSV[0] == "" {
		s.patchMain(w, r, "import", map[string]any{"Errs": []string{"Choose a CSV file first."}})
		return
	}
	raw := sg.CSV[0]
	if strings.HasPrefix(raw, "data:") { // defensive: strip data-URL prefix if present
		if i := strings.Index(raw, ","); i >= 0 {
			raw = raw[i+1:]
		}
	}
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		s.patchMain(w, r, "import", map[string]any{"Errs": []string{"Couldn't decode the uploaded file."}})
		return
	}
	cards, errs := parseCSV(bytes.NewReader(data), sess.user)
	if len(errs) > 0 {
		s.patchMain(w, r, "import", map[string]any{"Msg": "Nothing imported. Fix these and retry:", "Errs": errs})
		return
	}
	if len(cards) == 0 {
		s.patchMain(w, r, "import", map[string]any{"Errs": []string{"The file has no data rows."}})
		return
	}
	for i := range cards {
		if err := s.store.CreateCard(r.Context(), &cards[i]); err != nil {
			fail(w, err)
			return
		}
	}
	s.renderHome(w, r, sess.user, fmt.Sprintf("Imported %d cards.", len(cards)))
}
