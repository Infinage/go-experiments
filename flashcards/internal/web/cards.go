package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/starfederation/datastar-go/datastar"

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
	// Form values echoed back after a failed validation.
	Front, Back, Category string
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
	s.render(w, "EditCard", cardFormData{Card: c, Categories: names,
		Front: c.Front, Back: c.Back, Category: c.Category,
		MediaKind: mediaKindOf(c),
		Preview:   previewCardFlip(c.Front, c.Back, c)})
}

func (s *Server) saveCard(w http.ResponseWriter, r *http.Request) {
	// multipart parsing happens lazily inside the media readers below.
	front := strings.TrimSpace(r.FormValue("front"))
	back := strings.TrimSpace(r.FormValue("back"))
	category := strings.TrimSpace(r.FormValue("category"))

	d := cardFormData{Front: front, Back: back, Category: category,
		Preview: previewCardFlip(front, back, store.Card{})}
	if front == "" || back == "" {
		d.Error = "Front and back text are required."
		w.WriteHeader(http.StatusUnprocessableEntity)
		s.render(w, "NewCard", d)
		return
	}

	var id int64
	if idStr := r.FormValue("id"); idStr != "" {
		var err error
		id, err = strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}
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

	if err := s.saveMedia(r, id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/cards", http.StatusSeeOther)
}

// saveMedia stores the card-front media, enforcing image-XOR-audio: a
// card may carry an image or audio, never both. The form's media_kind
// field drives the cleanup of the unselected type.
func (s *Server) saveMedia(r *http.Request, id int64) error {
	image, err := readUpload(r, "front_image", "image")
	if err != nil {
		return err
	}
	if image.Bytes == nil {
		// Camera capture input (mobile "Take photo").
		image, err = readUpload(r, "front_image_camera", "image")
		if err != nil {
			return err
		}
	}
	audio, err := readUpload(r, "front_audio", "audio")
	if err != nil {
		return err
	}

	clearErr := func(err error) error {
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		return err
	}
	kind := r.FormValue("media_kind")
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
	case kind == "image":
		return clearErr(s.store.ClearAudio(id))
	case kind == "audio":
		return clearErr(s.store.ClearImage(id))
	default:
		// No media selected: clear whatever was there.
		if err := clearErr(s.store.ClearImage(id)); err != nil {
			return err
		}
		return clearErr(s.store.ClearAudio(id))
	}
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
