// Package srs implements SM-2 style spaced repetition scheduling.
package srs

import "time"

// Grade is a review outcome.
type Grade int

const (
	Forgot Grade = iota // 0: did not recall
	Hard
	Medium
	Easy
)

// State is the scheduling state of one card.
type State struct {
	DueAt       time.Time
	Interval    int // days
	Ease        float64
	Repetitions int
	Lapses      int
}

// defaultEase is the SM-2 ease multiplier for fresh cards. Chosen so
// Medium intervals grow 1 → 6 → 12 → 24 days (mastered on the 4th).
const defaultEase = 2.0

// New returns the initial state for an unseen card: due immediately.
func New() State {
	return State{DueAt: time.Time{}, Interval: 0, Ease: defaultEase}
}

// MasteredDays is the interval (in days) at which a card counts as
// mastered. Tunable constant; cards reaching it can be hidden from study.
const MasteredDays = 21

// Mastered reports whether the state's interval has reached the
// mastered threshold.
func Mastered(s State) bool { return s.Interval >= MasteredDays }

// Review applies a grade and returns the next state, per SM-2:
// quality 0 (Again) resets; otherwise the interval grows 1 → 6 → ×ease.
// Ease adjusts from the grade and is clamped to a floor of 1.3.
func Review(s State, g Grade, now time.Time) State {
	next := s
	switch g {
	case Forgot:
		// Requeue in-session: the card stays due now; the study page
		// appends it to the end of the current pass.
		next.Repetitions = 0
		next.Lapses++
		next.Interval = 0
		next.Ease = clamp(s.Ease - 0.20)
		next.DueAt = now
	case Hard:
		next.Ease = clamp(s.Ease - 0.15)
		if s.Repetitions == 0 {
			next.Interval = 1
		} else {
			// Slight growth rather than a full ease multiple.
			next.Interval = max(1, int(float64(s.Interval)*1.2))
		}
		next.Repetitions++
		next.DueAt = now.AddDate(0, 0, next.Interval)
	case Medium:
		next.Interval = nextInterval(s, 1.0)
		next.Repetitions++
		next.DueAt = now.AddDate(0, 0, next.Interval)
	case Easy:
		next.Ease = clamp(s.Ease + 0.15)
		next.Interval = nextInterval(s, 1.3)
		next.Repetitions++
		next.DueAt = now.AddDate(0, 0, next.Interval)
	}
	return next
}

func nextInterval(s State, factor float64) int {
	switch s.Repetitions {
	case 0:
		return 1
	case 1:
		return 6
	default:
		return int(float64(s.Interval) * s.Ease * factor)
	}
}

func clamp(ease float64) float64 {
	if ease < 1.3 {
		return 1.3
	}
	return ease
}
