package web

import (
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
	Status    string  `json:"status"`
	DeleteIDs []int64 `json:"deleteIds"`
}

// cardsData carries everything the cards list view needs.
type cardsData struct {
	Cards      []store.Card
	Query      string
	Category   string
	Media      string
	Status     string
	Categories []store.Category
	// NoneCount backs the *None* pseudo-row: cards without a category.
	NoneCount int
}

func (s *Server) cardsPage(w http.ResponseWriter, r *http.Request) {
	var sig cardsSignals
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	d, err := s.cardsData(sig.Query, sig.Category, sig.Media,
		status(sig.Status))
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
func (s *Server) cardsData(q, category, media,
	status string) (*cardsData, error) {
	cards, err := s.store.Cards(q, category, media, status)
	if err != nil {
		return nil, err
	}
	cats, err := s.store.Categories()
	if err != nil {
		return nil, err
	}
	none, err := s.store.UntaggedCount()
	if err != nil {
		return nil, err
	}
	return &cardsData{Cards: cards, Query: q, Category: cat(cats, category),
		Media: media, Status: status, Categories: cats,
		NoneCount: none}, nil
}

// status whitelists the review-state filter values a stray signal can set,
// mirroring the store's cardStatusExpr labels.
func status(v string) string {
	switch v {
	case "new", "learning", "mastered":
		return v
	default:
		return ""
	}
}

// cat passes the filter through only when it names a known category, so a
// stale value doesn't yield a silently empty list.
func cat(cats []store.Category, category string) string {
	if category == store.NoneFilter {
		return category
	}
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

// saveSignals holds the card-form save, submitted via datastar GET
// (signals on the query string — the Android webview asset bridge rejects
// request bodies). Media bytes never ride the query: picks are staged
// via POST /cards/media/stage and referenced here by short URL
// (/cards/media/tmp/<token>); the instant-preview base64 signals
// (imgPrev/audPrev) are filtered out client-side.
type saveSignals struct {
	ID        any    `json:"id"` // bind on a hidden input yields a string
	Front     string `json:"front"`
	Back      string `json:"back"`
	Category  string `json:"category"`
	MediaKind string `json:"mediaKind"`
	// ImgPick/AudPick carry the name of the file the user picked via
	// upload / camera / recorder. They act as opt-ins: a staged URL is
	// only attached when its pick string is set, so switching the media
	// kind or clearing a pick reliably drops the intent without having
	// to erase the staged URL client side.
	ImgPick   string `json:"imgPick"`
	AudPick   string `json:"audPick"`
	ImgStaged string `json:"imgStaged"`
	AudStaged string `json:"audStaged"`
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
	fail := func(err error, code int) {
		if isDatastarRequest(r) {
			// The fetch plugin evaluates text/javascript bodies, so
			// surface the failure as a toast instead of a silent status.
			w.Header().Set("Content-Type", "text/javascript")
			fmt.Fprintf(w, "toast(%q)", err.Error())
			return
		}
		http.Error(w, err.Error(), code)
	}
	// "None" is reserved for the untagged pseudo-category in the
	// filter dropdowns; a real tag with that name would be
	// indistinguishable from it.
	if strings.EqualFold(category, "none") {
		fail(errors.New(`the tag name "None" is reserved`),
			http.StatusUnprocessableEntity)
		return
	}
	if id != 0 {
		if err := s.store.UpdateCard(id, front, back, category); err != nil {
			fail(err, http.StatusInternalServerError)
			return
		}
	} else {
		var err error
		id, err = s.store.CreateCard(front, back, category)
		if err != nil {
			fail(err, http.StatusInternalServerError)
			return
		}
	}

	if err := s.saveMediaSignals(sig, id); err != nil {
		fail(err, http.StatusBadRequest)
		return
	}
	if isDatastarRequest(r) {
		// Datastar GET: the fetch plugin executes any text/javascript
		// response (datastar.js treats it as a scripted result), so answer
		// with a client navigation to the cards list — the plain-HTTP path
		// below keeps the classic 303 redirect.
		w.Header().Set("Content-Type", "text/javascript")
		_, _ = w.Write([]byte("location.href = '/cards'"))
		return
	}
	http.Redirect(w, r, "/cards", http.StatusSeeOther)
}

// saveMediaSignals stores the card-front media from staged picks,
// enforcing image-XOR-audio: a card may carry an image or audio, never
// both. mediaKind drives the cleanup of the unselected type.
func (s *Server) saveMediaSignals(sig saveSignals, id int64) error {
	image, err := s.adoptStaged(sig.ImgStaged, sig.ImgPick,
		"image", maxImageBytes)
	if err != nil {
		return err
	}
	audio, err := s.adoptStaged(sig.AudStaged, sig.AudPick,
		"audio", maxAudioBytes)
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
	d, err := s.cardsData(sig.Query, sig.Category, sig.Media,
		status(sig.Status))
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
