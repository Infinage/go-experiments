// Retro Flashcards service worker: offline shell for the PWA demo.
// Cache-first for static assets, network-first for pages with a cached
// shell fallback.
const VERSION = "v8";
const SHELL = [`/`, `/assets/css/index.css`, `/assets/js/datastar.js`,
  `/assets/fonts/poppins-400.woff2`, `/assets/fonts/poppins-500.woff2`,
  `/assets/fonts/poppins-600.woff2`, `/assets/fonts/poppins-700.woff2`,
  `/assets/static/manifest.webmanifest`, `/assets/static/icon.svg`];

self.addEventListener("install", (e) => {
  e.waitUntil(caches.open(VERSION).then((c) => c.addAll(SHELL)));
  self.skipWaiting();
});

self.addEventListener("activate", (e) => {
  e.waitUntil(caches.keys().then((keys) =>
    Promise.all(keys.filter((k) => k !== VERSION)
      .map((k) => caches.delete(k)))));
  self.clients.claim();
});

self.addEventListener("fetch", (e) => {
  const url = new URL(e.request.url);
  if (e.request.method !== "GET" || url.origin !== location.origin) return;
  if (url.pathname.startsWith("/assets/")) {
    // Cache-first: static assets are immutable between deploys.
    e.respondWith(caches.match(e.request).then((hit) => hit ||
      fetch(e.request).then((res) => {
        const copy = res.clone();
        caches.open(VERSION).then((c) => c.put(e.request, copy));
        return res;
      })));
    return;
  }
  // Network-first for pages: fresh data when online, shell when offline.
  if (e.request.mode === "navigate") {
    e.respondWith(fetch(e.request).then((res) => {
      const copy = res.clone();
      caches.open(VERSION).then((c) => c.put(e.request, copy));
      return res;
    }).catch(() => caches.match(e.request)
      .then((hit) => hit || caches.match("/"))));
  }
});
