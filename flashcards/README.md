# Flashcards App

A retro-styled, single-user flashcard app with SM-2 spaced repetition.

- Responsive design (mobile bottom tabs, desktop sidebar), no login
- Cards support text, an image and audio on the **front** (back is text
  only); media is stored as BLOBs in SQLite
- Study mode (default) with flip, grade (Forgot/Hard/Medium/Easy), shuffle,
  paging, category filter and "hide mastered"
- All cards view with search, media/category filters, bulk delete
- Per-category "decks" progress; reset progress per category or all
- Backup/export = copy the SQLite file

## Tech Stack

Go, tailwind (v4 CLI), datastar (SSE), SQLite (`modernc.org/sqlite`,
pure Go), html/template. Wails v3 is planned for desktop/mobile wrapping.

## Development

    make dev        # tailwind --watch + air (hot rebuild)

Build tools (installed separately): Go 1.27+, `tailwindcss` CLI v4,
`air`, `djlint` (template formatting).

The browser bundle `assets/js/datastar.js` is Datastar **v1.0.4**
(official bundle, no CDN at runtime); the Go SDK is
`starfederation/datastar-go` **v1.2.2** — wire-format compatible.
Poppins is bundled locally under `assets/fonts` (SIL OFL 1.1, see
`assets/fonts/OFL.txt`).

## Running

    make build && ./bin/flashcards          # serves on :8080
    ./bin/flashcards -addr :9000 -db x.db   # flags

Data lives in `flashcards.db` (SQLite, WAL) next to the binary.

## Deploy to a Raspberry Pi (demo)

    make build-pi                       # static linux/arm64 binary
    scp bin/flashcards-arm64 pi@<host>:~/
    ssh pi@<host> './flashcards-arm64 -addr :8080'

Then open `http://<pi>:8080` on the phone and "Add to Home Screen" —
a minimal PWA (manifest + service worker) provides the app icon and an
offline shell. For a persistent service on the Pi:

```ini
# /etc/systemd/system/flashcards.service
[Unit]
Description=Retro Flashcards
After=network.target

[Service]
WorkingDirectory=/home/pi
ExecStart=/home/pi/flashcards-arm64 -addr :8080
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

`sudo systemctl enable --now flashcards`

## Mobile (planned)

The HTTP handler is a plain `http.Handler`, so a Wails v3 wrapper can
mount it unchanged. Wails v3 is stable on desktop; Android/iOS remain
experimental, so mobile wrapping is a time-boxed spike, not a
dependency of the Pi demo.

## Layout

    internal/store    SQLite access + migrations (PRAGMA user_version)
    internal/srs      SM-2 scheduling (pure, unit tested)
    internal/web      handlers, Datastar SSE, templates (embedded)
    assets/           templates, tailwind, datastar, fonts, PWA files
