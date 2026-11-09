# Irgo framework — notes for flashcards mobile POC

> Source note: this file aggregates multiple web-search results (2026 session). The search
> backend summarised docs/README content — treat facts below as **orientation, not verbatim
> documentation**. Confirm load-bearing details (CLI commands, template API names) against
> `github.com/irgo/irgo` before building. No live fetch of irgo.app / GitHub was possible in the
> session that produced this file.

## Identity

- Name: **Irgo** — a Go mobile framework.
- Repo: `https://github.com/irgo/irgo`.
- Docs site: `https://irgo.app` (also referenced as `irgo.irgo.app`).
- Org repos: `https://github.com/irgo` (CLI, templates, examples, docs).
- Positioning (paraphrased): write app logic once in Go, run on Android + iOS, with a WebView-style
  runtime and hot reload. Not Flutter-style UI cross-platform — the UI layer is a web/native
  shell; the Go layer is the shared brain.

## Architecture (as described)

Three layers:

1. **Mobile shell** — Android WebView / iOS WKWebView hosting the app's UI ("Goblin" is the name
   used for this shell concept in one review). Provides native packaging, permissions, and
   store-submission (listing/APK-AAB upload) concerns.
2. **Datastar + SSE transport** — hypermedia pattern: server renders HTML fragments, pushes
   them (and `$signals` state) over Server-Sent Events. Frontend JS is deliberately thin —
   the vision is "release the whole app as a single Go binary".
3. **Go backend + SQLite** — one Go binary with embedded SQLite (`modernc.org/sqlite` pure-Go,
   cross-compiles to Android without cgo). Can run remotely or *embedded in the mobile shell*.

This maps unusually well to flashcards: our app is already exactly this shape
(Go `net/http` + datastar SSE fragments + SQLite-via-modernc), so the Go half of Irgo is
close to a mental model we already use.

## templ + Datastar in the Irgo/WebView pattern

The general pattern for templ + Datastar on WebView (and what to expect inside Irgo):

- Go side: templ components (`component.templ` files → generated Go), a small HTTP handler
  tree where each route renders a fragment via `templ.Handler(...).ServeHTTP`
  (`github.com/a-h/templ`).
- Datastar in-browser is loaded from a CDN bundle or embedded static asset; `@get`/`@post`
  actions hit the same Go handlers; responses are **HTML fragments** pushed back per datastar's
  SSE response shape.
- SSE is the state-push channel (`$signals` patches over a long-lived connection). In a
  WebView shell the same transport rides the local loopback or a remote host; on a phone
  network, long-lived connections may be killed after backgrounding — reconnects and partial
  payloads are the app's job to handle.

**Open question for the POC:** does Irgo's runtime expect a specific view layer (templ) or
also accept PHP-style plain `html/template`? Unverified — this is precisely what the
POC's template-transport test answers (row 3 / row 8 of `HANDOVER_POC_WAILS_VS_IRGO.md`'s
decision matrix).

## Build/toolchain (as described; confirm before use)

- CLI: `go install github.com/irgo/irgo@latest` *(re-run against the GitHub README — the
  exact module path and verb string may differ)*.
- Typical flow: `irgo create myapp` → `cd myapp` → `irgo run android` / `irgo run ios`.
- Under the hood: `gomobile bind` (`golang.org/x/mobile/cmd/gomobile`) produces an `.aar` /
  `.xcframework` from the Go core; still Android SDK (NDK 26.x-ish) + JDK 17/21.
- Hot reload for UI and Go changes (advertised feature).

## Ecosystem reality check (from reviews)

- Irgo is **not mainstream**: small community, no `pub.dev`-scale component ecosystem,
  sparse docs/examples compared with Flutter, Capacitor/Ionic, Kotlin Multiplatform, or
  React Native as of 2024–2025 reviews.
- Datastar itself spent 2024 stabilizing from v0 → v1 (`@get` / signal syntax changed);
  risk with small projects (incl. Irgo) is being rebuilt whenever the upstream moves.
- Native-feel gap: no native transitions/gestures, camera/bluetooth/NFC/background-push all
  route through the shell bridge or quit being feasible; SSE drops on backgrounding
  (minutes) on phone networks. Internal tools / consoles / warehouse handsets are the good
  fit; consumer-grade apps are not.
- SQLite single-file, single-writer: fine for a single-user flashcards app; sync/collab
  ceilings apply only if we later grow multi-device merge.

## Where that leaves the POC

- It fits our problem shape almost suspiciously well (Go + datastar + SQLite is already our
  stack) **and** its risks are smaller than our Wails plan only if:
  1. the view-layer question (templ vs `html/template`) yields a reusable answer in the POC,
  2. SSE can be served through the shell with no buffering (same transport risk as Wails,
     but Irgo is datastar-centric — better prior there), and
  3. the build-to-APK path is one-command stable (the `irgo run android` surface).
- Name the exact `github.com/irgo/irgo` version/revision in the POC commit so the comparison
  against Wails remains auditable — small-ecosystem projects drift.

## Suggested next step

Match the two POCs' mini-app spec in `HANDOVER_POC_WAILS_VS_IRGO.md` (card flip + live
x-of-y + one datastar SSE fragment + one SQLite write) and fill its decision matrix with
measured answers, not documentation promises.
