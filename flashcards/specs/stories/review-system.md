# Review System (Tinder-Style Swipe)

Aligned with Constitution v1.0 - swipe-right=correct, swipe-left=wrong, arcade feedback.

## Epic C-01: Start Review Session
- **US-C01: Start Review Session**
  - As a user, I want to start a review session for a specific tag so I can study those cards.
  - Acceptance: Filter cards by tag and due status (next_review <= now or no review), initialize review state, display first card.

## Epic C-02: Swipe Right = Correct
- **US-C02: Swipe Right = Correct**
  - As a user reviewing a card, I want to swipe right to mark my answer as correct.
  - Acceptance: Swipe gesture (mouse drag right or → arrow key), card tilts during drag, snaps to right edge on release, green feedback flash, "ding" sound, interval doubled via API, advances to next card.

## Epic C-03: Swipe Left = Wrong
- **US-C03: Swipe Left = Wrong**
  - As a user reviewing a card, I want to swipe left to mark my answer as incorrect.
  - Acceptance: Swipe gesture (mouse drag left or ← arrow key), card tilts during drag, snaps to left edge on release, red feedback flash, "buzzer" sound, interval reset to 1 hour, advances to next card.

## Epic C-04: Reveal Answer
- **US-C04: Reveal Answer**
  - As a user seeing a card prompt, I want to reveal the answer before committing.
  - Acceptance: Button to toggle answer visibility, animation of flip/reveal, keeps current review state intact.

## Epic C-05: Review Session Summary
- **US-C05: Review Session Summary**
  - As a user finishing a review session, I want to see my performance summary.
  - Acceptance: Right/wrong count displayed, tag name shown, "Back to tags" button, stats persisted for each card reviewed.

## Epic C-06: Keyboard Controls
- **US-C06: Keyboard Controls**
  - As a power user, I want keyboard shortcuts for reviewing without a mouse.
  - Acceptance: → arrow key = swipe right (correct), ← arrow key = swipe left (wrong), Escape = stop/reveal, Focus stays on card area.