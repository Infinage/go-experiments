# Refactoring & Migration

Aligned with Constitution v1.0 - handler reorganization, arcade UI implementation.

## Status legend

- ✅ Complete (verified in code)
- ⚠️ Partial
- ❌ Pending

Task records for completed work live in `specs/tasks/`.

## Epic E-01: Handler Reorganization
- **US-E01: Split Crowded handlers.go** — ✅ Complete
  - As a developer, I want handlers organized into domain-specific files so the codebase is maintainable.
  - Acceptance: `handlers.go` split into `handlers_auth.go` (auth + session middleware), `handlers_cards.go` (card CRUD + CSV import), `handlers_review.go` (review session + summary), `handlers_home.go` (home/dashboard). All existing endpoints keep working after the split.
  - Status: Done — see `specs/tasks/e01-handler-reorganization.md`, plus follow-ups `e01b-atomic-cardstat-upsert.md` and `e01c-datastar-sdk-and-code-review.md`.

## Epic E-02: Template & Styling Updates
- **US-E02: Arcade-Style Templates** — ⚠️ Partial
  - As a developer, I want templates updated to the arcade aesthetic so the UI matches the design vision.
  - Acceptance: All templates updated with the constitution's color system (slate base, indigo accent, emerald success, red error), `max-w-xl` centered layout, playful typography.
  - Status: Templates in `assets/html/app.html` use the slate/indigo/emerald/red utility palette and `max-w-xl p-6` layout; playful rounded typography absent; colors are hardcoded utility classes rather than variables. Static file handler for `/assets/css/index.css` and `/assets/js/datastar.js` now exists (embedded FS, served in `server.go`; templates excluded from the public FS).

- **US-E03: CSS Theming** — ✅ Complete
  - As a developer, I want CSS variables for color theming so the palette is centralized.
  - Acceptance: Color variables defined in `assets/css/input.css`, Tailwind build picks them up, no hardcoded colors scattered across templates.
  - Status: `assets/css/input.css` is the single home for custom theme tokens and all `@keyframes` (exposed as `animate-<name>` utilities via Tailwind v4 `@theme`). Per user decision, Tailwind's built-in palette covers the constitution's color system, so no custom color tokens are defined — custom colors get added only when Tailwind lacks them. First token: `--animate-confirm-pulse` (used by the bulk/single delete confirm).

## Epic E-03: Review System Overhaul
- **US-E04: Swipe/Drag Gesture Implementation** — ❌ Pending
  - As a developer, I want swipe/drag gesture handling in templates so the Tinder-style review works.
  - Acceptance: Card element draggable horizontally, tilt during drag, snap-to-edge on release, wired to review answer endpoints.
  - Status: Review flow is button-driven; no drag/gesture code. (App-facing stories: US-C02/US-C03, US-D03.)

- **US-E05: Sound Effects Integration** — ❌ Pending
  - As a developer, I want sound effects via Web Audio API so answers have audio feedback.
  - Acceptance: "Ding" on correct, "buzzer" on incorrect, subtle "whoosh" on swipe, respects system mute.
  - Status: No Web Audio code. (App-facing story: US-D06.)

- **US-E06: Keyboard Controls** — ❌ Pending
  - As a developer, I want keyboard controls wired to the review flow so power users can review without a mouse.
  - Acceptance: → arrow = correct, ← arrow = wrong, Escape = stop/reveal. (App-facing behavior: US-C06.)
  - Status: No keyboard handling in templates.

## Epic E-04: Polish & Testing
- **US-E07: Mobile Responsiveness Verification** — ❌ Pending
  - As a developer, I want the UI verified on mobile so the layout adapts gracefully.
  - Acceptance: Touch targets minimum 44px, full-width inputs on small screens, tested on common viewport sizes. (App-facing behavior: US-D05.)

- **US-E08: Cross-Browser Testing** — ❌ Pending
  - As a developer, I want the app tested across browsers so behavior is consistent.
  - Acceptance: Swipe gestures, sounds, and Datastar patches verified in Chromium and Firefox.

- **US-E09: Error Handling Edge Cases** — ⚠️ Partial
  - As a developer, I want edge cases handled so the app fails gracefully.
  - Acceptance: Malformed CSV rows, empty inputs, DB errors, and expired sessions all produce friendly messages instead of crashes.
  - Status: CSV errors, empty inputs, bad headers, and expired sessions produce friendly messages (verified in e01 smoke test + e01c). DB failures still surface as raw 500s via `fail()`; friendly DB-error messages remain.