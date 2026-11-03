# E-04: Demo Seed Data — Implementation Record

**Status:** Complete
**Related:** Manual-testing infrastructure; no user-facing story
**Date:** 2026-11-03

## Problem

Dummy test accounts (`demo`, `reviewer`, `test`) were originally created by
driving the live app against the local `flashcards.db`. That file is not
tracked in git, so the accounts existed only in one developer's environment.
Fresh checkouts got an empty database with no way to log in.

## What was done

- **`Store.SeedDemoData(ctx)`** (`src/pkg/repo.go`): creates the demo accounts
  (`demo`/`demo1234`, `reviewer`/`reviewer1`, `test`/`test1234`) and their
  sample cards through the **same `hashPassword` → `CreateUser` → `CreateCard`
  code paths as the app**, so credentials work identically in every
  environment (salt differs per seed; that doesn't matter for login).
- **Idempotent:** it is a no-op if any user already exists (checked via
  `SELECT COUNT(*) FROM users`), so restarts never duplicate users or cards.
- **Env gate:** `main.go` calls it only when `FLASHCARDS_SEED_DEMO=1` and logs
  one line when seeding occurred. Explicit opt-in per environment; production
  would simply not set the variable.
- `.gitignore` added (`flashcards.db`, `tmp/`) so the dev DB and build
  artifacts can't be committed accidentally.
- README's "Manual testing — dummy users" section now documents the env var.

## Verification

- New unit test `TestSeedDemoData` (temp-file DB): seeds once → 3 users, card
  counts 4/1/2, `checkPassword` verifies `demo1234` and rejects a wrong
  password; second seed call is a no-op.
- Live smoke test on a fresh temp dir:
  - `FLASHCARDS_SEED_DEMO=1` → seed log line; all three accounts log in via
    the real `POST /login`; `demo` lists 4 cards.
  - Restart with the env var on the populated DB → no seed log, no
    duplicates.

## Notes

- Seeded card content intentionally mirrors the accounts' original dev data
  (including `test`'s two vegetable cards) so the README table stays accurate.
- The default `FLASHCARDS_SEED_DEMO` state is off: a plain `go run .` on an
  empty DB starts with no users, as before.

## Behavior matrix

What a run with `FLASHCARDS_SEED_DEMO=1` does, by database state:

| DB state | Result |
|---|---|
| No DB file (or no `users` table) | Schema created; all 3 demo users + cards seeded |
| Existing DB, `users` empty | Demo accounts + cards seeded |
| Existing DB with any user | **No-op** — nothing inserted, no seed log line |
| Restart with the env var on an already-seeded DB | No-op — no duplicates |

The guard is `SELECT COUNT(*) FROM users` — more than zero users means skip
entirely, so a DB where someone has already signed up is never modified.

**Known limitation (accepted):** seeding is **not transactional**. If the
process dies mid-seed (after `demo` but before `reviewer`, say), the users
table is no longer empty and a re-run skips seeding — the DB is left with a
partial seed that must be fixed by hand. Accepted trade-off for a dev-only,
explicitly opted-in flag.
