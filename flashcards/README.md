# Flashcards App

A retro-styled, single-user flashcard app with SM-2 spaced repetition.

- Responsive design (mobile bottom tabs, desktop sidebar), no login
- Cards support text, an image and audio on the **front** (back is text
  only); media is stored as BLOBs in SQLite
- Study mode (default) with flip, grade (Forgot/Hard/Medium/Easy), shuffle,
  paging, category filter and "hide mastered"
- A *pass* = the cards you see between "All caught up" states: grading a
  card slots it into the future (it leaves the pass); Forgot keeps it and
  requeues it once at the end; a forgotten-twice card is dropped for the
  pass. "Review anyway" starts a flash pass over every card, due or not —
  the counter shows "Card x of y", where y = remaining cards + already
  graded this pass, and therefore never shrinks from paging and always
  terminates
- All cards view with search, media/category/status filters, bulk delete
- Card media can be captured in-app: live camera modal (webcam on desktop,
  camera on Android) snaps straight into the form, and a mic recorder
  records the audio front; on Android the WebView routes file choosers and
  camera/mic permissions through the native bridge
- Per-category "decks" progress; reset progress per category or all
- Backup/export = copy the SQLite file

## Tech Stack

Go, tailwind (v4 CLI), datastar (SSE), SQLite (`modernc.org/sqlite`,
pure Go), html/template. On Android the app runs inside a [Wails v3](https://github.com/wailsapp/wails) WebView (see the mobile section).

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

## Mobile (Android, working)

The app runs on Android as a [Wails v3](https://github.com/wailsapp/wails)
WebView wrapping the same HTTP handler and SQLite store. The web build is
unchanged (`make build`); Android uses these extra files:

    build/                               Android gradle tree (checked in)
    main.go (android branch), main_android.go    WebView entrypoint + c-shared init
    internal/web/samplecards.go          shared sample deck (also feeds "Add sample cards")
    cmd/seeddb/                          writes a pre-seeded device database
    Makefile apk targets                 the whole Android build, plain make + go + gradle

### Requirements

- Go 21+, JDK 21+, `adb`
- Android SDK with **NDK 26.3.11579264** (pinned in the `Makefile` as
  `NDK_VERSION`) and platform 34
- No build runner, no Wails install: every build step is plain shell/Go
  (gradle wrapper + `go build -buildmode=c-shared`), wired up in the
  `Makefile`'s `apk` targets (go-task was removed — same recipes).

A toolchain mismatch fails loudly **before** the build starts: `make apk`
first checks for the java executable and the exact pinned NDK directory.

### Build & install (Wails-free)

    make apk              # builds bin/flashcards.apk (arm64, debug-signed)
    adb install -r bin/flashcards.apk

Or with a device attached:

    make apk-run          # install -r + launch (keeps app data between runs)
    make apk-push-db      # replace the device db with a sample deck  (SRC=<path>)

Run `make test` first: the desktop web build and the Android build compile
the same `internal/web` code. To verify the Android split without the full
APK, run the cgo vet check the `Makefile` build uses (needs the NDK clang
for cross-compilation):

    NDK=$HOME/Android/Sdk/ndk/26.3.11579264; CGO_ENABLED=1 GOOS=android GOARCH=arm64 \
      CC=$NDK/toolchains/llvm/prebuilt/linux-x86_64/bin/aarch64-linux-android21-clang \
      go vet -tags android ./...

### Mobile build provenance (fair warning for future maintainers)

The files under `build/` are **not** the pristine output of `wails init`
(v3, alpha2.103, run once into a scratch project). They were generated
there and then hand-edited into the current, wails-CLI-free form — the
edits being:

- `com.wails.app` → `com.infinage.flashcards` in `gradle` (`applicationId`,
  NOT `namespace` — see below), `app_name` "Wails App" → "Flashcards",
  and the `am start` FQCN (now the `Makefile`'s `apk-run` target).
- The Java sources stay in package `com.wails.app` on purpose: the Wails
  Go module hardcodes its JNI exports (`Java_com_wails_app_WailsBridge_*`,
  see `pkg/application/application_android.go` in the pinned module), so the
  bridge classes cannot be renamed without forking the module.
- All desktop-only wails-init targets (darwin/windows/linux/docker) and
  `build/config.yml`/icons are removed; the only orchestration left is the
  `Makefile`'s `apk` targets (the wails-init Taskfiles were deleted).
- The `wails3 android overlay:gen` step is replaced by the committed
  `main_android.go` (its generated equivalent), removing the last
  wails-CLI dependency from the build. Regenerating via wails is never
  necessary; to re-diff against a fresh `wails init`, run it in a scratch
  directory (same alpha) instead of touching this tree.
- `local.properties` (gradle SDK path) and all build outputs are
  gitignored; the gradle wrapper jar is committed, so any clone with the
  pinned toolchain builds reproducibly from scratch.
- Launcher icons (`mipmap-*ic_launcher*.png`) are the app favicon
  (`assets/static/icon.svg`) rasterised to all five densities; the stock
  Wails "W" is gone.

### Device debugging

Go `log` output goes to the app's own cache dir (`adb` cannot see Go
stderr on Android):

    make apk-go-log       # adb shell run-as com.infinage.flashcards cat cache/flashcards-go.log
    make apk-logs         # adb logcat filtered to Wails output

`make apk-push-db` clears any stale WAL/shm journals first — replaying an
old WAL over a replaced main db would resurrect pre-seed data.

## Layout

    internal/store    SQLite access + migrations (PRAGMA user_version)
    internal/srs      SM-2 scheduling (pure, unit tested)
    internal/web      handlers, Datastar SSE, templates (embedded)
    assets/           templates, tailwind, datastar, fonts, PWA files
