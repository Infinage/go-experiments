# Wails3 framework — notes for flashcards mobile POC

> Source note: mirrors `IRGO_FRAMEWORK_NOTES.md` for symmetry. Facts are from your two saved doc
> dumps (`~/Downloads/5_Android · Wails.html`, `~/Downloads/5_Your First Mobile App · Wails.html`)
> plus the base v3.wails.io architecture reading from the original analysis. Deep-link API pages
> (`/guides/managed-pages/android-api-reference`) are **Cloudflare-gated — not fetched**; anything
> from those is marked **[UNVERIFIED — gated page]**. Treat all as orientation, not verbatim docs.

## Identity

- Name: **Wails** — a Go desktop/mobile framework in active development; v3 is the current major
  version with a rewritten architecture from v2.
- Docs: `https://v3.wails.io` (saved pages: "Android" guide + "Your First Mobile App" guide).
- Repo: `https://github.com/wailsapp/wails`.
- License: MIT.
- Positioning: build apps for desktop **and mobile** from a Go core + a webview frontend;
  effectively "what if Electron was native and Go-only" — but the important thing for us is
  that **mobile** is first-class in v3, not an afterthought.

## Architecture (verified from saved pages)

Desktop model (also the default mobile mode's consistency anchor):

- A native **webview** window (`WKWebView` on macOS, `WebView2` on Windows, GTK WebKit on Linux)
  renders the frontend.
- The Go process serves **assets in-process** rather than through raw `http.ListenAndServe`;
  the app configures an asset server (FS handler) that the webview consumes via its custom
  scheme or an internal handler (`WebViewAssetLoader` on Android).
- Go API surface: `wails3.New(app.Options{...})`, app.Run(), window controls, dialogs,
  clipboard, menus, tray, events — same codebase compiles to desktop and mobile.

Mobile model (Android, verified):

- **Same `main.go` builds desktop + Android.** "You do not need to change your Go code.
  The same `main.go` builds for all targets." *(verbatim, from Your First Mobile App)*
- Architecture: Android `WebView` renders the frontend; assets are served **in-process**
  via `WebViewAssetLoader` (no localhost server, no open ports).
- Go is compiled as a **C shared library `libwails.so`** (`GOOS=android` + Android NDK),
  loaded by a small Java host class. Platform-specific behaviour is isolated to
  `//go:build android` files.
- Commands (all under `wails3 task ...` and verified from the saved pages):
  - `android:run` — boots emulator, builds `libwails.so`, Gradle debug APK, installs, launches.
  - `android:logs` — stream logcat for the app.
  - `android:package` — release APK.
  - `android:package:fat` — universal APK (arm64 + x86_64 debug/release).
  - `android:bundle:fat` — **AAB**, the artifact Play requires for upload.
  - `android:deploy-device` / `android:run:device` — install/run on a physical device;
    `DEVICE_ID=<serial>` targets a specific one (how we'd sideload to my phone).
- Debug: WebView inspectable at `chrome://inspect` in dev builds.
- First build 5–10 min; incremental <1 min.

## templ? — No. It's `html/template`-friendly (or any templating you bring)

Wails3 does **not** require templ. The frontend is whatever you serve; we'd keep our existing
`html/template` templates + tailwind + datastar exactly as-is. **No view-layer rewrite.**
This is the single biggest divergence from irgo (which is templ-only).

## Prerequisites (verified)

- `ANDROID_HOME` / `ANDROID_SDK_ROOT` to the SDK dir.
- Android SDK: `platform-tools`, `platforms;android-35`, `build-tools;35.0.0`,
  **NDK 26.3.11579264**, plus emulator + a system image (installed via `sdkmanager`; the
  `commandlinetools-linux-15859902_latest.zip` in `~/Downloads` is the intended starting point).
- JDK 21 (Ubuntu: `sudo apt install openjdk-21-jdk`), `JAVA_HOME=/usr/lib/jvm/java-21-openjdk-amd64`.
- `wails3 doctor` verifies the toolchain end-to-end.
- We already have `commandlinetools-linux-15859902_latest.zip` in `~/Downloads` — the SDK
  install is future work, not blocked on anything we don't have.

## Ecosystem reality check

- Desktop mature, mobile **alpha** but builds on v2 heritage and a much larger team/repo than
  irgo; docs are more thorough (both saved pages are tutorial-dense).
- Same trade-off as irgo: webview-based, so no native transitions/gestures/notifications;
  camera/bluetooth route through the webview (file inputs with `capture=` work for the camera
  use-case).
- Storage stays Go-side (SQLite via modernc, file-based) — on-device in the app sandbox path.
  No PWA artifacts needed or wanted here.

## Where that leaves the POC

- Mapping to flashcards is the **smallest delta of any option**: nothing changes about
  templates, tailwind, datastar, or the Go stack. Entry-point untouched (`http.ListenAndServe`
  in dev, Wails asset handler in packaged mode).
- Risks concentrated in ONE row of the handover matrix: **SSE buffering through the in-process
  asset server** — that's the only live doubt. Hypothesis to test in the hello-POC: does a
  datastar SSE flush propagate through `WebViewAssetLoader` on an Android WebView?
- Fallback (already planned): swap the app to `http.ListenAndServe` on a loopback port
  (dev-sever mode we already run), letting the WebView talk to `http://127.0.0.1:<port>` like a
  normal browser page — same code we run today, sideloaded into an APK shell.
- Toolchain install is the slow step (NDK ~400 MB + JDK); we already have the CLI zip.

## Suggested next step

Same gate as irgo's: run the **hello-SSE 30-min experiment** (card flip + live `Card x of y`
+ one datastar SSE fragment + one SQLite write) through `wails3 task android:run` and measure
the POC's matrix row 1 (SSE through asset server) before committing to this stack for the
`HANDOVER_POC_WAILS_VS_IRGO.md` build-out.
