// Microphone recorder: wires [data-record-target] buttons to record audio
// into the referenced file input, so recordings flow through the normal
// form upload path.
(function () {
  "use strict";

  const state = new WeakMap(); // button -> {recorder, chunks, stream}

  function setInputFile(input, blob) {
    const file = new File([blob], "recording.webm", {type: blob.type});
    const dt = new DataTransfer();
    dt.items.add(file);
    input.files = dt.files;
    input.dispatchEvent(new Event("input", {bubbles: true}));
    input.dispatchEvent(new Event("change", {bubbles: true}));
  }

  function label(btn, text) {
    btn.textContent = text;
  }

  document.addEventListener("click", async (e) => {
    const btn = e.target.closest("[data-record-target]");
    if (!btn) return;
    e.preventDefault();

    const st = state.get(btn);
    if (st && st.recorder && st.recorder.state === "recording") {
      st.recorder.stop(); // onstop handler does the cleanup
      return;
    }

    let stream;
    try {
      stream = await navigator.mediaDevices.getUserMedia({audio: true});
    } catch (err) {
      label(btn, "Mic unavailable");
      btn.disabled = true;
      return;
    }
    const recorder = new MediaRecorder(stream);
    const chunks = [];
    recorder.addEventListener("dataavailable", (ev) => {
      if (ev.data.size > 0) chunks.push(ev.data);
    });
    recorder.addEventListener("stop", () => {
      stream.getTracks().forEach((t) => t.stop());
      const blob = new Blob(chunks, {type: recorder.mimeType});
      const input = document.querySelector(btn.dataset.recordTarget);
      if (input && blob.size > 0) setInputFile(input, blob);
      state.delete(btn);
      label(btn, "Record audio");
    });
    recorder.start();
    state.set(btn, {recorder: recorder, chunks: chunks, stream: stream});
    label(btn, "Stop recording");
  });
})();
