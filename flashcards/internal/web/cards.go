package web

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/starfederation/datastar-go/datastar"

	"github.com/infinage/go-experiments/flashcards/internal/srs"
	"github.com/infinage/go-experiments/flashcards/internal/store"
)

// cardsSignals mirrors the frontend signals on the cards pages.
type cardsSignals struct {
	Query     string  `json:"q"`
	Category  string  `json:"category"`
	Media     string  `json:"media"`
	DeleteIDs []int64 `json:"deleteIds"`
}

// cardsData carries everything the cards list view needs.
type cardsData struct {
	Cards      []store.Card
	Query      string
	Category   string
	Media      string
	Categories []store.Category
}

func (s *Server) cardsPage(w http.ResponseWriter, r *http.Request) {
	var sig cardsSignals
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	d, err := s.cardsData(sig.Query, sig.Category, sig.Media)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if isDatastarRequest(r) {
		// Search / category filter: patch just the list fragment.
		if err := datastar.NewSSE(w, r).
			PatchElements(s.templateHTML("cards-list", d)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	s.render(w, "Cards", d)
}

// cardsData builds the list view model.
func (s *Server) cardsData(q, category, media string) (*cardsData, error) {
	cards, err := s.store.Cards(q, category, media)
	if err != nil {
		return nil, err
	}
	cats, err := s.store.Categories()
	if err != nil {
		return nil, err
	}
	return &cardsData{Cards: cards, Query: q, Category: cat(cats, category),
		Media: media, Categories: cats}, nil
}

// cat passes the filter through only when it names a known category, so a
// stale value doesn't yield a silently empty list.
func cat(cats []store.Category, category string) string {
	for _, c := range cats {
		if c.Name == category {
			return category
		}
	}
	return ""
}

// cardFormData backs the new/edit form and its preview.
type cardFormData struct {
	Card       store.Card // zero value for "new"
	Categories []string
	Error      string
	Preview    cardFlipView
	// MediaKind is the selected front-media type: "", "image" or "audio".
	MediaKind string
	// StatusLine describes the card's review state ("due tomorrow" etc.),
	// empty for new cards.
	StatusLine string
	// Form values echoed back after a failed validation.
	Front, Back, Category string
}

// dueLine renders the review status of a card for the edit page.
func dueLine(st srs.State, exists bool) string {
	if !exists {
		return "Not started — due now"
	}
	if srs.Mastered(st) && st.DueAt.After(time.Now()) {
		days := int(time.Until(st.DueAt).Hours() / 24)
		return fmt.Sprintf("Mastered — next review in %d day%s",
			days, plural(days))
	}
	if !st.DueAt.After(time.Now()) {
		return "Due now"
	}
	days := int(time.Until(st.DueAt).Hours()/24) + 1
	if days == 1 {
		return "Due tomorrow"
	}
	return fmt.Sprintf("Due in %d day%s", days, plural(days))
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// mediaKindOf derives the toggle state from a card's stored media.
func mediaKindOf(c store.Card) string {
	switch {
	case c.HasImage:
		return "image"
	case c.HasAudio:
		return "audio"
	}
	return ""
}

// cardPageData wraps the form data with the page title.
type cardPageData struct {
	Title string
	Data  cardFormData
}

func (s *Server) newCard(w http.ResponseWriter, r *http.Request) {
	names, err := s.categoryNames()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, "NewCard", cardFormData{Categories: names,
		Preview: previewCardFlip("", "", store.Card{})})
}

func (s *Server) editCard(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r)
	if !ok {
		return
	}
	c, err := s.store.Card(id)
	switch {
	case errors.Is(err, store.ErrNotFound):
		http.NotFound(w, r)
		return
	case err != nil:
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	names, err := s.categoryNames()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	line := ""
	if st, exists, err := s.store.ReviewState(c.ID); err == nil && exists {
		line = dueLine(st, true)
	} else if err == nil {
		line = dueLine(st, false)
	}
	s.render(w, "EditCard", cardFormData{Card: c, Categories: names,
		Front: c.Front, Back: c.Back, Category: c.Category,
		MediaKind: mediaKindOf(c), StatusLine: line,
		Preview: previewCardFlip(c.Front, c.Back, c)})
}

// fileSignal is one file picked via data-bind on a file input: datastar
// reads the file as data URL and ships {name, contents (base64), mime}.
type fileSignal struct {
	Name     string `json:"name"`
	Contents string `json:"contents"` // base64, no data: prefix
	Mime     string `json:"mime"`
}

// saveSignals holds the card-form save, submitted via datastar GET
// (signals on the query string — the Android webview asset bridge rejects
// request bodies). Media arrives as base64 arrays from data-bind.
type saveSignals struct {
	ID        any          `json:"id"` // bind on a hidden input yields a string
	Front     string       `json:"front"`
	Back      string       `json:"back"`
	Category  string       `json:"category"`
	MediaKind string       `json:"mediaKind"`
	ImgData   []fileSignal `json:"imgData"`
	AudData   []fileSignal `json:"audData"`
}

func (s *Server) saveCard(w http.ResponseWriter, r *http.Request) {
	var sig saveSignals
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	front := strings.TrimSpace(sig.Front)
	back := strings.TrimSpace(sig.Back)
	category := strings.TrimSpace(sig.Category)

	d := cardFormData{Front: front, Back: back, Category: category,
		Preview: previewCardFlip(front, back, store.Card{})}
	if front == "" || back == "" {
		d.Error = "Front and back text are required."
		w.WriteHeader(http.StatusUnprocessableEntity)
		s.render(w, "NewCard", d)
		return
	}

	var id int64
	switch v := sig.ID.(type) {
	case float64:
		id = int64(v)
	case string:
		if v != "" {
			var err error
			id, err = strconv.ParseInt(v, 10, 64)
			if err != nil {
				http.Error(w, "bad id", http.StatusBadRequest)
				return
			}
		}
	}
	if id != 0 {
		if err := s.store.UpdateCard(id, front, back, category); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		var err error
		id, err = s.store.CreateCard(front, back, category)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if err := s.saveMediaSignals(sig, id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if isDatastarRequest(r) {
		// Datastar GET: respond with a redirect-style round-trip by patching
		// the cards list shell? The page navigates client-side on datastar,
		// so just confirm and let the client follow its own redirect.
		w.Header().Set("X-Flashcards-Saved", strconv.FormatInt(id, 10))
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/cards", http.StatusSeeOther)
}

// saveMediaSignals stores the card-front media from base64 signal data,
// enforcing image-XOR-audio: a card may carry an image or audio, never
// both. mediaKind drives the cleanup of the unselected type.
func (s *Server) saveMediaSignals(sig saveSignals, id int64) error {
	image, err := mediaFromSignals(sig.ImgData, "image", maxImageBytes)
	if err != nil {
		return err
	}
	audio, err := mediaFromSignals(sig.AudData, "audio", maxAudioBytes)
	if err != nil {
		return err
	}
	clearErr := func(err error) error {
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		return err
	}
	switch {
	case image.Bytes != nil:
		if err := s.store.SetImage(id, image); err != nil {
			return err
		}
		return clearErr(s.store.ClearAudio(id))
	case audio.Bytes != nil:
		if err := s.store.SetAudio(id, audio); err != nil {
			return err
		}
		return clearErr(s.store.ClearImage(id))
	case sig.MediaKind == "image":
		return clearErr(s.store.ClearAudio(id))
	case sig.MediaKind == "audio":
		return clearErr(s.store.ClearImage(id))
	default:
		if err := clearErr(s.store.ClearImage(id)); err != nil {
			return err
		}
		return clearErr(s.store.ClearAudio(id))
	}
}

// mediaFromSignals turns the picked files from a data/bind file input
// (datastar ships {name, contents base64, mime}) into store.Media with a
// size cap, sniffing the real content type.
func mediaFromSignals(files []fileSignal, kind string, limit int) (store.Media, error) {
	if len(files) == 0 || files[0].Contents == "" {
		return store.Media{}, nil
	}
	f := files[0]
	b, err := base64.StdEncoding.DecodeString(f.Contents)
	if err != nil {
		return store.Media{}, err
	}
	if len(b) > limit {
		return store.Media{}, errTooLarge(kind, limit)
	}
	m, err := store.Media{Bytes: b, Mime: sniffMime(b)}, sniffCheck(b, kind)
	return m, err
}

// sniffCheck validates the sniffed type against the accepted media sets
// (mirrors readUpload's enforcement).
func sniffCheck(b []byte, kind string) error {
	m := sniffMime(b)
	if _, ok := sniffed[m]; !ok {
		return errors.New("unsupported " + kind + " type " + m)
	}
	return nil
}

func (s *Server) deleteCards(w http.ResponseWriter, r *http.Request) {
	var sig cardsSignals
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.store.DeleteCards(sig.DeleteIDs); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	d, err := s.cardsData(sig.Query, sig.Category, sig.Media)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Reset the selection and re-render the list over SSE.
	sse := datastar.NewSSE(w, r)
	if err := sse.PatchSignals([]byte(`{"deleteIds": []}`)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := sse.PatchElements(s.templateHTML("cards-list", d)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) categoryNames() ([]string, error) {
	cats, err := s.store.Categories()
	if err != nil {
		return nil, err
	}
	names := make([]string, len(cats))
	for i, c := range cats {
		names[i] = c.Name
	}
	return names, nil
}

// pathID parses the {id} path value, writing a 400 on failure.
func (s *Server) pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		http.Error(w, "bad id", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

func isDatastarRequest(r *http.Request) bool {
	return r.Header.Get("Datastar-Request") == "true"
}
