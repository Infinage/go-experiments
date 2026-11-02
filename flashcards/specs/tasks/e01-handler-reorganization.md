# E-01: Handler Reorganization — Implementation Record

**Status:** Complete
**Story:** US-E01, `specs/stories/refactoring.md`
**Date:** 2026-11-02

## What was done

`src/pkg/handlers.go` (538 lines) was split by domain. The original file no
longer exists; all endpoints are unchanged and verified working.

### New file layout (`src/pkg/`)

| File | Contents |
|------|----------|
| `server.go` | `Server`/`session` types, `NewServer`, route wiring (`Handler`), CSRF middleware, `patchMain` (SSE render), `readSignals`, `fail`, `maxBodySize` |
| `handlers_auth.go` | `cookieName`/`sessionTTL`/`pbkdf2Iter` consts, `authed` middleware, session `lookup`/`startSession`, `hashPassword`/`checkPassword`, `handleLogin`/`handleSignup`/`handleLogout`, `loginData` |
| `handlers_home.go` | `handleIndex` (full-page render), `buildHome`/`renderHome`/`handleHome`, `homeData`/`tagView` |
| `handlers_cards.go` | `parseTags`, `handleNewForm`/`handleCreateCard`, `handleImportForm`/`handleImport`, `parseCSV`, `formData`/`importData` |
| `handlers_review.go` | `review` struct, `handleReviewStart`/`handleReveal`/`handleAnswer`, `renderReview` (must be called with session lock held), `reviewData`/`summaryData` |

View-data structs moved next to the handlers that use them rather than into
`server.go`.

## Decisions (with user approval)

1. **Templates: `assets/html/` + `go:embed`**
   - The build was broken before this work: `handlers.go` referenced an
     undefined `tmpl` template set. Fixed as part of this task.
   - `templates/app.html` moved to `assets/html/app.html`, matching the
     existing `assets/css/` and `assets/js/` layout.
   - `main.go` embeds `assets/html` (`//go:embed assets/html`), slices it with
     `fs.Sub`, and passes the `fs.FS` to `flashcards.NewServer(st, tmplFS)`.
     The pkg package stays filesystem-agnostic; the binary needs no templates
     on disk at runtime.
   - `NewServer` now parses templates and returns `(*Server, error)`; template
     parse failures fail fast at startup.
2. **Shared infrastructure → new `server.go`**; `handlers.go` deleted rather
   than kept as a slim file.
3. **Documentation:** constitution.md was intentionally left untouched; this
   tasks file is the record of decisions instead.

## Verification

- `gofmt -l` clean, `go build ./...` OK, `go vet ./...` OK.
- New unit tests (both packages pass):
  - `handlers_cards_test.go`: table-driven `parseTags` (empty, trim/lowercase,
    dedupe, custom separator, blank-item dropping) and `parseCSV` (BOM header,
    missing columns, short rows, blank required fields, extra columns,
    header-only file) plus a tags/username parsing check.
  - `handlers_auth_test.go`: password hash round-trip, rejection of wrong
    password and malformed stored hashes.
- Live smoke test (all routes, run from a scratch dir with a fresh temp DB so
  the dev `flashcards.db` was untouched):

  ```bash
  go build -o /tmp/flashcards-test .
  mkdir -p /tmp/fc-smoke && cd /tmp/fc-smoke
  /tmp/flashcards-test &          # fresh DB created here, port 8080

  H='-sS -H Datastar-Request:true'   # required header for non-GET routes
  ```

  ### Anonymously (no cookie)

  ```bash
  # Renders the login page (full HTML, `page` template)
  curl -s http://localhost:8080/ | head -5

  # CSRF guard: POST without the Datastar-Request header -> 403
  curl -s -o /dev/null -w "%{http_code}\n" -X POST http://localhost:8080/cards
  ```

  ### Signup / login / session

  ```bash
  # Signup -> SSE patch of home + Set-Cookie: sid=... (HttpOnly, SameSite=Lax)
  curl $H -D headers1.txt -d '{"username":"smoketest","password":"password123"}' \
      http://localhost:8080/signup
  SID=$(grep -i set-cookie headers1.txt | sed 's/.*sid=\([^;]*\).*/\1/')

  # Full-page GET with the session cookie -> "Your tags" + "smoketest"
  curl -s -H "Cookie: sid=$SID" http://localhost:8080/home

  # Login (existing user) -> home patch + fresh sid cookie
  curl $H -D headers2.txt \
      -d '{"username":"smoketest","password":"password123"}' \
      http://localhost:8080/login

  # Logout -> login patch on the same response
  curl $H -H "Cookie: sid=$SID" -X POST http://localhost:8080/logout
  ```

  ### Cards

  ```bash
  # Create card -> home patch with "Card saved." flash; tags show on home
  # ("capitals", "geo") as 0 due · 1 total
  curl $H -H "Cookie: sid=$SID" \
      -d '{"prompt":"Capital of France?","answer":"Paris","tags":"Geo, Capitals"}' \
      http://localhost:8080/cards

  # ?more=1 keeps the user on the form -> "Saved. Add another:"
  curl $H -H "Cookie: sid=$SID" -d '{"prompt":"2+2?","answer":"4","tags":"math"}' \
      "http://localhost:8080/cards?more=1"
  ```

  ### Review flow

  ```bash
  # Start (only due cards are queued; the already-answered geo card was
  # correctly skipped because its next_review moved into the future)
  curl $H -H "Cookie: sid=$SID" -X POST "http://localhost:8080/review/start?tag=geo"
      # -> prompt "Capital of France?"

  curl $H -H "Cookie: sid=$SID" -X POST http://localhost:8080/review/reveal
      # -> answer "Paris"

  curl $H -H "Cookie: sid=$SID" -X POST \
      "http://localhost:8080/review/answer?i=0&correct=1"
      # -> next card / summary when queue is exhausted

  curl $H -H "Cookie: sid=$SID" -X POST \
      "http://localhost:8080/review/answer?i=1&correct=0"
      # -> summary template (Right/Wrong counts) after the last card

  # Double-submit guard: stale i=0 after the session ended -> home patch
  curl $H -H "Cookie: sid=$SID" -X POST \
      "http://localhost:8080/review/answer?i=0&correct=1"
  ```

  ### CSV import

  ```bash
  # Valid file (base64 of "prompt,answer,tags\nQ1,A1,imported\n")
  # -> home patch with "Imported 1 cards."; tag "imported" appears with 1 due
  CSV_B64=$(printf 'prompt,answer,tags\nQ1,A1,imported\n' | base64 -w0)
  curl $H -H "Cookie: sid=$SID" -d "{\"csv\":[\"$CSV_B64\"]}" \
      -X POST http://localhost:8080/cards/import

  # Bad header -> friendly error "Header must include 'prompt' and 'answer'
  # columns."
  CSV_B64=$(printf 'question,answer\nQ,A\n' | base64 -w0)
  curl $H -H "Cookie: sid=$SID" -d "{\"csv\":[\"$CSV_B64\"]}" \
      -X POST http://localhost:8080/cards/import
  ```

  Results observed:
  - `GET /` renders login page anonymously; signup sets `sid` cookie and
    patches home; login works; `GET /home` shows user's tags.
  - `POST /cards` saves and returns home with flash; `?more=1` stays on form.
  - `POST /review/start?tag=…` queues due cards; `review/reveal` shows the
    answer; `review/answer?i=N&correct=…` advances and renders the summary
    (Right/Wrong counts) when the queue is exhausted.
  - Double-submit guard confirmed: a stale `i` after session end returns home.
  - CSV import: valid file imports ("Imported 1 cards."), bad header produces
    the friendly "Header must include 'prompt' and 'answer' columns." message.
  - CSRF guard confirmed: POST without `Datastar-Request` header → 403.
  - Logout clears the session; subsequent requests get the login patch.

  Scripting gotchas hit along the way (not app bugs): two earlier greps came
  back empty because the script used GET on POST-only routes (`Method Not
  Allowed`), and later requests ran after logout so they got the login patch —
  both re-tested correctly with proper POSTs and a fresh session.

## Notes / open items (not part of E-01)

- No static-file handler exists for `/assets/css/index.css` or
  `/assets/js/datastar.js`; templates reference both but `Handler()` doesn't
  serve them (pre-existing gap, likely covered by E-02 work).
- `constitution.md` §3.1 could later be amended to describe the handler split
  and the `assets/html` embed decision if we want the architecture diagram to
  stay current.
