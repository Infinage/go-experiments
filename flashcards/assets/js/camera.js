// Live camera capture for card front images.
//
// The [capture] attribute of a file input only influences the UI on
// mobile browsers; on desktop it is ignored and the input falls back to
// a plain file dialog. So the Camera button here opens a live webcam
// modal (getUserMedia + canvas snap) whenever that is possible, and
// clicks the capture input instead otherwise (phones keep the native
// UI, desktops without a camera or denied permission get the file
// dialog).
//
// Buttons marked data-camera-target="#<input-id>" control the modal.
// The snapped frame is pushed into that file input as a File, so the
// normal change handler and form upload paths keep working.
(() => {
  "use strict";

  let stream = null;
  let targetInput = null;

  function shot() {
    const el = document.getElementById("camera-video");
    const c = document.createElement("canvas");
    c.width = el.videoWidth;
    c.height = el.videoHeight;
    const ctx = c.getContext("2d");
    // The preview is mirrored (-scale-x-100 like a selfie view); snap the
    // same mirrored frame so the photo matches what was on screen.
    ctx.translate(c.width, 0);
    ctx.scale(-1, 1);
    ctx.drawImage(el, 0, 0);
    c.toBlob((blob) => {
      if (blob) {
        const dt = new DataTransfer();
        dt.items.add(new File([blob], "camera.jpg", { type: "image/jpeg" }));
        targetInput.files = dt.files;
        targetInput.dispatchEvent(new Event("change", { bubbles: true }));
      }
      close();
    }, "image/jpeg", 0.92);
  }

  function close() {
    if (stream) stream.getTracks().forEach((t) => t.stop());
    stream = null;
    targetInput = null;
    document.getElementById("camera-video").srcObject = null;
    document.getElementById("camera-modal").style.display = "none";
  }

  async function open(input) {
    targetInput = input;
    try {
      stream = await navigator.mediaDevices.getUserMedia({
        video: { facingMode: "user" },
        audio: false,
      });
    } catch (err) {
      // No camera / denied / insecure context: native capture flow.
      targetInput = null;
      input.click();
      return;
    }
    const el = document.getElementById("camera-video");
    el.srcObject = stream;
    document.getElementById("camera-modal").style.display = "flex";
    try {
      await el.play();
    } catch {
      // Playback without sound always autoplays; ignore defensively.
    }
  }

  let starts = []; // queued opens while camera.js hasn't loaded yet

  // Buttons can appear before this deferred script runs, so record early
  // clicks once and replay them below. The recorder must be detached
  // before replay: the replayed .click() would otherwise bubble back
  // into this listener and re-queue itself forever.
  const preBoot = (evt) => {
    if (evt.target.closest?.("[data-camera-target]")) starts.push(evt);
  };
  if (!window.__cameraBooted) {
    window.__cameraBooted = true;
    document.addEventListener("click", preBoot);
  }

  document.addEventListener("click", (evt) => {
    const btn = evt.target.closest("[data-camera-target]");
    if (btn) {
      const input = document.querySelector(btn.dataset.cameraTarget);
      if (!input || !navigator.mediaDevices?.getUserMedia) {
        input?.click(); // native capture input (camera on phones)
      } else {
        open(input);
      }
      return;
    }
    if (evt.target.closest("[data-camera-snap]")) shot();
    if (evt.target.closest("[data-camera-close]")) close();
  });

  // Replay any clicks that happened before real handlers existed.
  document.removeEventListener("click", preBoot);
  for (const evt of starts)
    if (evt.target.closest?.("[data-camera-target]"))
      evt.target.closest("[data-camera-target]").click();
})();
