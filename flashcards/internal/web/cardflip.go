package web

import (
	"strconv"

	"github.com/infinage/go-experiments/flashcards/internal/store"
)

// cardFlipView backs the reusable flip-card component shared by the study
// view and the new/edit form preview.
type cardFlipView struct {
	// Key disambiguates the element id so morphs replace the card
	// wholesale (no inherited flip state).
	Key string
	// Signal controls the flip; empty renders a static card.
	Signal string
	// Interactive adds click-to-flip behaviour.
	Interactive bool
	// Size is the height classes ("h-80 md:h-[420px]" etc.).
	Size     string
	Category string
	IsNew    bool
	// Front/Back are the static texts; when Live they are bound to the
	// $front/$back signals instead.
	Front, Back string
	Live        bool
	// ImageURL/AudioURL are static media sources (study view); when
	// LiveMedia the sources bind to $imgUrl/$audUrl instead.
	ImageURL, AudioURL string
	LiveMedia          bool
	TapHint            bool
}

// studyCardFlip builds the study-view card.
func studyCardFlip(sc *store.StudyCard) cardFlipView {
	v := cardFlipView{
		Key:         strconv.FormatInt(sc.ID, 10),
		Signal:      "revealed",
		Interactive: true,
		Size:        "h-80 md:h-[420px]",
		Category:    sc.Category,
		IsNew:       sc.New,
		Front:       sc.Front,
		Back:        sc.Back,
		TapHint:     true,
	}
	if sc.HasImage {
		v.ImageURL = mediaPath(sc.ID, "image")
	}
	if sc.HasAudio {
		v.AudioURL = mediaPath(sc.ID, "audio")
	}
	return v
}

// previewCardFlip builds the miniature live-preview card for the form.
func previewCardFlip(front, back string, card store.Card) cardFlipView {
	return cardFlipView{
		Key:         "preview",
		Signal:      "prevRevealed",
		Interactive: true,
		Size:        "h-64",
		Category:    card.Category,
		Front:       front,
		Back:        back,
		Live:        true,
		LiveMedia:   true,
		TapHint:     true,
	}
}
