package srs

import (
	"testing"
	"time"
)

var now = time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)

func TestNewCardDueImmediately(t *testing.T) {
	s := New()
	if !s.DueAt.IsZero() || s.Interval != 0 || s.Ease != 2.5 {
		t.Fatalf("new state: %+v", s)
	}
}

func TestFirstReviewsUseLearningSteps(t *testing.T) {
	s := Review(New(), Good, now)
	if s.Interval != 1 || s.Repetitions != 1 {
		t.Fatalf("first good: %+v", s)
	}
	s = Review(s, Good, now)
	if s.Interval != 6 {
		t.Fatalf("second good: %+v", s)
	}
	s = Review(s, Good, now)
	if s.Interval != 15 { // 6 * 2.5
		t.Fatalf("third good: %+v", s)
	}
	if want := now.AddDate(0, 0, 15); !s.DueAt.Equal(want) {
		t.Fatalf("due %v, want %v", s.DueAt, want)
	}
}

func TestAgainResetsAndLapses(t *testing.T) {
	s := Review(Review(Review(New(), Good, now), Good, now), Good, now)
	s = Review(s, Again, now)
	if s.Repetitions != 0 || s.Lapses != 1 || s.Interval != 0 {
		t.Fatalf("again: %+v", s)
	}
	if s.DueAt.Before(now) || s.DueAt.After(now.Add(time.Hour)) {
		t.Fatalf("again due soon: %v", s.DueAt)
	}
	// Ease dropped.
	if s.Ease >= 2.5 {
		t.Fatalf("ease not reduced: %v", s.Ease)
	}
}

func TestEaseFloorsAt1_3(t *testing.T) {
	s := State{Ease: 1.35, Interval: 10, Repetitions: 3}
	s = Review(s, Again, now)
	if s.Ease != 1.3 {
		t.Fatalf("ease floor: %v", s.Ease)
	}
}

func TestHardGrowsSlowly(t *testing.T) {
	s := State{Ease: 2.5, Interval: 10, Repetitions: 3}
	s = Review(s, Hard, now)
	if s.Interval != 12 { // 10 * 1.2
		t.Fatalf("hard interval: %d", s.Interval)
	}
	if s.Ease != 2.35 {
		t.Fatalf("hard ease: %v", s.Ease)
	}
}

func TestEasyBoosts(t *testing.T) {
	s := State{Ease: 2.5, Interval: 10, Repetitions: 3}
	s = Review(s, Easy, now)
	if s.Interval != 32 { // int(10 * 2.5 * 1.3)
		t.Fatalf("easy interval: %d", s.Interval)
	}
	if s.Ease != 2.65 {
		t.Fatalf("easy ease: %v", s.Ease)
	}
}

func TestMastered(t *testing.T) {
	if Mastered(State{Interval: 20}) {
		t.Fatal("20 days should not be mastered")
	}
	if !Mastered(State{Interval: MasteredDays}) {
		t.Fatal("threshold interval should be mastered")
	}
}

func TestReviewIsDeterministic(t *testing.T) {
	s := New()
	a := Review(Review(s, Good, now), Hard, now)
	b := Review(Review(s, Good, now), Hard, now)
	if a != b {
		t.Fatalf("non-deterministic: %+v vs %+v", a, b)
	}
}
