# mobile-wails-poc

Validate that a Go backend serving HTML over **datastar/SSE** runs correctly inside a
**Wails v3 Android** webview, backed by a real **SQLite** database. Throwaway code is
expected. Scope is deliberately minimal — this is a feasibility probe, not an app.

## Background facts (from Wails v3 docs)

- Wails v3, MIT, repo `github.com/wailsapp/wails`; mobile target is considered alpha.
- The same `main.go` builds for desktop and Android ("You do not need to change your Go code.
  The same `main.go` builds for all targets" — Your First Mobile App guide).
- On Android the WebView renders the frontend; assets are served **in-process** via
  `WebViewAssetLoader` (no localhost server, no open ports).
- Go compiles to a C shared library (`GOOS=android` + NDK) loaded by a small Java host class.
- No view-layer/templating requirement — plain `html/template` works as-is.
- Commands: `wails3 task android:run` (build + install + launch on emulator),
  `android:logs`, `android:package`, `android:package:fat`, `android:bundle:fat`,
  `android:run:device` (+ `DEVICE_ID=<serial>` for a physical device).
- Debug: WebView inspectable at `chrome://inspect` in debug builds.
- Toolchain: `ANDROID_HOME`/`ANDROID_SDK_ROOT`; SDK components `platform-tools`,
  `platforms;android-35`, `build-tools;35.0.0`, NDK 26.3.11579264, emulator + system image;
  JDK 21. `wails3 doctor` verifies the toolchain.

## Test goals (in priority order)

1. **Does datastar/SSE stream unbuffered through the in-process asset server on Android?**
   This is the primary risk. If it buffers or fails, the fall-back is a plain
   `http.ListenAndServe` on loopback (dev-server style) inside the packaged app.
2. **Does SQLite work end-to-end?** Real read of one record from one real table (no
   hello-world placeholder), rendered in the viewport.
3. Toolchain install sanity (first-run build time, emulator boot, debug attach).

## Concrete SQLite test

- `internal/db`: open SQLite (pure-Go `modernc.org/sqlite`), create one small table
  (e.g. `dict(value TEXT)`), seed one row, read it back via `SELECT value FROM dict LIMIT 1`.
- Display the row's text in the viewport on first render (server-side).
- Also wire the same read through a datastar SSE fragment triggered by a button click, so the
  identical read path is exercised over the SSE channel being tested in goal 1.

## Success criteria

| # | Check | Pass |
| --- | --- | --- |
| W1 | Server-rendered index shows inside the Wails v3 WebView (physical device, arm64) | PASS (RESULTS.md, screenshot) |
| W2 | App launches and stays alive on the device | PASS |
| W3 | SQLite row value is displayed | expected text visible |
| W4 | datastar SSE fragment updates on button click | DOM morphs without full page reload |

Device testing replaced the original emulator plan (`android:run`); see RESULTS.md
for the full pass/break log and the working `apk:build` + `adb install` recipe.

## Layout

```
mobile-wails-poc/
├── README.md
├── main.go            (wails3 entry; asset server + SSE fragment + sqlite read)
├── internal/db/       (sqlite open/seed/read)
├── internal/web/      (handler, templates, SSE fragment)
└── assets/            (static templates + datastar bundle)
```

Keep scratch discipline: don't polish, record what broke and what passed in
`RESULTS.md` as you go.
