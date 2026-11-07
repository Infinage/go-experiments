# Flashcards – Implementation Plan

## Decisions (from discussion)
- Single user, **no login/sign-up** (mockup sign-up screens are dropped).
- **Local-first**: Go + SQLite run on the same device as the UI. Build it as a locally hosted app first; mobile packaging later.
- Server-rendered HTML with **Datastar** (SSE), **Tailwind**, `html/template`.
- **Spaced repetition (SM-2 style)** scheduling.
- Image/audio allowed on the **card front only**; the back is text only. Stored as **BLOBs in SQLite**. No CSV import. Export/backup = copy the `.db` file.
- Retro look from `Retro Flashcards.html`: Study, All caught up, All cards + search, New card, Edit card (desktop and mobile layouts).

## Packaging reality check (Wails v3)
- Wails v3 is stable for **desktop**. **Android/iOS are experimental**: the same `main.go` and web frontend run in a native WebView, Go is built as a `.so` via the NDK (Android), and iOS needs macOS + Xcode.
- It *can* produce an APK, but expect rough edges. Treat mobile as a spike, not a dependency of the demo.
- **Pi demo fallback (works today):** run the Go binary on the Pi (linux/arm64), open it from the phone browser, and "Add to Home Screen" through a minimal PWA manifest. The data is then on the Pi, not the phone. This is a demo fallback, not the local-first goal.
- Keep the core as a plain `http.Handler` so the same code runs standalone, under Wails, or on the Pi.

## Architecture
```
main.go                  wiring, flags (-addr, -db), embed assets
internal/store/          SQLite access (modernc.org/sqlite, pure Go, no CGO)
internal/srs/            SM-2 scheduling (pure functions, unit tested)
internal/web/            handlers, Datastar SSE responses, templates
assets/html/             templates (layout, study, cards, form partials)
assets/css|js            tailwind output, datastar.js (vendored, //go:embed)
```
- **Dependency to approve:** `modernc.org/sqlite`. It's pure Go, so it cross-compiles to the Pi and Android without CGO. `mattn/go-sqlite3` needs CGO, which makes Android/Pi builds harder.
- `go:embed` templates and assets so it's a single binary.

## Data model
```sql
cards(id INTEGER PK, front_text, back_text,
      front_image BLOB, front_image_mime,
      front_audio BLOB, front_audio_mime,      -- media on the front only; back is text only
      created_at, updated_at)
review_state(card_id PK→cards ON DELETE CASCADE,
      due_at, interval_days, ease REAL, repetitions, lapses)
review_log(id, card_id, reviewed_at, grade)      -- optional, for stats
-- Status is derived per review_state: 'new' (no row), 'mastered'
-- (interval_days >= 21), else 'learning'. No mastered boolean.
```
- Media is served from `GET /cards/{id}/media/{image|audio}` (with `Content-Type`, ETag, and Range for audio). Don't inline BLOBs into list queries; select them only in the media handler.
- Upload limit (e.g. 5 MB image, 10 MB audio). Images are downscaled client-side, or the limit is enforced server-side.
- SQLite pragmas: WAL, foreign_keys=ON, busy_timeout.
- Embedded migrations (simple `user_version` runner).

## SRS (SM-2 style)
- Grades in the UI: **Forgot / Hard / Medium / Easy**, which map to SM-2
  quality scores.
- Forgot → repetitions=0, interval=0, ease −0.20, lapses++, due immediately;
  the card requeues in-session once at the end of the pass.
- Otherwise interval = 1, 6, then interval × ease; ease is adjusted and clamped to ≥ 1.3.
- "Mastered" = interval ≥ 21 days (threshold constant). "Hide mastered" is a study filter.
- Study queue = cards with `due_at <= now`, plus all never-reviewed cards
  (no daily cap). Empty queue → the "All caught up" screen with the next
  due time.
- `srs` is pure and takes `now` as a parameter, so it's deterministic to test.

## Screens and routes (Datastar)
| Screen | Route | Notes |
|---|---|---|
| Study (default `/`) | `GET /`, `POST /study/{id}/grade` | Flip card, grade buttons, progress, hide-mastered toggle, audio/image playback. Datastar patches the card area via SSE. |
| All caught up | same route, empty-queue state | |
| All cards + search | `GET /cards?q=` | Debounced Datastar search, grid/list, mastered badge, bulk select + delete, Add card button. |
| New card | `GET/POST /cards/new` | Front text + optional image/audio upload (front only), back text only, **live preview** (README: "Add card view to have a preview"). |
| Edit card | `GET/POST /cards/{id}`, `DELETE` | Same form as New card, plus delete. |
| Media | `GET /cards/{id}/media/...` | |

Navigation is a bottom tab bar on mobile (Study / Cards / +), a top bar on desktop.

## Increments (each leaves the app working, with tests)
1. **Foundation:** embed assets, store package, migrations, pragmas, `modernc` driver, config flags, layout template + Tailwind retro theme tokens. Test: store open/migrate.
2. **Cards CRUD (text only):** list, create, edit, delete, bulk delete. Test: store + handler tests (`httptest`).
3. **Search + All cards screen:** `LIKE`/FTS5 search with Datastar debounce. Test: search edge cases (empty, case, special chars).
4. **Media BLOBs:** upload (multipart), size and MIME validation (sniff, don't trust the client), media endpoint with ETag/Range, live preview in the form. Test: validation, round trip.
5. **SRS + Study mode:** `srs` package, queue query, flip/grade flow, progress, hide-mastered, All caught up. Test: table-driven SM-2 and queue tests.
6. **Responsive polish to the mockup:** mobile layouts, tab bar, retro fonts (bundle the fonts locally, no remote font runtime dependency), touch targets, safe-area insets.
7. **Pi demo build:** `GOOS=linux GOARCH=arm64` build, systemd unit, optional PWA manifest/icons/service worker (shell cache only), `Makefile` targets. Test on the real phone via the LAN.
8. **Wails spike (time-boxed):** wrap the handler in a Wails v3 desktop window first, then try the Android target. Document the result and blockers. This is not required for the demo.

## Testing and verification
- Unit: `srs`, store (in-memory SQLite), handlers (`httptest`).
- Manual: phone browser through the Pi, installed PWA, audio playback on iOS Safari (it needs a user gesture, and Range support matters).
- Run `go vet`, `go test ./...`, the tailwind build, and `gofmt` after each increment.

## Risks / open items
- Wails mobile is experimental. Mitigation: the Pi/PWA fallback and a time-boxed spike.
- BLOBs make the DB large. Mitigation: size limits, WAL, and a `VACUUM` note in the docs.
- Cleanup of the committed `tmp/` build artifacts and a `.gitignore` update (small housekeeping).
- README to be updated to match: no login, PWA/Pi instructions, backup = copy the DB.
