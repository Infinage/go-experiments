// Card-form media pick plumbing.
//
// The save request runs over GET (signals on the query string — the
// Android webview asset bridge rejects request bodies), so media bytes
// can never ride the save. Each pick is uploaded once to the staging
// store (POST /cards/media/stage) and the short staged URL is mirrored
// into a hidden data-bind input next to the file-name pick gate. The
// save signals then reference files, never bytes.
//
// The datastar bundle's data-on:change expressions proved unreliable on
// file inputs (they silently no-op), so this script drives both gates.
(() => {
  const STAGE_URL = "/cards/media/stage";
  const CHANNELS = {
    "front_image": ["front_image_pick", "front_image_staged", "img"],
    "front_image_camera": ["front_image_pick", "front_image_staged", "img"],
    "front_audio": ["front_audio_pick", "front_audio_staged", "aud"],
  };

  function setPartner(id, value) {
    const partner = document.getElementById(id);
    if (!partner || partner.value === value) return;
    partner.value = value;
    partner.dispatchEvent(new Event("input", {bubbles: true}));
    partner.dispatchEvent(new Event("change", {bubbles: true}));
  }

  // setValue + a datastar-visible input event on the toast partner.
  function pushToast(msg) {
    const el = document.getElementById("toast_partner");
    if (!el) return console.warn(msg);
    el.value = msg;
    el.dispatchEvent(new Event("input", {bubbles: true}));
    clearTimeout(el._timeout);
    el._timeout = setTimeout(() => {
      el.value = "";
      el.dispatchEvent(new Event("input", {bubbles: true}));
    }, 6000);
  }

  // Server-emitted save-error responses call this directly (the datastar
  // fetch plugin executes text/javascript bodies).
  window.toast = pushToast;

  // Match internal/web/media.go limits; rejecting before the fetch avoids
  // uploading megabytes that MaxBytesReader will cut mid-body anyway.
  const MAX_BYTES = {img: 5 << 20, aud: 10 << 20};

  async function stage(input, file, pickId, stagedId, kind) {
    setPartner(stagedId, ""); // clear a previous pick's URL while staging
    if (!file) return;
    if (file.size > MAX_BYTES[kind]) {
      dropPick(input, pickId, stagedId,
        kind === "img" ? "Image too large — max 5 MB. Pick a smaller one."
                       : "Audio too large — max 10 MB. Trim or pick a shorter clip.");
      return;
    }
    const fd = new FormData();
    fd.append("file", file, file.name);
    let res;
    try {
      res = await fetch(STAGE_URL, {method: "POST", body: fd});
    } catch (err) {
      console.warn("staging fetch failed", err);
      dropPick(input, pickId, stagedId,
        "Media upload failed — picked file was dropped. Try again.");
      return;
    }
    if (!res.ok) {
      let msg = await res.text();
      msg = msg.length < 120 ? msg : "Media upload failed — try again.";
      dropPick(input, pickId, stagedId, msg.trim());
      return;
    }
    const {url} = await res.json();
    setPartner(stagedId, url);
  }

  // A failed stage leaves the preview flapping on a pick that nothing
  // backs: clear the input so the picker resets and the card-form busy
  // spinner (waiting on pick-without-staged) unblocks.
  function dropPick(input, pickId, stagedId, msg) {
    if (input) input.value = "";
    setPartner(pickId, "");
    setPartner(stagedId, "");
    pushToast(msg);
  }

  document.addEventListener("change", (e) => {
    const chan = CHANNELS[e.target.name];
    if (!chan) return;
    const [pickId, stagedId, kind] = chan;
    const file = e.target.files?.[0];
    if (file) setPartner(pickId, file.name);
    stage(e.target, file, pickId, stagedId, kind);
  });

  // The toast signal is server-driven (save errors push toast(...) here)
  // and client-driven from stage failures below, but ordinary form typing
  // must never round-trip through it.
})();
