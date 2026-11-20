// @ts-check
"use strict";

const problems = document.querySelector(".problems");
if (!problems) throw new Error("Unable to find problems tag");

/**
  * Submits the answer
  * @param {HTMLInputElement} ele
  * @returns {void}
  */
function submit(ele) {
  ele.disabled = true;
  ele.parentElement?.classList.toggle(ele.dataset.answer === ele.value? "correct": "incorrect");

  // Display victory banner :)
  if (!problems || document.querySelectorAll("input:enabled").length) return;
  problems.innerHTML = "";
  const div = document.createElement("div");
  div.setAttribute("class", "final-message");
  div.innerText = "🔥 Congrats 🔥";
  const now = (new Date()).toDateString();
  const span =  document.createElement("span");
  span.setAttribute("class", "timestamp");
  span.innerText = now

  div.appendChild(span);
  problems.appendChild(div);
}

problems.addEventListener("focusout", evt => {
  if (!(evt.target instanceof HTMLInputElement)) return;
  evt.target.value && submit(evt.target);
});

problems.addEventListener("keydown", evt => {
  if (!(evt instanceof KeyboardEvent)) return;
  if (!(evt.target instanceof HTMLInputElement)) return;
  evt.key === "Enter" && submit(evt.target);
});

problems.addEventListener("keyup", evt => {
  if (!(evt instanceof KeyboardEvent)) return;
  if (!(evt.target instanceof HTMLInputElement)) return;
  if (evt.key >= '0' && evt.key <= '9') {
    evt.target.selectionStart = 0;
    evt.target.selectionEnd = 0;
  }
});

problems.addEventListener("animationend", async evt => {
  // delete incorrect problem
  if (!evt.target) return;
  problems.removeChild(/** @type Node */ (evt.target));

  const iterStr = /** @type {HTMLElement} */ (evt.target).dataset.iteration;
  const iters = Number(iterStr);

  // Load new problem(s)
  const res = await fetch(`/more?iteration=${iters}`);
  const raw = await res.text();
  if (!res.ok) {
    console.error(`fetch fail: ${raw}`);
    return;
  }
  problems.insertAdjacentHTML("beforeend", raw);
});
