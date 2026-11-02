# E-01c: go-datastar SDK Adoption + Code-Review Pass — Implementation Record

**Status:** Complete
**Related:** E-01 (`e01-handler-reorganization.md`), E-01b (`e01b-atomic-cardstat-upsert.md`)
**Date:** 2026-11-02

## What was done

Code-review round over the files created in E-01. Each item below was raised
in review and addressed:

### 1. go-datastar SDK replaces hand-rolled SSE helpers

- **Tried:** Initial import path `github.com/starfederation/datastar/sdk/go`
  (from the skill's example) — **failed**: the published `datastar` module
  (v1.0.4) does not contain that package. Found the correct module in the Go
  module cache: `github.com/starfederation/datastar-go/datastar` (v1.2.2),
  confirmed via its `go.mod` module line.
- `patchMain` previously assembled SSE frames manually (`event:`,
  `data: elements` line-prefixing, manual `Flush`). Now:
  `datastar.NewSSE(w, r)` + `sse.PatchElements(html, datastar.WithSelector("#main"))`.
  The SDK handles headers, event framing, and flushing.
- `readSignals` now delegates to `datastar.ReadSignals(r, dst)` (still wrapped
  with `http.MaxBytesReader` for the 8MB cap).
- **Signature ripple:** `patchMain` needed `r` for `NewSSE(w, r)`, so all ~20
  call sites gained `r`; `renderReview`'s signature also grew `r`.
- **Decision — selector vs. fragment-root:** the SDK patch targets `#main`
  explicitly via `WithSelector`, replacing the earlier convention where every
  fragment's root element had to be `<section id="main">`.
- Added dependency via `go get` + `go mod tidy` (pulls in compression deps:
  httpcompression/brotli/zstd/gzip/zlib, bytebufferpool).

### 2. Handler call-site comments

Every handler now has a one-line comment stating where it is called from,
e.g. `handleReviewStart` — "called when the user clicks a tag on the home
page to begin studying (POST /review/start?tag=...)". Covered: handleIndex,
handleLogin/Signup/Logout, handleHome, handleNewForm, handleCreateCard,
handleImportForm, handleImport, handleReviewStart, handleReveal, handleAnswer.

### 3. `review` struct field comments

Documented the review-session lifecycle and each field: `tag` (filter tag),
`queue` (due cards in order), `idx` (index of the next card to review; equals
`len(queue)` when done — confirmed the reviewer's assumption), `right`/`wrong`
tallies. Struct doc notes it is tied to a single review: starts when the user
picks a tag, ends when the queue is exhausted (summary shown).

### 4. Tag separator documentation

`parseTags` now documents that **form input splits on `,` and CSV import
splits on `;`** — previously the `;` convention existed only implicitly in
`parseCSV`'s call.

### 5. `\ufeff` BOM handling

Explained in a comment: `\ufeff` is the UTF-8 BOM many editors prepend to
files saved as "UTF-8 with BOM"; it must be stripped from the first header
cell. The one-liner `strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\ufeff")))`
was split into three statements for readability.

### 6. CSV parse: fail-fast on first error (behavior change)

- `parseCSV` previously accumulated up to 10 errors before stopping
  (`for row := 2; len(errs) < 10; row++`).
- **Decision:** break at the first row error instead — the UI shows one
  actionable message; continuing to scan rows the user must fix anyway adds
  no value. Now `for row := 2; ; row++` with `break` on either error kind
  (CSV syntax error or missing prompt/answer).
- Tests updated to match: `{"missing field in row", ..., 1, 1}` (valid rows
  before the bad row are still kept).

### 7. Password salt: per-user random salt documented as safe

Reviewer asked whether storing the salt inside the password string is safe,
or whether a fixed salt would do. **Decision: per-user random salt stored
alongside the hash is the standard, correct design** — documented on
`hashPassword`:
- The salt is not a secret; it defeats precomputed rainbow-table attacks.
- Unique salts mean two users with the same password get different hashes.
- A fixed/global salt would be *less* safe (identical passwords → identical
  hashes → single rainbow-table entry attacks every user).
- An attacker with the stored salt still must brute-force each password.

### 8. "That username is taken" no longer masks DB outages

- `handleSignup` previously showed "That username is taken" for *any*
  `CreateUser` error — misleading when the DB is down.
- **Tried:** matching on `err.Error()` containing `"constraint failed"` —
  **refined**: that string also covers foreign-key violations, so it now
  matches `"UNIQUE constraint failed"` specifically. Only genuine duplicate
  usernames get the friendly message; other DB failures go to `fail()` → 500.

### 9. One-off template structs → `map[string]any` (go-conventions)

The project's go conventions state: "For one-off HTML template data, prefer
`map[string]any` over custom structs." Six view-data structs each used by
exactly one template were converted; templates keep their existing field
references (`{{.Error}}`, `{{.Flash}}`, …) since `html/template` performs
identical case-sensitive lookups on map keys.

| Removed struct | Template | Replacement |
|---|---|---|
| `loginData{Error}` | `login` | `map[string]any{"Error": …}` / `map[string]any{}` |
| `formData{Flash}` | `cardform` | `map[string]any{"Flash": …}` |
| `importData{Msg, Errs}` | `import` | `map[string]any{"Msg": …, "Errs": …}` |
| `homeData{User, Flash, Tags}` + `tagView{TagCount, Q}` | `home` | `map[string]any` with `Tags` as `[]map[string]any{Name, Due, Total, Q}` |
| `reviewData{Tag, Idx, Pos, Total, Prompt, Answer, Revealed}` | `review` | `map[string]any{…}` |
| `summaryData{Tag, Right, Wrong}` | `summary` | `map[string]any{…}` |

- `buildHome` now returns `(map[string]any, error)` and builds the tag list as
  `[]map[string]any` instead of `[]tagView`.
- Kept as structs (not template data): `authSignals` (typed JSON decode target
  for `readSignals`) and `review` (long-lived review-session state).
- `gofmt` clean-up was needed on `handlers_cards.go` after the sed edits.

## Verification

- `gofmt -l` clean; `go build ./...` OK; `go vet ./...` OK;
  `go test ./... -count=1` OK (`src/pkg` passes, ~0.02s).
- SDK wiring verified by build + existing unit tests (parseCSV fail-fast
  expectations, parseTags, password round-trip all still green).
- Live end-to-end SSE behavior was exercised earlier in E-01's smoke test
  (login/home/review/import patches); the SDK emits the same
  `datastar-patch-elements` frames, with the patch now selector-targeted.

## Notes / open items

- The SDK's `NewSSE` takes `(w, r)` — keep both in any future helper that
  wraps `patchMain`.
- `Datastar-Request` CSRF middleware and full-page `GET /` rendering are
  unchanged; only SSE emission moved to the SDK.
- No static handler for `/assets/css/index.css` / `/assets/js/datastar.js`
  yet (carried over from E-01 open items; E-02 scope).
