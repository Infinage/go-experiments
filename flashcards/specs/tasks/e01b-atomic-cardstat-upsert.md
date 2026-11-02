# E-01b: Atomic `UpsertCardStat` + UTC due-times — Implementation Record

**Status:** Complete
**Related:** E-01 (`specs/tasks/e01-handler-reorganization.md`)
**Date:** 2026-11-02

## Problem

`UpsertCardStat` (`src/pkg/repo.go`) was a read-modify-write across three
statements on a pooled `*sql.DB`:

1. `SELECT current_interval_hours`
2. compute `nextInterval` in Go
3. `INSERT ... ON CONFLICT DO UPDATE`

Two concurrent answers for the same card could both read the same interval and
the second write would silently discard the first (lost update): e.g. tab A
"correct" and tab B "wrong" nearly simultaneously → the reset to 1h could be
overwritten by the stale doubling to 2h. Serialized within one session by
`handleAnswer`'s `sess.mu`, but real across two sessions of the same user.

A second latent bug: `next_review` was written from Go's `time.Now()` (local
time, driver-serialized format) but compared against SQLite's
`CURRENT_TIMESTAMP`/`datetime('now')` (UTC, canonical text) in
`TagCounts`/`DueCardsByTag` — skewing due times by the server's UTC offset.

## Fix

### 1. Single-statement atomic upsert

The whole read-modify-write is now one statement. Inside `DO UPDATE`, the bare
column name refers to the pre-update row, so doubling is based on the value as
of the write itself; interleaving can no longer lose an update.

```sql
INSERT INTO stats (user, card, next_review, current_interval_hours)
VALUES (?, ?,
    datetime('now', '+' || (CASE WHEN ? THEN 2 ELSE 1 END) || ' hours'),
    CASE WHEN ? THEN 2 ELSE 1 END)
ON CONFLICT(user, card) DO UPDATE SET
    current_interval_hours = CASE WHEN ? THEN current_interval_hours * 2 ELSE 1 END,
    next_review = datetime('now', '+' || (CASE WHEN ? THEN current_interval_hours * 2 ELSE 1 END) || ' hours')
```

Behavior preserved: a first-ever correct answer still lands on interval 2
(insert path); wrong answers reset to 1.

### 2. SQLite-side UTC timestamps (decision)

`next_review` is now computed by `datetime('now', ...)` inside the statement —
UTC, same clock and text format as every due-query comparison.

**Why this option** (vs. the alternatives considered):
- *Go-side `time.Now().UTC()`* fixes the offset but still relies on the
  driver's `time.Time` → text serialization matching what SQLite's date
  functions parse — a cross-format contract that can drift silently.
- *`datetime('now','localtime')`* would require changing every read-side
  comparison too and leaves offset-less timestamps that break if the server
  timezone ever changes.
- *SQLite-side UTC* means one engine owns both the write and every read — no
  cross-format contract at all, and UTC keeps data stable across server
  timezone changes. (Comment added in `repo.go`.)

Note: the modernc driver re-normalizes DATETIME-declared columns to RFC3339
when *scanning into Go*; the stored text is SQLite-canonical and `julianday`
parses it as UTC (verified by raw-SQL probes).

### 3. `busy_timeout` on the connection DSN

Under concurrent writes, pooled SQLite connections hit `SQLITE_BUSY`.
`NewStore` now opens with `?_fk=1&_pragma=busy_timeout(5000)` so writers queue
on the lock instead of failing.

## Verification

`gofmt` clean; `go build`/`go vet` pass; `go test ./... -count=1 -race` passes.

Test-suite speed note: the suite initially took ~5s under `-race` because
`TestPasswordHashRoundTrip` ran PBKDF2 at the production work factor (600k
rounds × 3 calls, ~4s under race instrumentation). `pbkdf2Iter` is now a var
(default 600_000, unchanged in prod) that `TestMain` lowers to 1000 for tests;
suite is ~1.1s with `-race`, ~0.3s without.

New tests in `src/pkg/repo_test.go` (temp-file DB per test, user seeded first
for the `cards.user` FK):
- `TestUpsertCardStatIntervalProgression` — 1st correct → 2, 2nd correct → 4,
  wrong → 1.
- `TestUpsertCardStatConcurrentNoLostUpdates` — 6 goroutines answering
  correct concurrently on the same card; interval must be 2⁶ = 64 (any smaller
  value means a lost update). Passes under `-race` with `busy_timeout`.
- `TestUpsertCardStatNextReviewMatchesSQLiteClock` — after a correct answer,
  `(julianday(next_review) - julianday('now')) * 24` ≈ 2h (proves the stored
  datetime is parsed as UTC by the same engine the due queries use), and the
  card is not yet due.

Debugging notes for posterity:
- First failing run of the clock test was **test bug, not code bug**:
  `julianday()` returns **days**, so a correct 2-hour delta reads as `0.0833`;
  the test now multiplies by 24 (confirmed via raw-SQL probe: delta × 1440 =
  119-120 minutes).
- Concurrent test initially failed with `SQLITE_BUSY` → led to the
  `busy_timeout` DSN fix above.
- Test helpers initially hit `FOREIGN KEY constraint failed` → tests must
  create the user before the card.

## Not done / follow-ups

- Existing dev DBs (`flashcards.db`) may contain old-format `next_review`
  values from the Go-serialized era; fresh dev DBs are fine. If needed, a
  one-off `UPDATE stats SET next_review = strftime('%Y-%m-%d %H:%M:%S',
  next_review)` migration could normalize them.
