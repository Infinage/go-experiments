# TODO / requirements ledger

Live list of accepted-but-unimplemented items. Move lines to CHANGELOG-ish
commits as they land; strike through when done.

## All-cards list view

- [x] The "not selected" checkbox is tiny and hard to tap (m00804).
      Fixed (m01270): circle size-5→size-7, ✓ shown only when selected,
      card gets an outline-2 when selected.
- [x] Tapping the card's status area should toggle selection (m00804).
      Fixed (m01270): status strip div→button mirroring the checkbox;
      Edit link unchanged, selection-by-tap never navigates.

## Category dropdown (edit form, mobile)

- [x] Current custom autosuggest dropdown stays open even with no matching
      entry; annoying on mobile (m01034). DECISION (m01272, user picked):
      "typing + chips" — dropped list/datalist entirely (no native
      suggestion pill over the keyboard), added tap-to-fill known-category
      chips below the input, live-filtered by typed text (data-show,
      case-insensitive contains). Chips sit OUTSIDE the <label> so a tap
      doesn't refocus the input; names embedded via printf %q to survive
      apostrophes (m01276–m01278).

## Done this session (moved down here to keep the open list short)

- [x] Camera switch (m01404): camera.js enumerates videoinput devices
      after the permission grant and shows a hidden #camera-switch
      (card_form.html modal row) cycling by exact deviceId; mirroring now
      follows the active track's getCapabilities().facingMode (front =
      selfie-mirror preview + mirrored snap, rear = unmirrored); switch
      failure keeps the old stream and flashes "Switch failed" for 1.2s.
      Single-camera desktops never see the button.
- [x] Status filter in all-cards view (m01312): cardsSignals/cardsData
      gain Status; server whitelists new/learning/mastered (web.cards
      status()); store.Cards takes a 4th arg filtered by cardStatusExpr
      (same mapping the list renders); deleteCards re-render keeps the
      filter; select added next to category (data-bind status);
      TestCardsStatusFilter covers new/learning/none/nonsense + post-review
      migration. All callers/tests bumped to 4-arg Cards.
- [x] App icon (m01312): /tmp/mkicon.py redraws assets/static/icon.svg
      (cream tile, pink+yellow retro cards, ink strokes — centred stroke
      emulation) into all 5 mipmap densities (48/72/96/144/192) replacing
      the default Wails "W"; ic_launcher + ic_launcher_round.

## "not selected" / card requirement semantics

- [ ] "not selected" must respect the card requirement rules (m01034).
      Codify what a card requires before it may be excluded from the
      selected pool.

## Media on device (m01034 test session)

- [x] Save stuck → 303 redirect crashed WebResourceResponse; fixed by
      navigating to Location instead (MainActivity serveAppAsset).
- [x] Stage POST 400 "no file sent" root cause: shouldInterceptRequest
      never exposes POST bodies (WebResourceRequest has no body). Fixed by
      routing staging uploads through the JS bridge:
      wails.serveAppAssetWithBody({path,contentType,head,bodyB64,tail}) →
      nativeServeAssetBody reassembles the multipart body in Go
      (android_jni.go) and drives the identical handler. mediapick.js
      stage() uses the bridge when present, plain fetch otherwise.
      (Camera flow verified by user m01192.)
- [x] <input type=file> silent no-op — root cause: WebView's default
      onShowFileChooser does nothing (stock wails doesn't implement it
      either). Fixed: real ACTION_GET_CONTENT chooser wired in the
      WebChromeClient (FILE_CHOOSER_REQUEST 7004, clipData/multi handled);
      manifest mine type did not matter (SAF needs no storage perms). Added
      READ_MEDIA_IMAGES/READ_MEDIA_AUDIO as a <13 fallback only.
- [x] Audio record — "mic busy, close other apps" (record.js
      NotReadableError from getUserMedia, mic permission ALREADY granted).
      ROOT CAUSE (logcat): Chromium requires both RECORD_AUDIO AND
      MODIFY_AUDIO_SETTINGS to consider any mic available — without it it
      logs `cr_media: Requires MODIFY_AUDIO_SETTINGS and RECORD_AUDIO. No
      audio device will be available for recording` + `Unable to select
      communication device!`. Fixed: MODIFY_AUDIO_SETTINGS added to the
      manifest (normal permission, no runtime dialog).
      ALSO FIXED (separate trap): permission flow lacked a completion
      callback (first tap burned on deny);
      onPermissionRequest now holds the WebView request pending while the
      app-level dialog runs (onRequestPermissionsResult grants it).
      TEMPORARY: record.js err.message suffix reverted after mic verified
      (m01246); MODIFY_AUDIO_SETTINGS verified by user (m01244 batch).

## Minor

- [x] Category dropdown autosuggest is annoying without keyboard-hiding;
      same family as the mobile dropdown item above.
- [ ] Makefile css/dev targets restored from pre-task-removal state
      (verified make css works; air untested since restore).
