# Refactoring & Migration

Aligned with Constitution v1.0 - handler reorganization, arcade UI implementation.

## Epic E-01: Handler Reorganization
- **US-E01: Split Crowded handlers.go**
  - As a developer, I want handlers organized into domain-specific files so the codebase is maintainable.
  - Acceptance: `handlers.go` split into `handlers_auth.go` (auth + session middleware), `handlers_cards.go` (card CRUD + CSV import), `handlers_review.go` (review session + summary), `handlers_home.go` (home/dashboard). All existing endpoints keep working after the split.

## Epic E-02: Template & Styling Updates
- **US-E02: Arcade-Style Templates**
  - As a developer, I want templates updated to the arcade aesthetic so the UI matches the design vision.
  - Acceptance: All templates updated with the constitution's color system (slate base, indigo accent, emerald success, red error), `max-w-xl` centered layout, playful typography.

- **US-E03: CSS Theming**
  - As a developer, I want CSS variables for color theming so the palette is centralized.
  - Acceptance: Color variables defined in `assets/css/input.css`, Tailwind build picks them up, no hardcoded colors scattered across templates.

## Epic E-03: Review System Overhaul
- **US-E04: Swipe/Drag Gesture Implementation**
  - As a developer, I want swipe/drag gesture handling in templates so the Tinder-style review works.
  - Acceptance: Card element draggable horizontally, tilt during drag, snap-to-edge on release, wired to review answer endpoints.

- **US-E05: Sound Effects Integration**
  - As a developer, I want sound effects via Web Audio API so answers have audio feedback.
  - Acceptance: "Ding" on correct, "buzzer" on incorrect, subtle "whoosh" on swipe, respects system mute.

- **US-E06: Keyboard Controls**
  - As a developer, I want keyboard controls wired to the review flow so power users can review without a mouse.
  - Acceptance: → arrow = correct, ← arrow = wrong, Escape = stop/reveal. (App-facing behavior: US-C06.)

## Epic E-04: Polish & Testing
- **US-E07: Mobile Responsiveness Verification**
  - As a developer, I want the UI verified on mobile so the layout adapts gracefully.
  - Acceptance: Touch targets minimum 44px, full-width inputs on small screens, tested on common viewport sizes. (App-facing behavior: US-D05.)

- **US-E08: Cross-Browser Testing**
  - As a developer, I want the app tested across browsers so behavior is consistent.
  - Acceptance: Swipe gestures, sounds, and Datastar patches verified in Chromium and Firefox.

- **US-E09: Error Handling Edge Cases**
  - As a developer, I want edge cases handled so the app fails gracefully.
  - Acceptance: Malformed CSV rows, empty inputs, DB errors, and expired sessions all produce friendly messages instead of crashes.