# POC handover — Wails3 vs irgo (Android-only) as the mobile stack for flashcards

Goal: the same Go app we have today — datastar SSE fragments, Go first-class handlers, SQLite —
packaged as an **Android APK**. Two candidates. Verdict input so far: neither requires a PWA;
both keep SQLite and datastar essentially untouched.

> Facts below are from the two saved doc sets (~/Downloads + irgo.dev, verbatim where
> marked) and repo inspection. Anything unconfirmed is marked **[VERIFY]**.

## 1. Head-to-head

| Dimension | Wails3 Android | irgo (Android) |
| --- | --- | --- |
| Core architecture | WebView renders frontend; assets served **in-process** via WebViewAssetLoader. Go compiled as C shared lib `libwails.so` (`GOOS=android` + NDK), loaded by a small Java host. **Zero Go code changes** — same `main.go` builds desktop + mobile | **Virtual HTTP**: WebView makes `irgo://` requests; native bridge (Kotlin) intercepts → gomobile → Go runtime (chi router + templ + business logic). **No network sockets at all.** |
| HTTP model | In-process asset serving; whether long-lived SSE flushes through that path is the open risk | In-process request adapter (`core.Request/Response` ↔ `net/http`); SSE response flows through the same adapter — framework-designed for it |
| Entry point | Same `main.go` | `main.go` (`//go:build !desktop`) → `mobile.Initialize()` + `mobile.SetHandler(r.Handler())`; a `serve` arg runs a real HTTP dev server on `:8080` for browser testing |
| Desktop story | Same repo dual-targets | Same codebase, `//go:build desktop` entry (real HTTP + webview) |
| Templ | Not required | **Yes** — templ templates are the framework's view layer |
| Static assets | Handled by Wails asset loader | `r.StaticFS("/static", http.FS(staticFS))`, standard `embed.FS` |
| Build to APK | `wails3 task android:run` (builds `libwails.so`, Gradle debug APK, installs, launches); `android:package` (release APK), `android:package:fat` (universal arm64+x86_64), `android:bundle:fat` (AAB for Play) | `irgo run android` (build + run on emulator); `irgo build android` → `build/android/irgo.aar` for Android Studio integration |
| Toolchain | Android SDK + build-tools 35, NDK 26.3.11579264, JDK 21, `wails3 doctor`; first build 5–10 min, increments <1 min | Go 1.21+, `gomobile init`, Android Studio SDK + NDK, `entr` for hot reload (or `irgo run android --dev`) |
| Maturity | v3 **alpha** (mobile) | Small project, MIT, single-author; docs on irgo.dev |
| Mine/ours mapping | Zero approach change; smallest delta to existing code | **View-layer change**: our `html/template` + tailwind static templates → templ. Toolchain installs `templ` CLI; router is `github.com/stukennedy/irgo/pkg/router` (chi under the hood) |

## 2. The SSE question (the blocker you suspected)

- **Wails3**: the in-process asset loader *may* buffer long-lived SSE responses; safe plan is
  a localhost HTTP listener (`http.ListenAndServe`) and let the WebView talk to the loopback
  like a normal web page, falling back to datastar's fetch/poll pattern only if SSE hiccups.
- **irgo**: no HTTP server exists at all on mobile — requests ride the `irgo://` scheme into
  the gomobile bridge, and the adapter executes `net/http`-shaped handlers in memory. SSE
  responses are part of its core pattern (`ctx.SSE().PatchTempl(...)`), so this is designed-in
  — but **[VERIFY] in a hello-world**, because buffering semantics over the virtual adapter are
  exactly the kind of thing docs promise and devices punish.

> **Practical take: run a 30-minute hello-SSE experiment on whichever stack we pick before
> committing.** Both tools do the full APK flow; irgo's is better *aligned* with datastar, but
> Wails' team behind `libwails.so` is bigger and the exact same code shape works on desktop.

## 3. Honest trade-off

| | Wails3 | irgo |
| --- | --- | --- |
| Bigger upstream / safer bet | ✅ (well-known desktop framework; mobile is alpha but builds on v2 heritage) | ❌ (tiny, single-author) |
| Less datastar/SSE risk | ⚠️ (in-process asset server; loopback fallback likely fine) | ✅ (virtual HTTP designed around datastar SSE) |
| Zero-new-toolchain | ✅ (NDK + JDK + `wails3`) | ⚠️ (`gomobile init` + NDK + templ CLI + `entr`) |
| View-layer change | none (`html/template` stays) | templ — must regenerate templates, new handler signatures |
| Ecosystem docs/examples | docs + examples (mobile tutorials) | thin docs page set; `llms.txt` for AI help |

## Recommended POC

1. **Hello-fragment app** (both stacks, or whichever you prefer): one card flip + live
   `Card x of y` + one datastar SSE fragment + one SQLite write. Confirm SSE streams
   unbuffered through the on-device transport.
2. Read the per-stack decision matrix (row 1 = SSE live; row 2 = offline SQLite path) and
   commit to the POC results, not the docs.
3. Everything else (toolchain install, NDK, gradle) is identical for both stacks — we never
   reinstall it.

Effort so far if POC clears: ~half a day to a real sideloadable APK.

## Decision matrix for the POC

| # | Question | If YES | If NO |
| --- | --- | --- | --- |
| 1 | Hello-fragment SSE over on-device transport works unbuffered? | Proceed with chosen stack | Wails: loopback HTTP fallback. irgo: investigate adapter buffering before choosing; both fallback to fetch/poll pattern |
| 2 | SQLite writes to a writable app-sandbox path on device with no code change beyond path? | Keep schema + driver as-is (modernc) | **[VERIFY]** path accessor (OS user-cache vs framework path API) |
| 3 | Same `main.go` dual-builds desktop + Android? | Keep single codebase | Accept the entry-point split (`!desktop`/`desktop`) |
| 4 | irgo's templ view layer + `router` signatures need a view rewrite of existing templates? | Budget the rewrite | If templ is adoptable without template rewrites, irgo risk drops |
| 5 | `irgo run android` (or `wails3 task android:run`) sideloads to a physical device in one command? | Demo path proven | Take the `.aar` + Android Studio integration path |
| 6 | Debug via `chrome://inspect` works for our datastar fragments? | Iterate on-device | Use `air` / desktop dev server instead |
| 7 | Phase 1 -> Phase 2 toolchain (JDK 21 + NDK 26 + gomobile + templ) installs cleanly? | Go | Budget for the NDK install being the slow step (download ~400 MB) |
| 8 | Static assets (`assets/css`/`assets/js`/`assets/html`) embed cleanly (`r.StaticFS` for irgo; asset loader for Wails) with no changes? | Keep current asset layout | Regenerate/minify step needed; **[VERIFY]** on device |
