# Flashcards → Wails3 Android analysis

Analysis requested (m01317): assess PWA support as an intermediate step, and whether it would
hinder the end goal — a full APK via the Wails3 route (Android only, iOS not required).
Sources: the two Wails3 docs pages saved to `~/Downloads/` (verbatim extracts in `/tmp/wails_android.txt`
and `/tmp/wails_first_mobile.txt`), plus repo facts. Anything not doc-backed is marked **[VERIFY]**.

## Recommendation (short version)

Skip the PWA work entirely. Go straight to Wails3 Android. The parts of a PWA that matter
(manifest + service worker + an offline/IndexedDB data layer) either do not carry over to Wails
or actively conflict with it, and offline data via a browser is the whole reason a PWA would be
"worth it" — which we can't get without rewriting the data layer to run in the browser. The Wails
port itself is small because the app is already shaped right: a plain `net/http` server whose
frontend is static files + datastar.

## 1. What PWA support would actually take

The app is a Go server holding SQLite server-side. Data ≠ browser. A PWA would need:

| Piece | Effort | Carries over to Wails? |
| --- | --- | --- |
| `manifest.json` + icons + meta tags | ~1h, trivial | No (Wails has its own launcher/APK) |
| Service worker (cache the shell + assets) | ~2–4h | No — Wails serves assets in-process via `WebViewAssetLoader`; a SW over that scheme is useless **[VERIFY]** |
| **Offline data layer** — SQLite-in-browser (sql.js/OPFS CRUD + export/import persistence) or full IndexedDB store + sync | Days | **Worse than no** — client DB layer + sync becomes dead, confusing code under Wails |

The third row is the crux: SQLite cannot "work as a PWA" by itself. Our data lives in the Go
process. Either the app is online (server DB, daily-reset storage as on the domain) or we build a
browser-side copy of the storage layer + sync — a real rewrite, and all of it is throwaway under
Wails where data goes back to a real SQLite file on device.

Feels like: PWA = shim for "app-like on domain with no installable artifacts" — but the daily-reset
storage kills the only reason to prefer it.

## 2. Does the intermediate PWA hinder the Wails goal?

Not fatally, but only the manifest/icons piece is cheap; the meaningful PWA work is wasted or
conflicting. Table above. Verdict: **no PWA**. Deploy the plain web app to the domain for quick
demos (accepting the posed daily reset), and spend the hours on Wails.

Decision confirmed in review (user, Lavish annotation): the **entire PWA route is ditched** —
no manifest, no service worker, no browser-side data layer. Nothing PWA-related will be built.

## 3. Wails3 Android — facts from the docs (verbatim + link)

Sourced from the two saved pages:

- **Same `main.go` builds desktop and Android** — "You do not need to change your Go code.
  The same `main.go` builds for all targets." (first-mobile-app page)
- Architecture: Android `WebView` renders the frontend; assets are served **in-process**
  (`WebViewAssetLoader` — no localhost server, no open ports); Go is compiled as a C shared lib
  `libwails.so` (`GOOS=android` + NDK), loaded by a small Java host; platform behaviour is
  `//go:build android` files. (Android page)
- Toolchain: `ANDROID_HOME`/`ANDROID_SDK_ROOT`; Android SDK incl. `platform-tools`,
  `platforms;android-35`, `build-tools;35.0.0`, **NDK 26.3.11579264**; JDK (Ubuntu
  `sudo apt install openjdk-21-jdk`, `JAVA_HOME=/usr/lib/jvm/java-21-openjdk-amd64`); `wails3 doctor` verifies.
- Command set: `wails3 task android:run` (boots emulator, builds `libwails.so`, Gradle debug APK, installs, launches);
  `android:logs`, `android:package` (release APK), `android:package:fat` (universal APK arm64 + x86_64),
  `android:bundle:fat` (AAB needed for Play), `android:deploy-device` / `android:run:device`
  (`DEVICE_ID=<serial>` for a specific device — this is how we sideload to my phone).
- Debug builds: WebView inspectable at `chrome://inspect`.
- First build 5–10 min; incremental <1 min.

## 4. Port work on this repo (file by file)

What changes here:

1. **Entry point split** — `main.go` currently does `http.ListenAndServe` (main.go:22-42).
   Under Wails3 the asset/root handler is handed to the Wails app instead. Branch per platform
   via build tags: `//go:build android` file calls `wails3` runtime with the existing
   `internal/web` handler as the AssetServer; `//go:build !android` keeps `ListenAndServe` as-is.
   **[VERIFY]** exact handler registration API (AssetServer option) when scaffolding with
   `wails3 init -n`/template inside this project.
2. **DB path** — `main.go` uses `flashcards.db` relative to CWD (main.go:23). CWD is not
   writable on Android; the db must go into the app sandbox (files dir). **[VERIFY]** preferred
   accessor (OS user-cache-dir vs Wails path API) on device — this is the only genuine porting
   question for storage.
3. **SQLite driver (modernc.org/sqlite, pure Go)** — **this is a clean answer to "things would be
   simple with sqlite3 as it is?"**: yes, the driver is pure Go (no cgo), so it compiles into
   `libwails.so` unchanged under `CGO_ENABLED=1` and runs the same file-based SQLite as on
   desktop. Nothing about the schema or queries changes. Only the path moves per (2).
4. **SSE/datastar over the WebView asset scheme** — fragments/fetch already go through plain
   HTTP in-process; **[VERIFY]** that datastar's SSE (EventSource) is not buffered by
   `WebViewAssetLoader` on Android WebView; fallback = switch to datastar fetch/poll pattern
   (does not need SSE). This is the single biggest runtime risk — test with a hello-world
   fragment streaming first.
5. **Responsive/mobile CSS** — already largely retro/phone-friendly from the mockup work; add
   44px tap targets + safe-area insets per the docs snippets when it feels cramped.
6. **No PWA artefacts** anywhere (no manifest/SW needed for Wails).

What stays exactly as-is: `internal/store`, `internal/web`, templates, tailwind (handlers +
static files render identically in WebView). Not as-is and flagged in review: **camera/record JS
cannot be assumed to carry over** — in a WebView (unlike Chrome), `<input type=file capture>`
requiring the host-app file-chooser bridge (`WebChromeClient.onShowFileChooser`) to exist, and
`MediaRecorder`/getUserMedia requires the WebView host to grant permission requests
(`onPermissionRequest`). Both are Wails-host-level and go on the hello-probe checklist:
**[VERIFY]** camera capture via file chooser and `MediaRecorder` on the Android WebView in
phase 1; fallback = plain file pick without `capture` (and/or native camera via SAF if the
bridge proves unreliable). Clipboard works on Android per docs.

## 5. What "works" per the docs vs. what we use

| Docs (Android page) "what works" | We use it? |
| --- | --- |
| WebView + in-process assets | ✅ whole app |
| Service bindings, events both ways | only if we adopt wails runtime later; we don't need it initially |
| Message dialogs (`AlertDialog`) | not used yet |
| Open file / files dialogs (SAF) | n/a (we use `<input type=file>` capture for camera/record) |
| Clipboard | fine |
| Save-file / directory dialogs | ❌ error → write into app sandbox instead (only relevant if we add export) |
| Window geometry / menus / tray | no-ops, irrelevant (fullscreen app) |

## 6. Effort + order of work

Rough order once toolchain exists; the docs claim the same `main.go` works with **zero Go code
changes** for a desktop-first app — we have small deltas ((1)–(3) above), not zero.

1. Install toolchain: use the already-downloaded
   `~/Downloads/commandlinetools-linux-15859902_latest.zip` per docs steps (sdkmanager →
   platforms;android-35, build-tools;35.0.0, ndk;26.3.11579264, emulator, system image; avdmanager
   create; env vars; `apt install openjdk-21-jdk`). Then `wails3 doctor`.
2. **Phase 1 — hello-world Wails3 mobile probe (do not lean on `wails3 init`).** We already have
   an existing project; init output is a reference, not a scaffold for flashcards. Write a
   minimal `main.go` + one datastar fragment over SSE through the in-process asset scheme →
   decide SSE vs fetch/poll (guards item 4 above, the biggest runtime risk).
3. **Phase 2 — replicate what `wails3 init` generates into the flashcards repo.** Study the
   generated Taskfile android targets, `build/` directory and app config, and port only those
   pieces rather than scaffolding fresh. **[VERIFY]** exact generated file set (v3 alpha).
4. Port flashcards: entry split, db path, no other logic changes → `android:run` on emulator;
   `chrome://inspect` for debugging.
5. `wails3 task android:run:device` → sideload APK on my phone for real demos.
6. Later, for Play: `android:bundle:fat` + own keystore (`ANDROID_KEYSTORE_FILE` etc.), and bump
   `versionCode` per upload (docs note). AAB only needed for the actual Play upload; debug APK
   is fine for personal installs.

Effort estimate: toolchain 30–60 min; scratch hello-world 30–45 min; real port (items 1–5)
≈ half a day, dominated by the SSE-over-asset-server risk item and normal squeaky-wheel NDK
first-build time. **[VERIFY]** everything via wails3 doctor + build, not by trust in alpha docs —
mobile is explicitly experimental in Wails v3.

## 7. Answers to the exact questions asked

- **"Would creating an intermediate PWA hinder the final goal?"** — Not seriously (manifest+icons
  is an hour of throwaway), but the *substantive* PWA work (service worker, offline data layer)
  is either wasted or conflicts with the Wails in-process asset model. Skip it.
- **"Things would be simple to just use the sqlite3 database stuff as it is?"** — Yes, genuinely:
  pure-Go driver, file-based, compiled into the same shared lib; only the db *path* moves into
  the app sandbox.
- **"PWA install on the domain useless unless sqlite3 works as a PWA?"** — Correct. A PWA install
  on the domain today caches the shell but not the data; with daily-reset server storage the
  install buys nothing useful.
- **Decision requested and confirmed (m01317): drop PWA focus, do the Wails APK route for
  Android only.**
