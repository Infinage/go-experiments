# Wails-on-Android: every quirk we hit (and the fixes)

One-page account of the surprises the Wails v3 Android shell caused while
porting this datastar/SSE app, so the next person doesn't rediscover them
the expensive way. Also answers "was it a lot of work?" at the bottom.

## How the shell works (context)

On desktop, wails dev/binary runs our Go `net/http` server and the webview
is just a browser — SSE works untouched. On **Android** there is no server
the webview may call (we refused the `INTERNET` permission by design, see
below). Instead every `https://wails.localhost/...` request is
**intercepted** in Java and ferried to Go over a JNI bridge:

```
WebView GET /cards?datastar={...}
  → MainActivity.shouldInterceptRequest
    → WebViewAssetLoader → WailsPathHandler.handle(path)   ← query stripped here
      → wails serveAssetForAndroid: runs Go handler into a buffer,
        keeps only the []byte, throws every header away
        → MIME guessed from the path's file extension
```

Stock wails assumes an asset bundle (real files: `app.js`, `logo.png`).
Our app instead serves dynamic routes with extensionless paths and
streaming responses, which is where every quirk below comes from.

## The quirks

1. **SSE responses lost their Content-Type → datastar silently died.**
   *Symptom:* home page renders, then every datastar navigation (tabs,
   filters, paging, grading) does nothing. No console error, no retry.
   *Root cause:* `/cards` has no file extension, so wails' extension-based
   MIME table answered `application/octet-stream`. datastar requires
   `text/event-stream` and **silently discards the body** otherwise — the
   only trace was an `FetchExpectedTextEventStream` error name buried in
   the bundled `datastar.js`.
   *Fix:* new JNI export `nativeServeAssetFull(pathWithQuery, method)` —
   Go runs the *real* handler (`assetHandler` + `httptest.NewRecorder`)
   and returns `{status, contentType, body}` JSON; Java builds a proper
   `WebResourceResponse`. (`android_jni.go`, `WailsPathHandler.java`.)

2. **The asset loader stripped the query string.**
   datastar sends its signals as `?datastar={...}`. WebViewAssetLoader
   dropped it before Go ever saw the request — even with the MIME fixed,
   every filter/search would have run signal-less.
   *Fix:* interception passes `path + "?" + query` (and the HTTP method)
   through to Go; POST goes via the same call with a body.

3. **Returning a 3xx from `shouldInterceptRequest` breaks the webview.**
   *Fix:* `serveAppAsset` inspects the Location header and calls
   `webview.loadUrl(location)` instead of returning a 3xx response.

4. **Second envelope bug in *our* Go path once headers flowed again.**
   The `ctRecorder` used to capture the Content-Type pre-write sniffed
   only the first 3 bytes; whitespace-only HTML starts made it answer
   `text/plain`, so the **edit page rendered as raw HTML text**.
   *Fix:* record what the handler *declared* before its first Write; if
   none, sniff the first 512 bytes of the buffered body like production
   Go does.

5. **`<input type=file>` silently no-ops on stock wails Android.**
   Stock wails never implements `WebChromeClient.onShowFileChooser`, the
   permission statement was missing; file uploads did nothing.
   *Fix:* implemented chooser + `RECORD_AUDIO` runtime permission +
   WebView `onPermissionRequest` grant path for camera/mic.

6. **The .so must be cross-compiled with the NDK toolchain, always.**
   *Symptom:* app kept stopping at launch, or hung on the splash screen
   while the page still worked. Plain `go build` with desktop flags
   produced a `.so` that loaded but misbehaved (build-mode/PIE issue).
   *Fix:* `GOOS=android GOARCH=… CC=$NDK/…/aarch64-linux-android21-clang
   CGO_ENABLED=1 go build -buildmode=c-shared` per ABI. Also: an installed
   APK with a **stale** `.so` that predates a new Java→Go export crashes
   with `UnsatisfiedLinkError` — a fresh install (not an update-over)
   avoids mystery crashes whenever a new `native` symbol is added.

7. **`TMPDIR` for SQLite/Go temp on device.** Android gives no
   writable temp dir; `main.go`'s android branch creates the app cache
   dir and sets `TMPDIR` before anything touches temp files.

8. **Two transports coexist on Android.** Wails' own runtime calls
   (`/wails/...`, native JSON messaging) use a separate path from our app
   requests. Everything datastar/API/media must use the repaired
   interception transport; only wails-internal calls stay native.
   A stray ServiceWorker registration error seen early on was cosmetic
   and unrelated (Chrome-only API, not active in the webview).

## Decisions that shaped the fix

- **No `INTERNET` permission** (user decision). The clean alternative —
  an embedded Go HTTP server on `127.0.0.1` + cleartext network-security
  config, where SSE works *exactly like desktop* for free — was
  therefore off the table. That one permission would have made quirks
  1–4 all disappear; everything above is the cost of refusing it.
- Repair the bridge generically (headers + status + query flow through)
  rather than patching wails' MIME table per-path: any future
  content type works automatically, and we don't fork wails' Go module.

## Was it a lot of work?

The core fix is small — roughly ~250 lines across `android_jni.go`,
`WailsPathHandler.java`/`MainActivity.java`, plus the media/permission
plumbing (file chooser, camera capture, mic) that stock wails simply
doesn't ship. The *cost* was almost entirely debugging time: each bug
presents as "UI loads but nothing works" with zero errors, and the
answers live in wails' own Android plumbing (Java + JNI), not in our
app code. Lessons:

- Prototype the transport (one GET over the bridge returning SSE)
  **before** porting the real app — one POC would have surfaced
  quirks 1–2 in an hour instead of a day each.
- "No INTERNET permission" is achievable, but price it up front: you are
  choosing the interception bridge for all dynamic traffic.
- Keep wails' module unpatched; own the Java bridge files (they're
  generated into the repo) so fixes survive tool re-runs.

Anything content-type-dependent added later (PDF previews, video,
whatever) works through the same bridge with no further changes — that
was the point of fixing the envelope instead of special-casing SSE.
