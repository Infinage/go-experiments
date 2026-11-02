# Flashcards Application - Constitution & Design

## Vision
A minimalist yet playful flashcard app with Tinder-style swipe review, arcade-inspired aesthetics, and cookie-based persistence. Built for individual learners who want an engaging, modern study experience.

## 1. Core Principles

### 1.1 Playful Arcade Aesthetic
- **Color palette**: High-contrast with accent colors for feedback (green = correct, red = wrong)
- **Typography**: Rounded, friendly sans-serif with character
- **Micro-animations**: Subtle motion on interactions (card flips, button presses, swipe feedback)
- **Sound effects**: Light audio cues for correct/incorrect answers (optional, user-configurable)

### 1.2 Tinder-Style Swipe Review
- Swipe right = correct answer (card moves right, advances)
- Swipe left = wrong answer (card moves left, advances)
- Visual feedback during and after swipe
- Review session summary with stats

### 1.3 Cookie-Based Session Management
- Lightweight token stored in HTTP-only cookie
- No server-side session state needed
- Persists across browser restarts (until cleared)
- Simple `sid` cookie name

### 1.4 Datastar SSE Reactivity
- Keep existing Datastar-go framework
- Use element patches for dynamic updates
- All DOM updates go through `patchMain` with element IDs

## 2. User Stories

Detailed user stories and acceptance criteria are maintained in `specs/stories/` to keep this document focused on enduring principles:

- **Epic A: Authentication** — see `specs/stories/authentication.md`
- **Epic B: Card Management** — see `specs/stories/card-management.md`
- **Epic C: Review System** — see `specs/stories/review-system.md`
- **Epic D: UI Interactions** — see `specs/stories/ui-interactions.md`

## 3. Technical Design

### 3.1 Architecture
```
Backend (Go 1.27)
├── Store layer (SQLite operations)
├── Server layer (HTTP handlers + Datastar wiring)
├── Middleware (CSRF, auth, session)
└── Templates (Datastar element sections)

Frontend (Datastar + Tailwind CSS)
├── Element sections: #main, #login, #home, #cardform, #import, #review, #summary
├── Signals for form state (prompt, answer, tags, csv)
└── Event handlers for all interactions
```

### 3.2 Database Schema (unchanged)
- `users`: username (PK), password (hashed salt+pbkdf2)
- `cards`: id (autoincrement), prompt, answer, user (FK)
- `tags`: name, card (composite PK)
- `stats`: user, card, next_review, current_interval_hours (composite PK)

### 3.3 Session Flow
1. User visits `/` → sees login form if no valid cookie
2. User submits login → server validates, sets `sid` cookie, patches to home
3. All subsequent requests include `sid` cookie → server looks up session
4. User logs out → cookie cleared, patches to login form

### 3.4 CSRF Protection
- Datastar requests must include `Datastar-Request: true` header
- Standard forms work with SameSite=Lax + cookie-based approach
- POST-only protection for non-GET methods

## 4. UI/UX Specification

### 4.1 Visual Design Language

#### Color System
- **Background**: Light slate gray (`bg-slate-50`) for calm base
- **Primary accent**: Indigo (`bg-indigo-600`, `text-indigo-600`) for actionable items
- **Success**: Emerald/green for correct answers
- **Error**: Red for wrong answers
- **Neutral**: Slate text colors, border colors

#### Typography
- Font family: System font stack, but with rounded feel
- Headings: Bold, larger scale
- Body: Medium weight for readability

#### Spacing & Layout
- `max-w-xl` centered content with `p-6` padding
- `space-y-4` vertical rhythm for sections
- `flex` and `grid` for component layout
- `divide-y` for list separators

### 4.2 Element Sections (Template IDs)

Each template defines a `<section id="main">` that Datastar patches in place:

| Section | Purpose | Key Elements |
|---------|---------|--------------|
| `login` | Authentication | Username/password inputs, login/signup buttons |
| `home` | Dashboard | User greeting, tag overview, CTA buttons |
| `cardform` | Add card | Textareas for prompt/answer, tags input, action buttons |
| `import` | CSV import | File input, header explanation, error/success messages |
| `review` | Study session | Current card prompt, answer reveal, swipe controls |
| `summary` | Review end | Right/wrong count, "Back" button |

### 4.3 Review Flow & Interaction Mechanics

The Tinder-style review flow, keyboard controls, micro-interactions, and sound effects are app requirements defined as user stories — not constitutional mandates:

- **Review flow & gestures**: see `specs/stories/review-system.md` (Epic C)
- **Micro-interactions, responsiveness, sound**: see `specs/stories/ui-interactions.md` (Epic D)

## 5. Refactoring & Migration

Implementation and refactoring tasks are tracked as user stories — not as an implementation plan embedded in this document:

- see `specs/stories/refactoring.md` (Epic E)

## 6. Directory Structure (Post-Refactor)

```
specs/
├── constitution.md          ← Design vision and enduring principles
└── stories/
    ├── authentication.md    ← Epic A: auth & session stories
    ├── card-management.md   ← Epic B: card CRUD & import stories
    ├── review-system.md     ← Epic C: swipe review & spaced repetition stories
    ├── ui-interactions.md   ← Epic D: micro-interactions & sound stories
    └── refactoring.md       ← Epic E: refactor & migration tasks

src/
├── pkg/
│   ├── types.go             ← Data models
│   ├── repo.go              ← Database operations
│   ├── handlers_auth.go     ← New: auth-only handlers
│   ├── handlers_cards.go    ← New: card CRUD handlers
│   ├── handlers_review.go   ← New: review session handlers
│   └── handlers_home.go     ← New: home/dashboard handlers
│
├── templates/
│   ├── app.html             ← Layout wrapper
│   ├── login.html           ← Auth page
│   ├── home.html            ← Dashboard page
│   ├── cardform.html        ← Add card form
│   ├── import.html          ← CSV import
│   ├── review.html          ← Study session (with swipe)
│   └── summary.html         ← Review end stats
│
├── assets/
│   ├── css/
│   │   ├── input.css        ← Tailwind source
│   │   └── index.css        ← Compiled output
│   └── js/
│       └── datastar.js      ← Datastar client
│
└── main.go                  ← Entry point (unchanged)
```

## 7. Migration Notes

### From Current to New Architecture
- **Handlers split**: `handlers.go` → multiple domain files
- **Templates updated**: Modern arcade styling, new element IDs
- **CSS enhanced**: Color variables, micro-animation keyframes
- **Session flow**: Same cookie-based approach, cleaner middleware
- **Review flow**: Major UX upgrade with swipe gestures

### Backward Compatibility
- Database schema unchanged ✓
- API endpoints restructured but compatible ✓
- Cookie name `sid` unchanged ✓
- Can start fresh or migrate existing DB ✓

---

*Constitution v1.0 — Playful, modern, swipe-review flashcards for the casual learner.*