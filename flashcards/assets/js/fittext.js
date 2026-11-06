// Dynamic text fitting: scales [data-fit] blocks to fill their container,
// so long text and media can coexist inside the fixed-height flip cards.
(function () {
  "use strict";

  function fits(el, size) {
    el.style.fontSize = size + "px";
    const box = el.parentElement;
    return el.scrollHeight <= box.clientHeight &&
      el.scrollWidth <= box.clientWidth;
  }

  function fit(el) {
    const min = +(el.dataset.fitMin || 12);
    const max = +(el.dataset.fitMax || 48);
    let lo = min, hi = max, best = min;
    while (lo <= hi) {
      const mid = Math.floor((lo + hi) / 2);
      if (fits(el, mid)) {
        best = mid;
        lo = mid + 1;
      } else {
        hi = mid - 1;
      }
    }
    el.style.fontSize = best + "px";
  }

  function fitAll() {
    document.querySelectorAll("[data-fit]").forEach(fit);
  }

  // Refit when nodes/text change (datastar patches, $front typing) —
  // but not for our own style writes (attribute mutations are unobserved).
  const mo = new MutationObserver(() => fitAll());
  mo.observe(document.body, {
    childList: true,
    subtree: true,
    characterData: true,
  });
  window.addEventListener("resize", fitAll);
  // datastar patches land async; run a settle pass after each fetch.
  document.addEventListener("datastar-fetch", () => setTimeout(fitAll, 60));
  fitAll();
})();
