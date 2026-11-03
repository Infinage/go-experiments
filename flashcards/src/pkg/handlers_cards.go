package flashcards

import (
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// cardForm mirrors the prompt/answer/tags signals of the card create/edit
// forms. Tags is the raw input string (comma-separated), not yet parsed.
// The same struct reads incoming signals and seeds data-signals for edit
// forms, so the signal names are defined in exactly one place.
type cardForm struct {
	Prompt string `json:"prompt"`
	Answer string `json:"answer"`
	Tags   string `json:"tags"`
}

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
	var sg cardForm
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

// cardView is the per-row view data for the cards list. Tags is a joined,
// display-ready string.
type cardView struct {
	ID     int64
	Prompt string
	Tags   string
}

// buildCardsViews fetches the user's cards and flattens them into the
// display-ready rows the cardslist template renders.
func (s *Server) buildCardsViews(r *http.Request, user string) ([]cardView, error) {
	cards, err := s.store.GetCardsByUser(r.Context(), user)
	if err != nil {
		return nil, err
	}
	views := make([]cardView, 0, len(cards))
	for _, c := range cards {
		views = append(views, cardView{ID: c.ID, Prompt: c.Prompt,
			Tags: strings.Join(c.Tags, ", ")})
	}
	return views, nil
}

// handleCardsList is called when the user clicks "Your cards" on the home
// page (GET /cards). It patches the card list into view.
func (s *Server) handleCardsList(w http.ResponseWriter, r *http.Request, sess *session) {
	views, err := s.buildCardsViews(r, sess.user)
	if err != nil {
		fail(w, err)
		return
	}
	s.patchMain(w, r, "cardslist", map[string]any{"Cards": views})
}

// renderCardsList re-renders the cards list with a flash message.
func (s *Server) renderCardsList(w http.ResponseWriter, r *http.Request, user, flash string) {
	views, err := s.buildCardsViews(r, user)
	if err != nil {
		fail(w, err)
		return
	}
	s.patchMain(w, r, "cardslist", map[string]any{"Cards": views, "Flash": flash})
}

// ownedCard looks up id among the user's cards, returning the card and a
// boolean for whether it exists and belongs to the caller.
func (s *Server) ownedCard(r *http.Request, user string, id int64) (Card, bool, error) {
	cards, err := s.store.GetCardsByUser(r.Context(), user)
	if err != nil {
		return Card{}, false, err
	}
	for _, c := range cards {
		if c.ID == id {
			return c, true, nil
		}
	}
	return Card{}, false, nil
}

// signalsJSON encodes a cardForm as a data-signals JSON object so Datastar
// seeds the bound signals with the card's current content.
func signalsJSON(f cardForm) string {
	b, _ := json.Marshal(f)
	return string(b)
}

// handleEditForm is called when the user clicks "Edit" on a card row
// (GET /cards/{id}/edit). It patches the edit form prefilled with the card.
func (s *Server) handleEditForm(w http.ResponseWriter, r *http.Request, sess *session) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.renderCardsList(w, r, sess.user, "That card doesn't exist.")
		return
	}
	card, ok, err := s.ownedCard(r, sess.user, id)
	if err != nil {
		fail(w, err)
		return
	}
	if !ok {
		s.renderCardsList(w, r, sess.user, "That card doesn't exist.")
		return
	}
	s.patchMain(w, r, "cardedit", map[string]any{
		"ID": card.ID, "Prompt": card.Prompt, "Answer": card.Answer,
		"Tags":    strings.Join(card.Tags, ", "),
		"Signals": signalsJSON(cardForm{card.Prompt, card.Answer, strings.Join(card.Tags, ", ")}),
	})
}

// handleEditCard is called when the user saves the edit form
// (POST /cards/{id}/edit). It validates input and updates the card.
func (s *Server) handleEditCard(w http.ResponseWriter, r *http.Request, sess *session) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.renderCardsList(w, r, sess.user, "That card doesn't exist.")
		return
	}
	card, ok, err := s.ownedCard(r, sess.user, id)
	if err != nil {
		fail(w, err)
		return
	}
	if !ok {
		s.renderCardsList(w, r, sess.user, "That card doesn't exist.")
		return
	}
	var sg cardForm
	if err := readSignals(r, &sg); err != nil {
		s.patchMain(w, r, "cardedit", map[string]any{
			"ID": card.ID, "Prompt": card.Prompt, "Answer": card.Answer,
			"Tags": strings.Join(card.Tags, ", "), "Flash": "Bad request.",
			"Signals": signalsJSON(cardForm{card.Prompt, card.Answer, strings.Join(card.Tags, ", ")}),
		})
		return
	}
	if strings.TrimSpace(sg.Prompt) == "" || strings.TrimSpace(sg.Answer) == "" {
		s.patchMain(w, r, "cardedit", map[string]any{
			"ID": card.ID, "Prompt": sg.Prompt, "Answer": sg.Answer, "Tags": sg.Tags,
			"Flash":   "Prompt and answer are required.",
			"Signals": signalsJSON(sg),
		})
		return
	}
	card.Prompt = strings.TrimSpace(sg.Prompt)
	card.Answer = strings.TrimSpace(sg.Answer)
	card.Tags = parseTags(sg.Tags, ",")
	if err := s.store.UpdateCard(r.Context(), card); err != nil {
		fail(w, err)
		return
	}
	s.renderCardsList(w, r, sess.user, "Card updated.")
}

// parseBulkIDs extracts the selected card ids from bulk-selection signals.
// Checkboxes bind signals named pick_<id> to booleans; only truthy ones count.
func parseBulkIDs(sg map[string]any) []int64 {
	var ids []int64
	for k, v := range sg {
		if !strings.HasPrefix(k, "pick_") {
			continue
		}
		on, _ := v.(bool)
		if !on {
			continue
		}
		id, err := strconv.ParseInt(strings.TrimPrefix(k, "pick_"), 10, 64)
		if err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

// ownedIDs filters ids down to those that belong to the user, so one user can
// never delete another user's cards via crafted signals.
func ownedIDs(cards []Card, ids []int64) []int64 {
	own := make(map[int64]bool, len(cards))
	for _, c := range cards {
		own[c.ID] = true
	}
	var out []int64
	for _, id := range ids {
		if own[id] {
			out = append(out, id)
		}
	}
	return out
}

// handleDeleteCard is called when the user confirms deleting one card
// (DELETE /cards/{id}).
func (s *Server) handleDeleteCard(w http.ResponseWriter, r *http.Request, sess *session) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.renderCardsList(w, r, sess.user, "That card doesn't exist.")
		return
	}
	_, ok, err := s.ownedCard(r, sess.user, id)
	if err != nil {
		fail(w, err)
		return
	}
	if !ok {
		s.renderCardsList(w, r, sess.user, "That card doesn't exist.")
		return
	}
	if err := s.store.DeleteCards(r.Context(), []int64{id}); err != nil {
		fail(w, err)
		return
	}
	s.renderCardsList(w, r, sess.user, "Deleted card.")
}

// handleDeleteCards is called when the user bulk-deletes selected cards
// (DELETE /cards). The Datastar client sends DELETE signals as a querystring
// (?datastar=…), which ReadSignals reads natively.
func (s *Server) handleDeleteCards(w http.ResponseWriter, r *http.Request, sess *session) {
	var sg map[string]any
	if err := readSignals(r, &sg); err != nil {
		s.renderCardsList(w, r, sess.user, "Bad request.")
		return
	}
	ids := parseBulkIDs(sg)
	if len(ids) == 0 {
		s.renderCardsList(w, r, sess.user, "Select at least one card to delete.")
		return
	}
	cards, err := s.store.GetCardsByUser(r.Context(), sess.user)
	if err != nil {
		fail(w, err)
		return
	}
	ids = ownedIDs(cards, ids)
	if err := s.store.DeleteCards(r.Context(), ids); err != nil {
		fail(w, err)
		return
	}
	s.renderCardsList(w, r, sess.user, fmt.Sprintf("Deleted %d cards.", len(ids)))
}
