// Dynamic text fitting for the flip cards.
//
// Front matter: text is hard-capped at 2 rendered lines (data-fit-lines="2").
// The font shrinks dynamically within the 2-line budget; if even the floor
// font can't keep it to 2 lines, a "Read full text" chip appears (only then)
// and opens a shared modal with the full text + a shortness tip. Media (the
// image/audio strip) keeps visual priority via plain flex CSS — no JS
// budgeting.
//
// Back matter: text-only, so it keeps the classic shrink-to-fill behavior
// (grow/shrink between data-fit-min and data-fit-max with no line cap).
(function () {
  "use strict";

  var LEADING = 1.25; // leading-tight on the flip faces

  function fits(el, size, box) {
    el.style.fontSize = size + "px";
    return el.scrollHeight <= box.clientHeight &&
      el.scrollWidth <= box.clientWidth;
  }

  function fit(el) {
    var min = +(el.dataset.fitMin || 12);
    var max = +(el.dataset.fitMax || 48);
    var lines = +(el.dataset.fitLines || 0);
    var box = el.parentElement;
    var lo = min, hi = max, best = -1;

    var cap = function (size) {
      // Extra 2px headroom absorbs fractional line-height rounding.
      return el.scrollHeight <= size * LEADING * lines + 2;
    };

    var ok = function (size) {
      return fits(el, size, box) && (!lines || cap(size));
    };

    while (lo <= hi) {
      var mid = Math.floor((lo + hi) / 2);
      if (ok(mid)) { best = mid; lo = mid + 1; } else { hi = mid - 1; }
    }
    if (best < 0) best = min; // floor: overflow beyond, chip decides
    el.style.fontSize = best + "px";

    // Chip only for capped fronts that still overflow at the floor font.
    var chip = el.parentElement.parentElement.querySelector(":scope > .fit-expand");
    if (chip) {
      var overflows = el.scrollHeight > box.clientHeight + 1 ||
                      el.scrollWidth > box.clientWidth + 1 ||
                      (lines && el.scrollHeight > best * LEADING * lines + 2);
      chip.classList.toggle("hidden", !overflows);
    }
  }

  function fitAll() {
    document.querySelectorAll("[data-fit]").forEach(fit);
  }

  // Shared full-text modal used by every .fit-expand chip; built once.
  function ensureModal() {
    if (document.getElementById("ft-modal")) return;
    var wrap = document.createElement("div");
    wrap.id = "ft-modal";
    wrap.className =
      "fixed inset-0 z-50 hidden items-center justify-center p-4";
    wrap.style.background = "rgba(43,18,8,.5)";
    wrap.innerHTML =
      '<div class="box max-w-md w-full max-h-[80vh] flex flex-col gap-3 p-4">' +
      '  <div class="flex items-center justify-between">' +
      '    <h2 class="font-semibold text-lg">Full front text</h2>' +
      '    <button id="ft-modal-close" class="btn bg-white" type="button">✕</button>' +
      '  </div>' +
      '  <div id="ft-modal-text" class="overflow-y-auto text-sm whitespace-pre-line">' +
      '  </div>' +
      '  <div class="bg-sun border-2 border-ink rounded-xl px-3 py-2.5 text-xs font-semibold">' +
      '    💡 Flashcards work best when the front is short — keep large bodies of text like this one on the back.' +
      '  </div>' +
      '</div>';
    document.body.appendChild(wrap);
  }

  // Capture phase so stopping the event here also keeps the outer
  // flip-scene's data-on:click from firing (flip and read are separate).
  document.addEventListener("click", function (e) {
    var chip = e.target.closest(".fit-expand");
    if (chip) {
      e.stopPropagation();
      ensureModal();
      var face = chip.closest(".flip-face");
      var src = face && face.querySelector("[data-fit]");
      var text = document.getElementById("ft-modal-text");
      if (src && text) text.textContent = src.textContent;
      document.getElementById("ft-modal").classList.remove("hidden");
      document.getElementById("ft-modal").classList.add("flex");
      return;
    }
    if (e.target.closest("#ft-modal-close") || e.target.id === "ft-modal") {
      var m = document.getElementById("ft-modal");
      m.classList.add("hidden");
      m.classList.remove("flex");
    }
  }, true);

  // Refit when nodes/text change (datastar patches, $front typing) —
  // but not for our own style writes (attribute mutations are unobserved).
  var mo = new MutationObserver(function () { fitAll(); });
  mo.observe(document.body, {
    childList: true,
    subtree: true,
    characterData: true,
  });
  window.addEventListener("resize", fitAll);
  // datastar patches land async; run a settle pass after each fetch.
  document.addEventListener("datastar-fetch", () => setTimeout(fitAll, 60));
  ensureModal();
  fitAll();
})();
