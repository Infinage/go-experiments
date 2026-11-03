# Flashcards

A simple mobile responsive web application with a clean gamified UI supporting multiple users.

## Tech stack:
- Backend: Golang 1.27
- Frontend framework: Datastar (go-datastar SDK)
- Database: Sqlite3 (modernc/sqlite)
- Minimal external dependencies

## Manual testing — dummy users

Run the server with `FLASHCARDS_SEED_DEMO=1` to fill an **empty** database
with the accounts below (no-op if users already exist). Without the variable,
no seeding happens. The dev `flashcards.db` is not tracked in git.

| Username | Password | Sample data |
|---|---|---|
| `demo` | `demo1234` | 4 cards across geo / math / chem tags |
| `reviewer` | `reviewer1` | 1 card (chem) |
| `test` | `test1234` | 2 cards (vegetables) |
