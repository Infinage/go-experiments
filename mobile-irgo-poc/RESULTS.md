# RESULTS.md — mobile-irgo-poc

Live log of what broke and what passed. Facts dated/session-stamped, irgo **v0.3.1**
(commit via `go install github.com/stukennedy/irgo/cmd/irgo@latest`).

## Toolchain facts

- Go 1.27.3; Android SDK `~/Android/Sdk` (NDK 26.3.11579264); JDK 21 — all pre-existing.
- `irgo install-tools`: templ + air already present, installed gomobile (`golang.org/x/mobile`
  @ 2026-10-11 revision), ran `gomobile init` — zero errors.
- Live target: attached physical Android device `ed69dfb0` via adb (no emulator binary configured;
  per user choice device is the primary target).
- irgo CLI self-provisions into `~/.irgo` ONLY for what's missing; the probing of existing
  SDK/NDK via env vars worked without extra downloads.

## Doc vs reality discrepancies (record as of v0.3.1)

1. GitHub README/main documents `irgo project new`, `irgo app run android`, `irgo tools doctor`.
   Actual v0.3.1 CLI: `irgo new`, `irgo run android`, `irgo install-tools` (no doctor/tools verbs).
2. `irgo install-tools --help` does NOT honor `--help`: it just runs and installs.
3. Scaffold ships **broken out of the box**: templ generator v0.3.1020 emits
   `templ.ResolveAttributeValue` but go.mod pins templ v0.3.977 → build fails with
   `undefined: templ.ResolveAttributeValue`. Fix: `go get github.com/a-h/templ@v0.3.1020` + tidy.
4. `irgo new` scaffolds ios/ but NOT android/Example. For APK, v0.3.1's `run android` requires
   an Android Studio-shaped project at `android/Example` (gradlew + app/libs/irgo.aar +
   kotlin/com/irgo/*.kt); the module ships a complete example project at module_path/android/Example
   that must be copied in by hand.
5. llms.txt (repo-bundled) matches v0.3.1 API well BUT documents `ctx.SSE().PatchElements(id, html)`
   and `RemoveElements(selector)`; actual names are `PatchHTMLByID(id, html)` / `RemoveByID(id)`.
   Also `sse.PatchSignals` exist ✓, `ctx.IsDatastar()` ✓.

## Framework API ground truth (from $GOMODCACHE/...irgo@v0.3.1 source)

- Page handlers: `r.GET("/", func(ctx *router.Context) (string, error))` — the returned string IS the
  page; **nothing forces templ**. html/template output works via this path (verified).
- Fragments without templ: `native := SSE.PatchHTMLByID(id string, html string, opts...)` wraps
  datastar-go ServerSentEventGenerator.PatchElements with `WithSelectorID(id)`; datastar-go default
  patch mode is `outer` — the fragment must BE the complete element (with its id).
- Signals: `sse.PatchSignals(map[string]any{...})` → `evt: datastar-patch-signals`.
- SSE shape observed over the wire: `event: datastar-patch-elements` + `data: selector #row-out` +
  multi-line `data: elements ...` — goes through `starfederation/datastar-go v1.1.0`.
- Testing helper exists: `github.com/stukennedy/irgo/pkg/testing` (irgotest.NewClient(r.Handler()),
  .Get, .Datastar().Get, AssertSSE...) — used in handlers_test.go.

## Test results

| Check | Result | Evidence |
| --- | --- | --- |
| I1 device install/launch | **PASS** | `./gradlew assembleDebug` → 89 MB app-debug.apk, `adb install -r` Success, launched on Oppo Android 13 (API 33); libgojni.so (arm64-v8a) loaded in-process |
| I2 html/template renders + SSE-morphs, NO templ | **PASS (device + desktop)** | Page rendered in WebView from `Mobile.renderInitialPage()` (html/template); tap → `datastar-patch-elements` morphed `#row-out` on device |
| I3 SQLite row displayed | **PASS (device)** | modernc.org/sqlite v1.62.0 pure-Go — creates/seeds/reads `/data/user/0/com.irgo.example/files/irgo.db`; initial server-render shows `sqlite-roundtrip-ok`, fragment re-read (body=417) morphs it |
| I4 datastar SSE fragment on click | **PASS (device)** | tap → `poc-hr: GET /api/card?datastar={"sqliteRows":0,"dbPath":""} → 200 body=417`; DOM morph + patch counter increment visible on screen |
| I5 chrome://inspect attach | pending | not yet attempted (setWebContentsDebuggingEnabled not enabled in shipped IrgoActivity) |
| I6 buffer/flush behavior | **PASS (observed)** | SSE response delivered as COMPLETE buffered body (417 bytes) via bridge handleRequest — shouldInterceptRequest cannot stream incrementally; fragments arrive only after handler returns. Latency imperceptible for POC semantics. THIS LIMITATION OF SHOULDINTERCEPTREQUEST IS A REAL FOUNDATION FINDING FOR THE FRAMEWORK. |

## Android issues found & FIXED in slice 3 (Kotlin/bridge layer)

6. Shipped gradlew has OLD github-style DEFAULT_JVM_OPTS quoting → `java: invalid source release`/class-name confusion; 
   fixed by dropping inner quotes (`DEFAULT_JVM_OPTS='-Xmx64m -Xms64m'`).
7. Shipped Example is missing gradle.properties entirely → AndroidX resolution fails; added
   `android.useAndroidX=true` (+ jvmargs + nonTransitiveRClass).
8. Shipped Example manifest (@mipmap/ic_launcher icons) without res/mipmap resources → AAPT linking
   error; removed android:icon/roundIcon from manifest for POC.
9. **Shipped Kotlin bridge is stale against irgo's OWN bind pipeline** — imports `irgo.Irgo` and
   `irgo.Core.Response`, classes gomobile bind could never generate for any module (bind
   names Java packages from the Go package names; irgo's own commands.go binds `<mod>/mobile →
   mobile.*`). Adapted bridge to `mobile.Mobile`/`mobile.Response` — every function signature matched 1:1.
10. IrgoWebSocketInterface expects framework WebSocketCallback not re-exported by our thin wrapper;
    POC app uses no websockets → deleted + removed addJavascriptInterface("IrgoNative")
    (DEFERRED: proper fix = re-export SetNativeCallback/WebSocket* in the app wrapper).
11. Android cwd is NOT writable → app.NewRouter()'s db.Open("irgo.db") fails with SQLITE_CANTOPEN(14),
    and log.Fatalf turns it into an instant process exit → infinite crash loop with splash forever.
    Fix: `mobile.Mobile.setDBPath(filesDir.absolutePath + "/irgo.db")` in MainActivity.onCreate
    BEFORE super.onCreate (Go wrapper exposes SetDBPath→db.SetPath override). KEY FINDING:
    initialize-order matters; a Fatalf in a Go binding library is a crash loop on Android.
12. **WebView fetch() rejects custom schemes** — datastar.js <script> loads through
    shouldInterceptRequest fine, but fetch('irgo://app/api/card') fails CLIENT-side with
    `URL scheme "irgo" is not supported` and never reaches interception console error observed).
    Fix: rebase bridge on the official `https://appassets.androidplatform.net` pseudo-domain;
    intercept that host in IrgoWebViewClient and use it as page base URL + fetch target.
    IMPORTANT FRAMEWORK FINDING: irgo's irgo://app scheme convention cannot work for
    fetch-based Datastar requests on Android WebView (at minimum not without an XHR shim).

## On-device verification (slice 3, Oppo Android 13 API 33, device ed69dfb0)

- Launch flow (logcat): `poc-init: start → router built → handler set → done`, then WebView loads
  irgo://app→appassets initial page, console "Irgo bridge initialized",
  `poc-hr: GET /static/js/datastar.js → 200 body=30732` (embedded static via bridge).
- Tap → `GET /api/card?datastar={"sqliteRows":0,"dbPath":""} → 200 body=417` → DOM morph:
  "Fragment patch (SSE re-read): sqlite-roundtrip-ok", "DB path: /data/user/0/com.irgo.example/files/irgo.db",
  "Fragment patches received: 1" — all visible on device screen (screenshot series /tmp/device4..7.png).
- DB file verified created at the app-writable path (SetDBPath override worked).

## Notes / gotchas found during slice 1–2

- `go run . serve &; kill $!` leaves the compiled child running and holding :8080 — stale-process
  pain; use `go build -o /tmp/pocserve .` + explicit binary, verify with `ss -ltn`.
- `pkill -f mobile-irgo-poc` kills the shell itself (pattern matches own cmdline).
- Page template data-binding: `data-text="$dbPath"` renders the SIGNAL (client), not the server
  SendVar — initial server fallback text is what html/template renders. Recorded, cosmetic.
