# E-03: Theming Foundation + Card Edit/Delete — Implementation Record

**Status:** Complete
**Stories:** US-E03, US-B04, US-B05, US-B06 (new), plus static-asset gap from E-02 scope
**Date:** 2026-11-02

## What was done

### 1. Static asset serving (E-02 open item)

- Templates referenced `/assets/css/index.css` and `/assets/js/datastar.js`
  but nothing served them. New second embed in `main.go`
  (`//go:embed assets/css assets/js`), rooted via `fs.Sub(..., "assets")`,
  passed to `NewServer` as `staticFS`.
- `server.go` serves it at `GET /assets/` via `http.FileServer(http.FS(...))`
  with `StripPrefix`. Templates under `assets/html` are deliberately NOT in
  the public FS (embed excludes them; verified 404).

### 2. CSS theming (US-E03)

- `assets/css/input.css` is now the single home for custom theme tokens and
  all `@keyframes`, exposed as `animate-<name>` utilities via Tailwind v4
  `@theme`. First token: `--animate-confirm-pulse` (delete-confirm feedback).
- **Decision (user):** Tailwind's built-in palette covers the constitution's
  color system; custom color tokens get added only when Tailwind lacks them.
  So E03's "no hardcoded colors" is satisfied via the built-in palette +
  centralized animations, not via renamed semantic utilities.

### 3. Card list, edit, delete, bulk delete (US-B04/B05/B06)

- New repo method `GetCardsByUser` — all of a user's cards with tags. The
  existing `GetAllCards` (unused, and its user-scoped branch a weaker
  duplicate of `DueCardsByTag`) was **removed** rather than kept as dead code.
- Routes (all `authed`, owner-checked against `GetCardsByUser`):
  - `GET /cards` — "Your cards" list (entry button added on home; per-row
    checkbox, tags, Edit, two-step inline Delete confirm).
  - `GET /cards/{id}/edit` / `POST /cards/{id}/edit` — edit form prefilled by
    seeding `data-signals` with server-side JSON-encoded current values;
    prompt/answer required validation mirrors creation.
  - `DELETE /cards/{id}` — single delete via `Store.DeleteCards`
    (card + stats + tags in one tx). Confirm is client-side: the Delete
    button reveals "Really delete / No" via boolean signals — no round-trip.
  - `DELETE /cards` — bulk delete from checkbox signals (`pick_<id>`
    booleans → `parseBulkIDs`); `ownedIDs` silently drops ids not owned by
    the caller, so crafted signals can't delete other users' cards.
- **Verb decision:** single and bulk deletes use the `DELETE` method, not
  `POST /cards/{id}/delete`. The vendored Datastar client supports `@delete`,
  and the SDK's `ReadSignals` reads DELETE signals from the `?datastar=`
  querystring natively — verified live.
- The card create/edit forms share one `cardForm` struct (json-tagged
  `prompt`/`answer`/`tags`) for both reading signals and seeding
  `data-signals`, so the signal names are defined in exactly one place.
- New story **US-B06: Bulk Delete Cards** added to `card-management.md`
  (user-requested addition).

## Bugs found & fixed during implementation

- **Embed path prefix:** `go:embed assets/css` keeps the `assets/` directory
  in the FS root. Passing the embed FS raw made `/assets/css/…` resolve one
  level too deep (404s; `/assets/` listing showed an `assets/` entry).
  Fixed by `fs.Sub(staticFS, "assets")` in `main.go` before passing it in.

## Verification

- `gofmt -l` clean; `go build ./...` / `go vet ./...` OK;
  `go test ./... -count=1` green.
- New unit tests: `TestGetCardsByUser` (ownership scoping, tag join,
  update/delete round-trip), `TestParseBulkIDs` (truthy-only, ignores
  malformed ids and foreign keys), `TestOwnedIDs` (foreign id dropped).
- Live smoke test (fresh temp DB):
  - `GET /assets/css/index.css` → 200 text/css; `/assets/js/datastar.js` →
    200; `/assets/html/app.html` → 404 (no template leak).
  - Create 3 cards → list shows all. Edit card 1 (prompt, answer, tags) →
    "Card updated." with new content/tags.
  - Single delete → "Deleted card.", remaining list correct.
  - Bulk delete with one foreign id injected → "Deleted 2 cards." (foreign
    id silently dropped).
  - Second user deleting the first user's card → "That card doesn't exist."

## Parked / follow-ups

- B05's original acceptance said the home dashboard re-patches after delete;
  the cards list patches instead (entry point moved to "Your cards").
  Constitution/story wording may need a small amendment.
- Cards list has no pagination or tag filter — fine at current scale.
- Edit form fields are seeded via signals, so a no-JS browser shows empty
  textareas (the whole app is Datastar-first; acceptable).
- Bulk selection has no "select all / none" toggle; delete button is enabled
  even with nothing selected (server replies "Select at least one card").
