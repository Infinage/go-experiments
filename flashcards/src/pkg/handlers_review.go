package flashcards

import (
	"fmt"
	"net/http"
)

// review represents a single active review session, tied to the user's
// current session lifetime (starts when the user picks a tag to study, ends
// when the queue is exhausted and the summary is shown).
type review struct {
	tag          string // the tag this review session was filtered by
	queue        []Card // cards due under that tag, in review order
	idx          int    // index into queue of the next card to review; equals len(queue) when done
	right, wrong int    // tallies of how many cards were answered correctly/incorrectly
}

// handleReviewStart is called when the user clicks a tag on the home page to
// begin studying (POST /review/start?tag=...). It looks up due cards for that
// tag, initializes a new review session in the user's session, and patches the
// review UI to show the first card.
func (s *Server) handleReviewStart(w http.ResponseWriter, r *http.Request, sess *session) {
	tag := r.URL.Query().Get("tag")
	cards, err := s.store.DueCardsByTag(r.Context(), sess.user, tag)
	if err != nil {
		fail(w, err)
		return
	}
	if len(cards) == 0 {
		s.renderHome(w, r, sess.user, fmt.Sprintf("Nothing due under %q right now.", tag))
		return
	}
	sess.mu.Lock()
	defer sess.mu.Unlock()
	sess.review = &review{tag: tag, queue: cards}
	s.renderReview(w, r, sess, false)
}

// handleReveal is called when the user reveals the answer for the current
// review card (POST /review/reveal). It re-renders the review card with the
// answer visible, without advancing the queue.
func (s *Server) handleReveal(w http.ResponseWriter, r *http.Request, sess *session) {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	if sess.review == nil {
		s.renderHome(w, r, sess.user, "")
		return
	}
	s.renderReview(w, r, sess, true)
}

// handleAnswer is called when the user marks the current review card as
// correct or incorrect (POST /review/answer?i=...&correct=...). It records the
// result via the store, advances the review queue, and patches the next card
// (or the summary once the queue is exhausted).
func (s *Server) handleAnswer(w http.ResponseWriter, r *http.Request, sess *session) {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	rv := sess.review
	if rv == nil {
		s.renderHome(w, r, sess.user, "")
		return
	}
	// Ignore double-submits: the client sends the index it answered.
	if r.URL.Query().Get("i") != fmt.Sprint(rv.idx) || rv.idx >= len(rv.queue) {
		s.renderReview(w, r, sess, false)
		return
	}
	correct := r.URL.Query().Get("correct") == "1"
	err := s.store.UpsertCardStat(r.Context(), sess.user, rv.queue[rv.idx].ID, correct)
	if err != nil {
		fail(w, err)
		return
	}
	if correct {
		rv.right++
	} else {
		rv.wrong++
	}
	rv.idx++
	s.renderReview(w, r, sess, false)
}

// renderReview must be called with sess.mu held.
func (s *Server) renderReview(w http.ResponseWriter, r *http.Request, sess *session, revealed bool) {
	rv := sess.review
	if rv.idx >= len(rv.queue) {
		s.patchMain(w, r, "summary", map[string]any{
			"Tag": rv.tag, "Right": rv.right, "Wrong": rv.wrong,
		})
		sess.review = nil
		return
	}
	c := rv.queue[rv.idx]
	s.patchMain(w, r, "review", map[string]any{
		"Tag": rv.tag, "Idx": rv.idx, "Pos": rv.idx + 1, 
		"Total": len(rv.queue), "Prompt": c.Prompt, 
		"Answer": c.Answer, "Revealed": revealed,
	})
}
