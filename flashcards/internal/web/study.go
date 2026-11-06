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

// gradeChoice is one study grade button with its meaning hint.
type gradeChoice struct {
	Value, Label, Hint string
}

// deckSeg is one category's slice of the Decks stacked bar.
type deckSeg struct {
	Name          string
	Total         int
	Mastered      int
	Learning      int
	NotStarted    int
	MasteredPct   int
	LearningPct   int
	NotStartedPct int
}

// studyData backs the whole study page and its card fragment.
type studyData struct {
	CardFlip     cardFlipView
	Card         *store.StudyCard // nil when nothing is due
	Stats        store.StudyStats
	HideMastered bool
	DeckBar      []deckSeg
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
		// Toggle or refresh: patch just the study area, and clear any
		// busy signal left by the replaced triggering button.
		sse := datastar.NewSSE(w, r)
		if err := sse.PatchSignals(busyResetSignals()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := sse.PatchElements(s.templateHTML("study-area", d)); err != nil {
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
	bar := make([]deckSeg, len(decks))
	for i, d := range decks {
		bar[i] = deckSeg{Name: d.Name, Total: d.Total,
			Mastered: d.Mastered, Learning: d.Learning,
			NotStarted: d.NotStarted, MasteredPct: d.MasteredPct,
			LearningPct: d.LearningPct, NotStartedPct: d.NotStartedPct}
	}
	d := &studyData{Stats: stats, HideMastered: sig.HideMastered, DeckBar: bar,
		Category: categoryIn(cats, sig.Category), Categories: cats,
		Idx: idx, Count: len(q), Decks: decks, Force: sig.Force}
	if idx < len(q) {
		d.Card = &q[idx]
		d.CardFlip = studyCardFlip(&q[idx])
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

// busyResetSignals clears every per-button busy signal. SSE patches
// replace the triggering button mid-flight, so its indicator cleanup
// never runs client-side; the response clears the signals instead.
func busyResetSignals() []byte {
	return []byte(`{"busyShuffle": false, "busyAgain": false,
		"busyHard": false, "busyGood": false, "busyEasy": false,
		"busyPrev": false, "busyNext": false, "busyReset": false}`)
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
		`{"revealed": false, "idx": 0, "grade": ""}`)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := sse.PatchSignals(busyResetSignals()); err != nil {
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
	if err := sse.PatchSignals(busyResetSignals()); err != nil {
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
		{"HTTP status 301 means…", "Moved Permanently", "web"},
		{"Which HTML tag embeds JavaScript?", "<script>", "web"},
		{"CSS property for rounded corners", "border-radius", "web"},
		{"CSS display value that stacks elements side by side", "flex (or inline-flex)", "web"},
		{"What does JSON stand for?", "JavaScript Object Notation", "web"},
		{"Which HTTP verb is idempotent and caches?", "GET", "web"},
		{"Meta tag for mobile viewport width", `<meta name="viewport">`, "web"},
		{"Go keyword to launch a goroutine", "go", "langs"},
		{"Go type for a dynamically-sized slice of bytes", "[]byte", "langs"},
		{"Which Go construct handles concurrent receive?", "select", "langs"},
		{"Go function that runs deferred cleanups?", "defer", "langs"},
		{"Map type syntax in Go", "map[string]int", "langs"},
		{"What does 'nil' mean in Go?", "The zero value of pointers, slices, maps, channels and functions", "langs"},
		{"Which SQL clause filters rows?", "WHERE", "db"},
		{"SQL command to change a table schema?", "ALTER TABLE", "db"},
		{"What does JOIN do?", "Combines rows from two tables on a matching column", "db"},
		{"SQL aggregate for counting rows?", "COUNT(*)", "db"},
		{"Which index type full-text search uses in SQLite?", "FTS5 virtual table", "db"},
		{"What is a database transaction?", "A group of operations applied all-or-nothing", "db"},
		{"Capital of Australia?", "Canberra", "geography"},
		{"Capital of Canada?", "Ottawa", "geography"},
		{"Longest river in Africa?", "The Nile", "geography"},
		{"Largest desert on Earth?", "Antarctic Desert", "geography"},
		{"Which country has the most time zones?", "France (12)", "geography"},
		{"Mount Everest sits on the border of…", "Nepal and China", "geography"},
		{"Spanish for 'Good morning'", "Buenos días", "spanish"},
		{"Spanish for 'Thank you'", "Gracias", "spanish"},
		{"Spanish for 'How much does it cost?'", "¿Cuánto cuesta?", "spanish"},
		{"Spanish for 'Where is the bathroom?'", "¿Dónde está el baño?", "spanish"},
		{"Spanish for 'I would like…'", "Me gustaría…", "spanish"},
		{"Spanish for 'See you later'", "Hasta luego", "spanish"},
		{"How many strings does a standard guitar have?", "Six", "music"},
		{"Tempo marking for 'slow'", "Largo or Adagio", "music"},
		{"What does 'forte' mean?", "Loud", "music"},
		{"Key signature with no sharps or flats?", "C major / A minor", "music"},
		{"Speed of light in vacuum?", "299,792,458 m/s", "science"},
		{"Chemical symbol for gold?", "Au", "science"},
		{"Powerhouse of the cell?", "Mitochondria", "science"},
		{"A very long card front to test text wrapping and overflow handling in the retro card layout across desktop and mobile viewports, with several clauses and no manual line breaks anywhere in the string", "A very long card back that also needs to wrap correctly inside the mint answer face, keeping the border and padding intact while remaining fully readable on narrow phone screens as well as wide desktop panes.", "web"},
		{"Who painted the Mona Lisa?", "Leonardo da Vinci", "art"},
		{"Which movement was Salvador Dalí part of?", "Surrealism", "art"},
		{"Primary colors of the RYB model?", "Red, yellow, blue", "art"},
		{"Who wrote 'One Hundred Years of Solitude'?", "Gabriel García Márquez", "books"},
		{"Author of the 'Discworld' series?", "Terry Pratchett", "books"},
		{"In which year did the Berlin Wall fall?", "1989", "history"},
		{"Who was the first emperor of Rome?", "Augustus", "history"},
		{"The Industrial Revolution began in which country?", "Great Britain", "history"},
		{"Main ingredient of guacamole?", "Avocado", "cooking"},
		{"What is 'umami'?", "The savoury basic taste", "cooking"},
		{"Which film won the first Academy Award for Best Picture?", "Wings (1929)", "movies"},
		{"Who directed 'Inception'?", "Christopher Nolan", "movies"},
		{"How often are the Summer Olympics held?", "Every four years", "sports"},
		{"In which sport would you perform a slam dunk?", "Basketball", "sports"},
		{"What does DNA stand for?", "Deoxyribonucleic acid", "science"},
		{"Speed of a sneeze can exceed…", "160 km/h", "science"},
	}
	for _, c := range seeds {
		if _, err := s.store.CreateCard(c[0], c[1], c[2]); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
