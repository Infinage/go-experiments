# FRAMEWORK_FINDINGS.md — mobile-irgo-poc (irgo v0.3.1)

Findings from validating irgo on Android (device: Oppo Android 13, API 33, adb `ed69dfb0`).
Companion to `RESULTS.md` (which records pass/fail test evidence). Each finding lists:
**symptom → root cause → fix used → suggested framework fix**. Severity: 🔴 blocker (no
workaround path in the shipped code), 🟠 major (hours lost), 🟡 papercut.

Outcome in one line: **the Go core is solid — every failure we hit was in the
irgo-provided Android shell (gradle project, Kotlin bridge) or the bridge design
(custom scheme).**

---

## A. Clarifying the "stuck" session — what actually blocked us

The single biggest time sink was **exception-chain debugging with no logging**:
`irgo run android` (via AGP) fails as *one opaque `log.Fatal` line and exit 2* — potentially
from gradle, AGP/gomobile, javac, kotlinc, aapt2, or the Go bind step — and **irgo's own
plugin logs nothing about the failing step** and swallows the underlying error (finding A3).
We attacked it in two flanks at once: (1) instrumented irgo's plugin source (added logging,
re-ran from source to see the REAL underlying error, then reverted), (2) re-created the
APK build by hand outside irgo — `gomobile bind` + the module's own `android/Example`
gradle project — one manual step at a time. That split isolated each problem to exactly
one layer, surfaced all the findings below, and left us with a working APK sooner
than waiting on irgo's launcher would have.

### A1. `irgo new` scaffold does not compile (🔴, FIRST-RUN BLOCKER)
- **Symptom:** fresh `irgo new hi` fails `go run` with `undefined: templ.ResolveAttributeValue`.
- **Root cause:** scaffold pins `github.com/a-h/templ v0.3.977` in go.mod while the templ
  CLI it tells you to install emits code using `templ.ResolveAttributeValue` (added in
  v0.3.1020). Version skew inside the scaffold itself.
- **Fix used:** `go get github.com/a-h/templ@v0.3.1020 && go mod tidy`.
- **Suggested:** pin the scaffold to the template-engine version the installed CLI generates
  for, or make `irgo install-tools` align the version.

### A2. No `irgo doctor`; README names tools that don't exist (🟠)
- **Symptom:** README documents `irgo project new`, `irgo app run android`, `irgo tools doctor`.
  Actual v0.3.1 CLI: `irgo new`, `irgo run android`, `irgo install-tools` (no doctor verb).
  `install-tools` ignores `--help` and just runs.
- **Cost:** we couldn't get a quick "toolchain ok [Y/N]" verdict and mis-suspected the NDK.
- **Fix used:** proved each tool by running it individually (go, gomobile, gradle, adb).
- **Suggested:** `irgo doctor` is genuinely the missing first-rung tool; also sync README verbs.

### A3. `irgo run android` masks the real build error, single-word diagnosis (🟠)
- **Symptom:** `irgo run android` fails with a bare `process exited with code 2` and no
  sub-log; irgo's plugin (internal/commands/commands.go, buildAndroid) hides the gradle
  stdout/stderr that would name the failing step.
- **Root cause:** exec.Output() style capture that discards combined output on failure.
- **Fix used:** temporary instrumentation of irgo's plugin source (log the underlying
  stdout/stderr), run from a fork, capture the real error, then revert. Full manual gradle
  invocation with `--stacktrace` confirmed the same.
- **Suggested:** pipe gradle's output through (or tee to a file) on failure; a single
  `[FAIL] build exited 2` line costs users hours.

### A4. Manual APK build, NDK suspicion resolved (🟡 lesson)
- **Symptom:** suspected the pre-installed NDK 26.3 was incompatible (irgo's own toolchain
  check messages list NDK versions, e.g. r25+).
- **Root cause:** NDK 26.3 is fine. `gomobile bind -target=android -androidapi=24 -o
  irgo.aar .` exits 0 cleanly. There was never an NDK version mismatch.
- **Lesson:** the `irgo install-tools` NDK-parts message enumerates versions irgo *knows
  about*, not a hard compatibility floor; trusting existing NDK 26.3 was correct.
- (Working manual path: `apps/mobile-irgo-poc % gomobile bind -androidapi=25 -target=android -o app/libs/irgo.aar .`)

### A5. Irregular gradle()-based build pipeline (🟠 usability)
- **Symptom:** `irgo run android` shells out to gradle with a temporarily-generated settings.gradle
  that points at a cached copy of the framework's prebuilt `irgo-android.jar` and injects
  `-Pmodule.fdlibm`-style config; output interleaving makes partial-failure reporting incoherent.
- **Fix used:** bypassed; built via module example gradle project + our own wrapper task.
- **Suggested:** persist the generated settings.gradle as a project file, or switch to a
  normal gradle include and document a "build without irgo run" path — teams will need a
  CI-able build anyway.

---

## B. `irgo new` scaffold broken in several independent ways (🔴, all five independently fatal)

### B1. templ version skew (see A1)
### B2. `web` folder isn't the web (🟠)
- `web/` holds only `datastar.js` and is **not wired into the Go app at all**; the actual
  pages/templates live in `app/templates/` with go:embed. Misleading folder name/copy
  (+02-02.md says web/ is where the frontend lives).
- **Suggested:** either wire web/ into the embed/static pipeline or rename/move with docs.

### B3. `config.json` doesn't reflect real keys (🟡)
- Written scaffold config.json differs from what the app actually reads/writes
  (app/config app.Config keys); harmless but cost a debugging detour.

### B4. Shipped android/Example gradle project does not build (🔴 ×3)
1. **gradlew wrapper script** uses old github-style `DEFAULT_JVM_OPTS='"-Xmx64m" "-Xms64m"'
   quoting → java misparse / "invalid source release" style errors. Fix: drop inner quotes.
2. **No gradle.properties at all** → AndroidX resolution errors. Fix: add
   `android.useAndroidX=true`, jvmargs, nonTransitiveRClass.
3. **Manifest references mipmap icons that don't exist** (no res/mipmap-* in the example) →
   aapt2 link failure. Fix: removed icon attrs for the POC (proper fix: ship icons).

### B5. Android example Kotlin bridge is stale vs its own generators (🔴 major)
- Shipped `android/Example/app/.../IrgoActivity.kt` imports `irgo.Irgo` and
  `irgo.Core.Response`. gomobile **cannot generate a package named `irgo` or `irgo.Core`** from
  irgo's own pipeline: bind names Java packages after the *bound Go package's name* (`mobile`
  for irgo's own commands.go, which binds `<yourmod>/mobile → mobile.*`) — so even irgo's own
  gateway Android project's bridge can't compile against the AAR produced by its own binder.
- **Fix used:** rewrote bridge Kotlin to `mobile.Mobile` / `mobile.Response` (every function
  matched 1:1, mapping trivially). IrgoWebSocketInterface additionally required a
  `framework.WebSocketCallback` that our thin wrapper doesn't re-export → deleted (no websockets
  in POC); note: re-export pair `SetNativeCallback` + WebSocket* methods needed in an app
  `mobile` wrapper to restore it.
- **Suggested:** ship the example bridge against the *actual* generated API (mobile.Mobile),
  or generate the bridge from the bind output (a `irgo android scaffold` step).

---

## C. Chrome Custom Tab / bridge design findings on a real device

### C1. Custom `irgo://` scheme breaks WebView fetch() (🔴 design-level)
- **Symptom:** Datastar SSE fragments don't fetch on Android; JS console: `TypeError: Failed
  to fetch` / scheme not supported — while desktop works.
- **Root cause:** WebView/chromium **rejects non-http(s) schemes for fetch() entirely**
  (client-side, before any native interception can help). irgo's Android bridge routes through
  `irgo://app/...` via `addJavascriptInterface`/shouldOverrideUrlLoading — that combination
  supports <script> SRC loading but cannot power Datastar's fetch()-based SSE.
- **Fix used:** rebase the bridge on `https://appassets.androidplatform.net` — the official
  WebViewAssetLoader pseudo-domain, interceptable in shouldInterceptRequest with a plain
  WebViewClient; used it as the page base and for all fetch calls. Works, zero changes
  to Datastar/datastar-go.
- **Suggested (framework):** default the Android shell to appassets base URL instead of a
  custom scheme, or document the constraint loudly; alternatively provide an XHR/fetch
  JS shim for custom schemes.

### C2. SSE responses buffered wholesale (🟠 characteristic)
- **Symptom:** fragments appear only after the handler returns; no intermediate flushing.
- **Root cause:** shouldInterceptRequest returns one complete body — no incremental streaming
  across the bridge. Framework limitation, not a bug: transparent for this POC, but relevant
  for token-stream UIs (LLM chat) where irgo's positioning might attract users.
- **Suggested:** document; if long-lived streams matter later, consider keeping a persistent
  addJavascriptInterface channel or localhost HTTP listener just for SSE.

### C3. Android cwd not writable → log.Fatalf crash-loop in router init (🔴)
- **Symptom:** app stuck on splash forever; in logcat, process repeatedly exits.
- **Root cause:** app.NewRouter does `db.Open("irgo.db")` relative to cwd; Android cwd isn't
  writable → SQLITE_CANTOPEN(14) → `log.Fatalf` inside gomobile bindings = immediate process
  death + infinite relaunch loop.
- **Fix used:** `Mobile.setDBPath(filesDir.absolutePath + "/irgo.db")` from MainActivity.onCreate
  **before** super.onCreate (a SetPath override plumbed through our mobile wrapper).
- **Suggested (framework):** (a) never log.Fatalf inside library code; return error (or)
  panic with a recoverable message; (b) on Android, default the db path to the app's filesDir
  as part of the framework's mobile bootstrap.

### C4. Logcat is effectively silent about Go panics (🟡)
- Only `E crash_dump` and process-exit hints; no Go stack traces visible.
- **Suggested:** framework could install a panic hook that writes to app files + logcat.

---

## D. What worked well (worth keeping in the report)

- After workarounds, **the Go core held** end-to-end: html/template pages, Datastar SSE
  fragments (patch-elements + patch-signals), signals → server via querystring, SQLite
  (modernc.org/sqlite) read path, PWA-shaped static serving — **all verified on device**.
- The framework's own `pkg/testing` (irgotest.NewClient) is a nice touch and worked for
  handler tests.
- The engine's SSE protocol output matched llms.txt largely (only 2 method names off:
  `ctx.SSE().PatchElements`→`PatchHTMLByID`, `RemoveElements`→`RemoveByID`).
- Datastar + html/template (no templ) is viable as irgo's own docs claim ("templ is optional").
- Android's chrome custom tab deeper finding: custom-scheme bridges remain the #1 gap in
  every Go-mobile framework (irgo included); appassets.androidplatform.net is the one
  official escape hatch and works.

## E. Patching footprint — what we changed upstream vs. in-repo

Answer to "did we patch irgo or any underlying core?": **No — nothing upstream was
permanently patched.** Every fix lives inside our own repo as an app-owned adapter layer.

### Untouched (stock versions, zero patches)
- **irgo core** — used purely as a library (v0.3.1, standard go.mod pin).
- **gomobile, AGP, gradle core, templ, datastar-js, datastar-go, modernc.org/sqlite** — stock.
  Only version *selection* changed (templ bumped to v0.3.1020 in OUR go.mod to match the
  CLI's codegen output, finding A1).

### Temporary forks, already discarded
- irgo's internal plugin sources — forked to inject logging and see the real gradle error
  (A3); discarded once diagnosed.
- gradle itself — forked to trace its file-existence checking (the mkdir issue); discarded.
- Residue note: a leftover `gradle` binary in the repo binary dir remains from that session.

### What we own in-repo (the adapter layer)
1. `mobile/mobile.go` — thin Go wrapper: `SetDBPath` (cwd-not-writable fix, C3) + logging
   bridge (`poc-init:`/`poc-hr:` lines) used for tracing.
2. `android/Example/.../IrgoActivity.kt`, `IrgoWebViewClient.kt`, `MainActivity.kt` — the
   irgo-provided example Kotlin bridge, COPIED into our repo and adapted: wired to the real
   generated `mobile.Mobile`/`mobile.Response` API (B5), `IrgoWebSocketInterface` deleted
   (not re-exported by our wrapper), bridge base URL switched to
   `https://appassets.androidplatform.net` (custom `irgo://` scheme fix, C1).
3. `app/libs/irgo.aar` — rebuilt with `gomobile bind` directly instead of `irgo run`'s pipeline.
4. Gradle wrapper quoting fix (B4.1) — applied to the wrapper COPY in our repo only.

Caveat for follow-ups: when irgo fixes its Android example upstream, our copy of
IrgoActivity.kt/IrgoWebViewClient.kt becomes divergent — either re-diff against the new
upstream or consciously keep ours (ours fixes C1 + B5, which upstream may not).

## F. Testable claim summary (linked to RESULTS.md)
- I1 install/launch: PASS (device)
- I2 html/template + SSE morph, no templ: PASS (device + desktop)
- I3 SQLite row display: PASS (device)
- I4 datastar SSE on click: PASS (device)
- I5 chrome://inspect: pending (needs setWebContentsDebuggingEnabled(true) + toolbar attach)
- I6 buffer/flush: PASS with C2 caveat (buffered wholesale, not streamed)
