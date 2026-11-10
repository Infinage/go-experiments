# Flashcards APK Handover

**Status: web app feature-complete and tested; APK build pipeline scaffolded and previously proven end-to-end on device (POC). Next step: rebuild the APK from this repo's current source and re-run the on-device flows listed below.**

## Where things stand

- Web app (Go + datastar SSE + sqlite) is the single source of truth; the APK shell (Wails3 WebView) wraps the same handlers — `main.go` serves plain HTTP, `main_android.go` feeds the same `internal/web` into a Wails window under the `android` build tag.
- Android build tree lives in `build/` (gradle wrapper, manifest, native bridge sources); build orchestration is the root `Makefile`'s `apk` targets (go-task was removed — see README "Mobile build provenance"). `main.go` / `main_android.go` entrypoints done. Icons placeholder尚未 replaced.
- Previous POC (now deleted from disk — its outputs live in git history of `mobile-wails-poc`) verified: APK installs, launches, WebView renders the app, sqlite opens on-device storage, seeding via adb works. Known issues found and fixed in POC — service-worker cache staleness (bump `assets/static/sw.js` VERSION on every asset change) and activity-launch flicker on cold start (double-launch guard in `MainActivity`, retained in `build/android/app/src/main/java`).
- Desktop/web build remains the default: `make` / `air` for dev (`tmp/` gitignored), `bin/` outputs gitignored.

## Feature parity checklist (just landed on web; APK must match)

- Staged-media save: large front images/audio no longer put into the card edit form save URL; media stages to server first (`internal/web/staging.go`), then save references the staged path. Works for camera-captured and uploaded files up to 32 MB.
- "None" pseudo-category (`__none__`): study mode + all-cards filter to cards without tags; counts in dropdowns are live (study shows pending-review counts, decremented after each grade via `pendingSignals`).
- Review counter: `Card x of y` stays consistent across paging (Next/Previous), grades, category filter, hide-mastered toggle, reset-progress. Class of bug from before: stale UI binding; fixed by recompute+patch pattern.
- Toast: red tint, `shadow-retro-lg` to match preview card; shown on client-side saved-tag rejects ("none" reserved) and recorded-file size-limit hits.
- Recorder cancel: previously the cancel path silently failed (`WeakMap.values()` misuse); fixed via explicit `live` tracking in `assets/js/record.js`.

## Flows to test on device after fresh APK install

1. Study pass with new cards: N never-reviewed cards appear in pass queue; counter advances 1→2→… and terminates at "All caught up" with no infinite loop on "Review anyway".
2. Grades: Easy/Medium/Hard push card out (due tomorrow+); Forgot requeues it once at the end of the queue and it must reappear exactly at queue end, then work like a normal grade.
3. Counter must not double-count when paging back (← Previous) after grading, nor after category change mid-study.
4. Category filter switch mid-session: list/count refresh immediately; hide-mastered toggle same behavior.
5. Reset progress: reset current category only; counter and pending counts return to full N.
6. None pseudo-category: dropdown shows `None (n)`; filtering to None shows only untagged cards; after grade the count decrements live; Reset-progres on None resets only untagged cards.
7. Save card with tag "none" (lowercase): rejected with `Tag name “None” is reserved` toast.
8. Front media: image upload, camera capture, audio upload, record; cancel record; size limit rejects with toast; large image/audio (5–30 MB) saves without the old lock-up.
9. Staged media persists across save→edit→save round-trip (no media loss).
10. Toast styling: red, retro shadow, no overlap issues.

## Build/test commands

```sh
make css              # regenerate web assets (tailwindcss)
make build            # desktop/dev binary
make dev              # dev server + css watch
# APK:
make css              # edit assets? regenerate tarball-fresh css first
make apk              # libwails.so (arm64) + gradle assembleDebug
# output: bin/flashcards.apk
make apk-run          # install -r + launch on the connected device
```

(Targets live in the root `Makefile`; device selection via `DEVICE_ID=<serial>`.)

## Known not-yet-automated checks

- Camera capture/record on a physical device — headless test browsers can't grant real mic/camera; verify manually per flows 8.
- Service-worker VERSION bump discipline: after any change to `assets/js/*` or html, bump `assets/static/sw.js` VERSION constant before rebuilding, or stale caches will mask fixes on device (bit us twice).
