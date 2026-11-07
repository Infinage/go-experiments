// Microphone recorder: wires [data-record-target] buttons to record audio
// into the referenced file input, so recordings flow through the normal
// form upload path. A recording panel (the input's closest div) gets the
// .recording class while the mic is live: CSS shows the
// [data-record-live] row (red dot, timer, stop/cancel buttons) and hides
// the [data-record-hide] helper elements for the duration.
(function () {
  "use strict";

  const state = new WeakMap(); // target input -> {recorder, chunks, stream}

  function setInputFile(input, blob) {
    const file = new File([blob], "recording.webm", {type: blob.type});
    const dt = new DataTransfer();
    dt.items.add(file);
    input.files = dt.files;
    input.dispatchEvent(new Event("input", {bubbles: true}));
    input.dispatchEvent(new Event("change", {bubbles: true}));
  }

  function openUI(input) {
    return {
      panel: input.closest("div"),
      timer: null,
    };
  }

  // Toggle the .recording class on the panel; CSS does the show/hide.
  function recordingUI(input, st, on) {
    const ui = openUI(input);
    ui.live = ui.panel?.querySelector("[data-record-live]") || null;
    if (on) {
      ui.panel.classList.add("recording");
      ui.timer = ui.live?.querySelector("[data-record-timer]") || null;
      if (ui.timer) {
        let secs = 0;
        ui.timer.textContent = "0:00";
        st.ticks = setInterval(() => {
          secs++;
          ui.timer.textContent =
            Math.floor(secs / 60) + ":" + String(secs % 60).padStart(2, "0");
        }, 1000);
      }
      return;
    }
    clearInterval(state.get(input)?.ticks);
    ui.panel.classList.remove("recording");
  }

  const pending = new Set(); // start() awaiting getUserMedia

  function start(btn) {
    const input = document.querySelector(btn.dataset.recordTarget);
    if (!input) return;
    if (state.get(input) || pending.has(input))
      return; // already recording (or starting) via another button

    if (!navigator.mediaDevices?.getUserMedia) {
      btn.textContent = "Mic unavailable";
      btn.disabled = true;
      return;
    }
    pending.add(input);
    navigator.mediaDevices.getUserMedia({audio: true}).then((media) => {
      pending.delete(input);
      const recorder = new MediaRecorder(media);
      const chunks = [];
      recorder.addEventListener("dataavailable", (ev) => {
        if (ev.data.size > 0) chunks.push(ev.data);
      });
      recorder.addEventListener("stop", () => {
        // Restore the panel UI while the record state is still readable.
        recordingUI(input, state.get(input), false);
        const st = state.get(input);
        state.delete(input);
        media.getTracks().forEach((t) => t.stop());
        if (st?.cancelled) return;
        const blob = new Blob(chunks, {type: recorder.mimeType});
        if (blob.size > 0) setInputFile(input, blob);
      });
      recorder.start();
      state.set(input, {recorder, chunks, stream: media, cancelled: false});
      recordingUI(input, state.get(input), true);
    }).catch(() => {
      pending.delete(input);
      btn.textContent = "Mic unavailable";
      btn.disabled = true;
    });
  }

  document.addEventListener("click", (e) => {
    const btn = e.target.closest("[data-record-target]");
    if (btn) {
      e.preventDefault();
      const input = document.querySelector(btn.dataset.recordTarget);
      if (!input) return;
      const st = state.get(input);
      if (st) st.recorder.stop(); // onstop handler does the cleanup
      else start(btn);
      return;
    }
    const cancel = e.target.closest("[data-record-cancel]");
    if (cancel) {
      // Cancel whichever recording is active (the cancel button is not
      // tied to a specific input), instead of hardcoding one target.
      const st = [...state.values()][0];
      if (st) {
        st.cancelled = true;
        st.recorder.stop();
      }
    }
  });
})();
