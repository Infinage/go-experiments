// Mirrors file picks into the imgPick/audPick signal gates.
//
// The datastar bundle's data-on:change expressions proved unreliable on
// file inputs (they silently no-op for non-capture inputs), so the
// card form gates its picked-media state on imgPick/audPick string
// signals instead, and this script keeps them in sync: it reads the
// file name straight off the input and pushes it into a hidden
// data-bind text input whose change event updates the signal through
// the regular binding path.
(() => {
  const PARTNERS = {
    "front_image": "front_image_pick",
    "front_image_camera": "front_image_pick",
    "front_audio": "front_audio_pick",
  };

  function setPartner(id, value) {
    const partner = document.getElementById(id);
    if (!partner || partner.value === value) return;
    partner.value = value;
    partner.dispatchEvent(new Event("input", {bubbles: true}));
    partner.dispatchEvent(new Event("change", {bubbles: true}));
  }

  document.addEventListener("change", (e) => {
    const partnerId = PARTNERS[e.target.name];
    if (!partnerId) return;
    setPartner(partnerId, e.target.files?.[0]?.name || "");
  });
})();
