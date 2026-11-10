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

  // Android WebView: shouldInterceptRequest never exposes POST bodies, so a
  // plain fetch to the stage route arrives at the Go handler without its
  // file and 400s ("no file sent"). When the native bridge offers a body-
  // capable passthrough, route the upload through it; everywhere else the
  // normal fetch works.
  const bridgeStage = (typeof window !== "undefined" &&
    typeof window.wails?.serveAppAssetWithBody === "function")
    ? window.wails.serveAppAssetWithBody.bind(window.wails)
    : null;

  // One multipart body, produced either from the bridge envelope or a fetch
  // Response — stage() only cares about {ok, status, text(), json()}.
  function makeBoundary() {
    return "wailsStage" + Date.now().toString(36) +
      Math.random().toString(36).slice(2, 10);
  }

  async function stageViaFetch(fd) {
    try {
      return await fetch(STAGE_URL, {method: "POST", body: fd});
    } catch (err) {
      console.warn("staging fetch failed", err);
      return null;
    }
  }

  async function stageViaBridge(file) {
    const boundary = makeBoundary();
    let b64;
    try {
      b64 = await new Promise((resolve, reject) => {
        const fr = new FileReader();
        fr.onload = () => resolve(String(fr.result).split(",", 2)[1] || "");
        fr.onerror = () => reject(fr.error);
        fr.readAsDataURL(file);
      });
    } catch (err) {
      console.warn("staging file read failed", err);
      return {ok: false, status: 0, text: async () => "Media upload failed — try again."};
    }
    // bodyB64 is base64 of the FILE bytes; head/tail are the tiny multipart
    // framing strings. Go reassembles body = head + fileBytes + tail byte-
    // exactly, so the server sees an ordinary multipart upload.
    const payload = JSON.stringify({
      path: STAGE_URL,
      contentType: `multipart/form-data; boundary=${boundary}`,
      head:
        `--${boundary}\r\n` +
        `Content-Disposition: form-data; name="file"; filename="${file.name.replace(/"/g, "")}"\r\n` +
        `Content-Type: ${file.type || "application/octet-stream"}\r\n\r\n`,
      bodyB64: b64,
      tail: `\r\n--${boundary}--\r\n`,
    });
    let env;
    try {
      env = JSON.parse(await bridgeStage(payload));
    } catch (err) {
      console.warn("staging bridge failed", err);
      return {ok: false, status: 0, text: async () => "Media upload failed — try again."};
    }
    const text = () => {
      const raw = env.body ? atob(env.body) : "";
      return Promise.resolve(raw);
    };
    const json = () => Promise.resolve(JSON.parse(atob(env.body)));
    return {ok: env.status >= 200 && env.status < 300, status: env.status,
            contentType: env.contentType, text, json};
  }

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
    const res = bridgeStage ? await stageViaBridge(file) : await stageViaFetch(fd);
    if (!res || !res.ok) {
      let msg = res ? await res.text() : "";
      msg = !msg ? "Media upload failed — picked file was dropped. Try again."
        : msg.length < 120 ? msg : "Media upload failed — try again.";
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
