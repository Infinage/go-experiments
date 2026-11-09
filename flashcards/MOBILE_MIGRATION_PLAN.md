# Mobile migration decision — Wails3 vs irgo (post-POC, both proven on device)

Decision record + migration plan. Evidence: both POCs' `RESULTS.md` logs (physical Oppo
Android 13, arm64): `mobile-wails-poc/RESULTS.md`, `mobile-irgo-poc/RESULTS.md`.

## Verdict

**Wails3.** Both POCs passed their core checks, but irgo needed 12 issues resolved (5 of
them framework-defect fixes inside an 857-line hand-patched Kotlin bridge), while Wails
needed 4 small, test-covered fixes entirely in Go/Taskfile. Long-term maintainability
points the same way: the irgo POC carries a Kotlin bridge that diverges from upstream
(framework's shipped bridge imports classes gomobile could never generate); every upstream
update would invalidate our patches. irgo upstream is also quiet (last commit ~3 months
ago) and its scaffold does not build out of the box.

## Facts from the POCs

| | wails-poc | irgo-poc |
| --- | --- | --- |
| Core checks | W1–W4 + D1 PASS (desktop preview too) | I1–I4, I6 PASS; I5 (chrome inspect) pending |
| Issues encountered | 4 (template FS double-sub, x86_64 default arch, DB path to app files, datastar 404) | 12 incl. scaffold broken (templ pin mismatch), missing android/Example (manual copy), stale Kotlin bridge (un-compilable imports — adapted 1:1), gradlew quoting, missing gradle.properties, missing mipmap res, **`irgo://` custom scheme cannot serve datastar fetch()** (rebased onto `appassets.androidplatform.net` pseudo-domain), Fatalf→crash-loop |
| SSE behaviour | streams through WebViewAssetLoader, morphs live | **only delivered after handler returns** (shouldInterceptRequest buffers; fine for our fragment latency today, but a real foundation limit) |
| Native-side code we own | none (Kotlin host is wails-generated) | 857 lines Kotlin (IrgoActivity/Bridge/WebViewClient/WebSocketInterface), hand-fixed |
| APK size | 29 MB debug | 89 MB debug |
| Effort signature | small fixes, all in `internal/web` + Taskfile + main | build system archaeology + bridge surgery |

## Pros / cons (brief)

**Wails3**
+ No native code to own; mobile + desktop from one `main.go`; SSE works; ports flashcards nearly 1:1 (html/template + datastar + modernc sqlite all stay); active upstream; smaller APK than irgo.
− v3 alpha (API churn — pin the version in Taskfile/tooling notes); arch must be pinned (arm64) in `apk:build`; `com.wails.app` appId + paths must be replaced with our own; chrome://inspect unverified.

**irgo**
+ Virtual-HTTP design is conceptually neat; `PatchHTMLByID`/`PatchSignals` worked exactly as documented; POC proved the no-templ path works (`html/template` letters + SSE morphs pass).
− Hand-maintained Kotlin bridge = permanent maintenance debt; several framework-embedded defects (custom fetch scheme, stale bridge, broken scaffold versions); SSE only after handler completes; small/quiet upstream; heavier APK; demo not buildable out of the box.

## Migration plan (flashcards → Wails3 Android)

Phase 1 — project shell
1. New build files via `wails3 init`-equivalent copied from mobile-wails-poc (Taskfile.yml with `apk:build` pinning `ARCH: arm64`, gradle dir, wails.json).
2. `main.go` split: keep web mode (`http.ListenAndServe`, untouched default); add `//go:build android` entry reusing the POC's main shape (Wails asset server + datastar + sqlite read).
3. applicationId: `com.<user>.flashcards`; splash/icon from our beat.

Phase 2 — port the server layer
4. `internal/web`, `internal/store`, `assets/` (html/tailwind/camera JS) — expected unchanged (same shapes the POC proved).
5. DB path: app files dir (`/data/data/<appId>/files/flashcards.db`) via POC's `db.Open` fallback list; log routing to app cache (POC's `poc-go.log` pattern) until verified with `adb logcat`/`run-as cat`.

Phase 3 — device verification (the audit scripts, on device)
6. Counter semantics run-through: x-of-y under paging/grade/filter/reset-status (the exact regressions fixed in 7fcb1ef + reviewer round) on the Wails webview.
7. Media: image/audio upload/capture/record through WebView file inputs (familiar flashcards behavior — the only item POC did not exercise; treat as **[VERIFY]** in a scratch build).
8. chrome://inspect + battery sanity (the two "not verified" rows).

Phase 4 — release-ready
9. `wails3 task android:package:fat` (universal debug APK) first, then release with our keystore; Play path: `android:bundle:fat` AAB + own versionConfig.

Rule: keep every android-only delta inside `//go:build android` files + `Taskfile.yml`/gradle — the desktop web build must stay our default, zero-branch flow.
