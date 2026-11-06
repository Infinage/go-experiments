package web

import (
	"encoding/binary"
	"hash/fnv"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/starfederation/datastar-go/datastar"

	"github.com/infinage/go-experiments/flashcards/internal/srs"
	"github.com/infinage/go-experiments/flashcards/internal/store"
)

// studySignals mirrors the frontend signals on the study pages.
type studySignals struct {
	HideMastered bool   `json:"hideMastered"`
	Category     string `json:"category"`
	Grade        string `json:"grade"`
	// Idx is the position within the queue (0-based) for Prev/Next.
	Idx int `json:"idx"`
	// ShuffleSeed randomises the queue order; stable while it doesn't
	// change, so paging doesn't reshuffle.
	ShuffleSeed int64 `json:"shuffleSeed"`
	// Force queues everything due-or-not ("review anyway").
	Force bool `json:"force"`
}

// studyData backs the whole study page and its card fragment.
type studyData struct {
	Card         *store.StudyCard // nil when nothing is due
	Stats        store.StudyStats
	HideMastered bool
	Category     string
	Categories   []store.Category
	Decks        []store.CategoryProgress
	Force        bool
	// Idx/Count locate the current card in the queue.
	Idx, Count int
	// Position is "Card N of M" within the current queue (1-based).
	Position string
}

func (s *Server) studyPage(w http.ResponseWriter, r *http.Request) {
	var sig studySignals
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	d, err := s.studyData(sig)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if isDatastarRequest(r) {
		// Toggle or refresh: patch just the study area.
		if err := datastar.NewSSE(w, r).
			PatchElements(s.templateHTML("study-area", d)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	s.render(w, "Study", d)
}

// studyData builds the study view model from the request signals.
func (s *Server) studyData(sig studySignals) (*studyData, error) {
	q, err := s.queue(sig.HideMastered, sig.Category, sig.Force)
	if err != nil {
		return nil, err
	}
	if sig.ShuffleSeed > 0 {
		shuffle(q, sig.ShuffleSeed)
	}
	// Clamp the position into the (possibly shorter) queue.
	idx := sig.Idx
	if idx < 0 {
		idx = 0
	}
	if idx >= len(q) {
		idx = len(q) - 1
	}
	if idx < 0 {
		idx = 0
	}
	stats, err := s.store.Stats()
	if err != nil {
		return nil, err
	}
	cats, err := s.store.Categories()
	if err != nil {
		return nil, err
	}
	decks, err := s.store.CategoryProgresses()
	if err != nil {
		return nil, err
	}
	d := &studyData{Stats: stats, HideMastered: sig.HideMastered,
		Category: categoryIn(cats, sig.Category), Categories: cats,
		Idx: idx, Count: len(q), Decks: decks, Force: sig.Force}
	if idx < len(q) {
		d.Card = &q[idx]
	}
	if d.Count > 0 {
		d.Position = "Card " + strconv.Itoa(idx+1) + " of " +
			strconv.Itoa(d.Count)
	}
	return d, nil
}

// shuffle orders the queue deterministically from a seed, so paging
// through a shuffled deck stays stable until the seed changes.
func shuffle(q []store.StudyCard, seed int64) {
	keys := make(map[int64]float64, len(q))
	for _, c := range q {
		h := fnv.New64a()
		binary.Write(h, binary.LittleEndian, seed)
		binary.Write(h, binary.LittleEndian, c.ID)
		keys[c.ID] = float64(h.Sum64())
	}
	sort.Slice(q, func(i, j int) bool {
		return keys[q[i].ID] < keys[q[j].ID]
	})
}

// queue fetches the due queue, filtered by category when given.
func (s *Server) queue(hideMastered bool, category string, force bool) ([]store.StudyCard, error) {
	q, err := s.store.Queue(hideMastered, force)
	if err != nil {
		return nil, err
	}
	if category == "" {
		return q, nil
	}
	var out []store.StudyCard
	for _, c := range q {
		if c.Category == category {
			out = append(out, c)
		}
	}
	return out, nil
}

// categoryIn passes the filter through only when it names a known
// category, so a stale value doesn't silently filter everything out.
func categoryIn(cats []store.Category, category string) string {
	for _, c := range cats {
		if c.Name == category {
			return category
		}
	}
	return ""
}

// gradeCard applies a review grade and patches the next card over SSE.
func (s *Server) gradeCard(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r)
	if !ok {
		return
	}
	var sig studySignals
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// The grade arrives merged into the signals by the frontend action.
	grade, err := srs.ParseGrade(sig.Grade)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := s.store.Review(id, grade, time.Now()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// The graded card leaves the queue; keep the index so the next card
	// slides in, and reset the flip + paging signals.
	d, err := s.studyData(sig)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sse := datastar.NewSSE(w, r)
	if err := sse.PatchSignals([]byte(
		`{"revealed": false, "idx": ` + strconv.Itoa(d.Idx) + `}`)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := sse.PatchElements(s.templateHTML("study-area", d)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// resetProgress clears scheduling state (optionally per category), after
// the frontend double-click confirmation.
func (s *Server) resetProgress(w http.ResponseWriter, r *http.Request) {
	var sig studySignals
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.store.ResetProgress(sig.Category); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	d, err := s.studyData(sig)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sse := datastar.NewSSE(w, r)
	if err := sse.PatchSignals([]byte(
		`{"revealed": false, "idx": 0, "confirm": false, "force": false}`)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := sse.PatchElements(s.templateHTML("study-area", d)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// seedCards inserts a few demo cards when the database is empty, so a
// fresh install has something to study.
func (s *Server) seedCards(w http.ResponseWriter, r *http.Request) {
	st, err := s.store.Stats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if st.Total != 0 {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	seeds := [][3]string{
		{"What does HTML stand for?", "HyperText Markup Language", "web"},
		{"What does CSS stand for?", "Cascading Style Sheets", "web"},
		{"HTTP status 404 means…", "Not Found", "web"},
		{"Go keyword to launch a goroutine", "go", "langs"},
		{"Which SQL clause filters rows?", "WHERE", "db"},
	}
	for _, c := range seeds {
		if _, err := s.store.CreateCard(c[0], c[1], c[2]); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
