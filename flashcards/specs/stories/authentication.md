# Authentication & Onboarding

Aligned with Constitution v1.0 - cookie-based sessions, arcade-style flashcards.

## Epic A-01: Register Account
- **US-A01: Register Account**
  - As a new user, I want to register with a unique username and password so I can have my own study space.
  - Acceptance: Username 1-32 chars, password minimum 8 chars, unique username validation, cookie-based login after registration.

## Epic A-02: Login
- **US-A02: Login**
  - As a returning user, I want to login with my credentials so I can access my flashcards.
  - Acceptance: Credentials validated against stored hash, `sid` cookie set, redirected to home.

## Epic A-03: Logout
- **US-A03: Logout**
  - As a logged-in user, I want to logout so my session is cleared.
  - Acceptance: `sid` cookie cleared, redirected to login form, session removed from server memory.

## Epic A-04: Session Persistence
- **US-A04: Session Persistence**
  - As a user, I want my login to persist across browser sessions (until cookie clear).
  - Acceptance: `sid` cookie stored HTTP-only, server validates on each request, no server-side session state needed.