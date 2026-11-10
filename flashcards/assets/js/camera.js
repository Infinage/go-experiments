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
//
// If the machine exposes more than one camera (phones have front +
// rear), after the grant the modal enumerates the video inputs and shows
// a data-camera-switch button cycling through them. Mirroring follows
// the active camera: front-facing previews snap mirrored (selfie view),
// rear/unknown cameras snap what the lens sees.
(() => {
  "use strict";

  let stream = null;
  let targetInput = null;
  let devices = [];      // videoinput MediaDeviceInfos, after grant
  let deviceIdx = -1;    // index of the active camera in devices
  let mirrored = true;   // front-facing default

  function shot() {
    const el = document.getElementById("camera-video");
    const c = document.createElement("canvas");
    c.width = el.videoWidth;
    c.height = el.videoHeight;
    const ctx = c.getContext("2d");
    // The preview is mirrored like a selfie when the active camera is
    // front-facing; snap the same frame the user saw.
    if (mirrored) {
      ctx.translate(c.width, 0);
      ctx.scale(-1, 1);
    }
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
    devices = [];
    deviceIdx = -1;
    mirrored = true;
    document.getElementById("camera-video").srcObject = null;
    document.getElementById("camera-modal").style.display = "none";
  }

  function setMirrored(on) {
    mirrored = !!on;
    document.getElementById("camera-video").classList
      .toggle("-scale-x-100", mirrored);
  }

  // Track facingMode is implementation-quiet: Chromium reports a string
  // or list per getCapabilities(), other engines may omit it. Unknown
  // cameras keep the front-facing (mirrored) default.
  function trackFacing(track) {
    const f = track.getCapabilities?.().facingMode;
    const facing = Array.isArray(f) ? f[0] : f;
    return facing === "environment" ? "environment" : "user";
  }

  function attach(s) {
    stream = s;
    const track = s.getVideoTracks()[0];
    setMirrored(trackFacing(track) === "user");
    const settings = track.getSettings();
    deviceIdx = devices
      .findIndex((d) => d.deviceId === settings.deviceId);
    const el = document.getElementById("camera-video");
    el.srcObject = stream;
    document.getElementById("camera-switch").classList
      .toggle("hidden", devices.length < 2);
    document.getElementById("camera-modal").style.display = "flex";
    el.play().catch(() => {
      // Live preview without sound always autoplays.
    });
  }

  // Media-kind switches must close any open camera preview.
  window.stopActiveCamera = function () {
    if (stream) close();
  };

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
    // Labels are only filled in after a live grant; enumerate now so the
    // switch button can cycle by deviceId from the first switch on.
    try {
      const all = await navigator.mediaDevices.enumerateDevices();
      devices = all.filter((d) => d.kind === "videoinput");
    } catch {
      devices = []; // switch stays hidden; single-camera behaviour
    }
    attach(stream);
  }

  // Cycle to the next video input. Re-prompts on desktops whose second
  // camera was never granted; on failure keep showing the old stream.
  async function switchCamera() {
    if (devices.length < 2 || deviceIdx < -1) return;
    const next = devices[(deviceIdx + 1) % devices.length];
    try {
      const s = await navigator.mediaDevices.getUserMedia({
        video: { deviceId: { exact: next.deviceId } },
        audio: false,
      });
      stream.getTracks().forEach((t) => t.stop());
      attach(s);
    } catch {
      const btn = document.getElementById("camera-switch");
      btn.textContent = "Switch failed";
      setTimeout(() => { btn.textContent = "Switch camera"; }, 1200);
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
    if (evt.target.closest("[data-camera-switch]")) switchCamera();
  });

  // Replay any clicks that happened before real handlers existed.
  document.removeEventListener("click", preBoot);
  for (const evt of starts)
    if (evt.target.closest?.("[data-camera-target]"))
      evt.target.closest("[data-camera-target]").click();
})();
