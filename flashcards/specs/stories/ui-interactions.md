# UI Interactions & Micro-Animitations

Aligned with Constitution v1.0 - playful arcade feel, responsive interactions.

## Status legend

- ✅ Complete (verified in code)
- ⚠️ Partial
- ❌ Pending

## Epic D-01: Playful Button Animations
- **US-D01: Playful Button Animations** — ❌ Pending
  - As a user, I want buttons to feel responsive and lively.
  - Acceptance: Scale-down 0.95x on press, restore on release, subtle shadow change, color hover transitions.
  - Status: Templates have `hover:` color states only; no active/press scale or shadow transitions.

## Epic D-02: Input Focus Effects
- **US-D02: Input Focus Effects** — ❌ Pending
  - As a user, I want inputs to feel welcoming.
  - Acceptance: Border glow on focus, outline pulse when empty, smooth focus-in/out transitions.
  - Status: Inputs are plain utility-styled; no focus styling present.

## Epic D-03: Card Swipe Tilt
- **US-D03: Card Swipe Tilt** — ❌ Pending
  - As a user, I want visual feedback of the drag direction.
  - Acceptance: Card rotates 5deg in swipe direction during drag, natural snap-back on release, ease-out animation.
  - Status: No drag/swipe logic exists in any template. (Consumer stories: US-C02/US-C03; developer story: US-E04.)

## Epic D-04: Feedback Color Flash
- **US-D04: Feedback Color Flash** — ❌ Pending
  - As a user, I want immediate visual confirmation.
  - Acceptance: Brief color pulse (green for correct, red for wrong) on card, 300ms duration, then fades to normal.
  - Status: Not implemented; answers currently swap templates with no transition.

## Epic D-05: Responsive Layout
- **US-D05: Responsive Layout** — ⚠️ Partial
  - As a user on mobile or desktop, I want the UI to adapt gracefully.
  - Acceptance: max-w-xl centered container, mobile-full width inputs, touch-friendly tap targets minimum 44px, collapsed navigation on small screens.
  - Status: `max-w-xl p-6` container and mobile viewport meta done; 44px touch targets and mobile-width refinements unverified/unfinished. (Developer story: US-E07.)

## Epic D-06: Sound Effects
- **US-D06: Sound Effects** — ❌ Pending
  - As a user, I want audio feedback for correct/incorrect answers.
  - Acceptance: Gentle "ding" for correct (frequency 523Hz C5), soft "buzzer" for incorrect, respects system mute, optional toggle in settings (future).
  - Status: No Web Audio code anywhere. (Developer story: US-E05.)