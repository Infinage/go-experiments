# RESULTS.md — throwaway feasibility probe log

Recorded as work proceeded. Device: physical OPPO CPH2223 (Android 13, arm64-v8a), USB debugging.
Toolchain: wails3 v3.0.0-alpha2.103, SDK at `/home/kael/Android/Sdk`, Gradle 9.2.1, modernc.org/sqlite, Datastar local bundle.

## What was proven

| ID | Claim | Result | Evidence |
| --- | --- | --- | --- |
| W1 | Index renders server-side (html/template) inside a Wails v3 WebView on Android | PASS | Screenshot: "Wails + Datastar + SQLite POC" page on device |
| W2 | App launches and stays alive on the device | PASS | `pidof com.wails.app` stable, WebView chromium loaded, bridge initialized |
| W3 | SQLite (modernc.org/sqlite) works inside the Go backend on android/arm64 | PASS | Rendered "hello from SQLite inside the Wails webview" |
| W4 | Datastar SSE fragment round-trip works on device | PASS | Tap "Re-read from SQLite (SSE)" → "re-read at 07:14:49 (SSE)" patched in via `event: datastar-patch-elements` |
| D1 | Same handler works on desktop (preview mode) | PASS | `bin/mobile-wails-poc` built and ran with the same index render + fragment |

## What broke, in order

1. **`template.ParseFS` "pattern matches no files" while `fs.Glob` matched.**
   Cause: `New()` applied `fs.Sub(assets, "assets")` a second time (test had already
   subbed). `fs.Sub` does not check existence and `fs.Glob` returns `(nil, nil)` for a
   missing dir — the error message was a lie. Fix: pass a project-root FS everywhere
   (embed root already contains `assets/`), parse `assets/index.html`, serve paths as-is.
2. **First APK had no native lib / built for x86_64.** The toolchain defaults to the
   host arch (emulator target). Fix: `Taskfile.yml` `apk:build` pins `ARCH: arm64`.
3. **~1s crash loop after launch, no crash in `logcat -b crash`.**
   Cause: `db.Open` fallbacks are all unwritable for an app uid on Android
   (`os.UserCacheDir` unsupported, `/tmp` unwritable, cwd `/`) → `log.Fatal` → clean
   exit, invisible because Go's stderr is lost on Android. Fix:
   - `main.go`: on android, route `log` output to
     `/data/data/com.wails.app/cache/poc-go.log` (pull with
     `adb shell run-as com.wails.app cat cache/poc-go.log`)
   - `main.go`: on android, default DB path to `/data/data/com.wails.app/files/poc.db`.
4. **datastar.js 404 on device ("asset not found: /assets/js/datastar.js").**
   Cause: `static()` prepended an extra `assets/` prefix to an FS whose root already
   contains `assets/`. Fix: serve URL paths as-is; kept a regression test
   (`TestStaticServesDatastarBundle`).

Note: OPPO blocks `crash_dump` — native crashes never reach `logcat -b crash` on this
device; "process has died" loops with no crash entry are the tell.

## Build / run recipe (working as of this log)

```sh
wails3 task apk:build                 # Go (arm64) + gradle assembleDebug → bin/mobile-wails-poc.apk
adb install -r bin/mobile-wails-poc.apk
adb shell am start -n com.wails.app/.MainActivity
```

Direct gradle (when needed): the gradle binary lives in
`~/.gradle/wrapper/dists/gradle-9.2.1-bin/...` and needs
`build/android/local.properties` with `sdk.dir=/home/kael/Android/Sdk`
(the wails3 task sets this up itself).

Go tests: `go test ./internal/web/ ./internal/db/` — cover index render, SSE patch
shape, and the static bundle path. All pass.

## Not verified

- W5-style power/battery impact (device was on charger, 100%) — needs a longer
  unplanned session on battery to measure background drain.
- `chrome://inspect` DevTools attach — not attempted; adb-over-webview debugging
  would need `WebView.setWebContentsDebuggingEnabled`, which the alpha template may
  gate behind a flag.

## Honest caveats for the write-up

- The APK is 29 MB debug (Go + embedded assets + webview scaffolding). Release size
  and startup time were not optimized.
- `com.wails.app` applicationId and hardcoded app cache/files paths are POC
  scaffolding straight from the alpha template; a real project would own its IDs and
  paths (Activity-provided dirs or a path resolver).
- Splash-to-content transition was instant in practice but is not instrumented.
