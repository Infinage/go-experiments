# Review System (Tinder-Style Swipe)

Aligned with Constitution v1.0 - swipe-right=correct, swipe-left=wrong, arcade feedback.

## Status legend

- ✅ Complete (verified in code)
- ⚠️ Partial
- ❌ Pending

## Epic C-01: Start Review Session
- **US-C01: Start Review Session** — ✅ Complete
  - As a user, I want to start a review session for a specific tag so I can study those cards.
  - Acceptance: Filter cards by tag and due status (next_review <= now or no review), initialize review state, display first card.
  - Status: `POST /review/start` in `handlers_review.go`; due filtering via `DueCardsByTag` with UTC `next_review` (per e01b).

## Epic C-02: Swipe Right = Correct
- **US-C02: Swipe Right = Correct** — ❌ Pending (backend only)
  - As a user reviewing a card, I want to swipe right to mark my answer as correct.
  - Acceptance: Swipe gesture (mouse drag right or → arrow key), card tilts during drag, snaps to right edge on release, green feedback flash, "ding" sound, interval doubled via API, advances to next card.
  - Status: `POST /review/answer?correct=1` exists and doubles the interval, but the UI is a button ("Got it"). No drag/tilt/snap, flash, or sound — frontend work pending (US-E04, US-D03, US-D04, US-D06).

## Epic C-03: Swipe Left = Wrong
- **US-C03: Swipe Left = Wrong** — ❌ Pending (backend only)
  - As a user reviewing a card, I want to swipe left to mark my answer as incorrect.
  - Acceptance: Swipe gesture (mouse drag left or ← arrow key), card tilts during drag, snaps to left edge on release, red feedback flash, "buzzer" sound, interval reset to 1 hour, advances to next card.
  - Status: `POST /review/answer?correct=0` exists and resets the interval, but the UI is a button ("Missed it"). Frontend work pending (US-E04, US-D03, US-D04, US-D06).

## Epic C-04: Reveal Answer
- **US-C04: Reveal Answer** — ✅ Complete
  - As a user seeing a card prompt, I want to reveal the answer before committing.
  - Acceptance: Button to toggle answer visibility, animation of flip/reveal, keeps current review state intact.
  - Status: `POST /review/reveal` in `handlers_review.go`. Note: reveal animation (flip) not implemented — cosmetic part tracked under Epic D.

## Epic C-05: Review Session Summary
- **US-C05: Review Session Summary** — ✅ Complete
  - As a user finishing a review session, I want to see my performance summary.
  - Acceptance: Right/wrong count displayed, tag name shown, "Back to tags" button, stats persisted for each card reviewed.
  - Status: summary template rendered by `renderReview` when the queue is exhausted; stats persisted via `UpsertCardStat` (atomic, per e01b).

## Epic C-06: Keyboard Controls
- **US-C06: Keyboard Controls** — ❌ Pending
  - As a power user, I want keyboard shortcuts for reviewing without a mouse.
  - Acceptance: → arrow key = swipe right (correct), ← arrow key = swipe left (wrong), Escape = stop/reveal, Focus stays on card area.
  - Status: No keyboard handling in any template. (Developer-side story: US-E06.)