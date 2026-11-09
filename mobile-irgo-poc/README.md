# mobile-irgo-poc

Validate that a Go backend serving HTML over **datastar/SSE** runs correctly inside the
**irgo** Android runtime, backed by a real **SQLite** database. Throwaway code is expected.
Scope is deliberately minimal — this is a feasibility probe, not an app.

## Background facts (from irgo.dev/docs)

- irgo, MIT, repo `github.com/stukennedy/irgo`; docs at `irgo.dev/docs`.
- Mobile architecture ("virtual HTTP"): the WebView makes `irgo://`-scheme requests; a native
  bridge intercepts them and routes to a Go runtime compiled via gomobile. **No network
  sockets on mobile at all.** Handlers execute in-process through an HTTP adapter
  (`core.Request`/`core.Response` ↔ `net/http`) in memory.
- Frontend JS is datastar; the framework's stated pattern is
  `User Interaction → Datastar Request → Go Handler → SSE Response → DOM Morph`.
- Handler API: `ctx.ReadSignals(&signals)` for inbound datastar signals, `ctx.SSE().PatchTempl(...)`
  for responses, chi-style router behind `github.com/stukennedy/irgo/pkg/router`.
- Entry (`//go:build !desktop`): `mobile.Initialize()` + `mobile.SetHandler(r.Handler())`;
  an extra `serve` arg runs a plain HTTP dev server on `:8080` for browser testing.
- Static assets: standard `embed.FS` + `r.StaticFS("/static", ...)`.

## Test goals (in priority order)

1. **Does irgo work WITHOUT templ?** The docs show templ as the view layer
   (`ctx.SSE().PatchTempl(...)`), but the question is whether standard `html/template` events
   work — i.e. render with `html/template`, serve the same HTML over the SSE adapter, and see
   whether datastar morphs it on the client. If templ is hard-required by the adapter (or by
   SSE response types), that is a blocker-level fact and it gets recorded in `RESULTS.md`.
2. **Does datastar/SSE stream unbuffered through the virtual HTTP bridge on Android?**
   In-process request/response adapters can buffer long-lived streams.
3. **Does SQLite work end-to-end?** Real read of one record from one real table (no
   hello-world placeholder), rendered in the viewport.
4. Toolchain sanity (`gomobile init` path, first-run build time, emulator boot, debug attach).

## Concrete SQLite test

- `internal/db`: open SQLite (pure-Go `modernc.org/sqlite`), create one small table
  (e.g. `dict(value TEXT)`), seed one row, read it back via `SELECT value FROM dict LIMIT 1`.
- Display the row's text in the viewport on first render (server-side).
- Also wire the same read through a datastar SSE fragment triggered by a button click, so the
  identical read path is exercised over the SSE channel being tested in goal 2.

## Success criteria

| # | Check | Pass |
| --- | --- | --- |
| I1 | `irgo run android` installs and launches on the emulator | app runs, no crash |
| I2 | Plain `html/template` renders + SSE-morphs without templ involved | fragment morphs against an `html/template`-rendered page |
| I3 | SQLite row value is displayed | expected text visible |
| I4 | datastar SSE fragment updates on button click | DOM morphs without full page reload |
| I5 | `chrome://inspect` attach works | DevTools shows our DOM |
| I6 | If I4 buffers over the virtual adapter, is there a supported way to flush (adapter config or documented pattern)? | fallback documented |

## Layout

```
mobile-irgo-poc/
├── README.md
├── main.go            (!desktop entry; mobile.Initialize + SetHandler; serve-arg dev server)
├── internal/db/       (sqlite open/seed/read)
├── internal/web/      (handler, html/template templates, SSE fragment)
├── static/            (datastar bundle)
└── RESULTS.md         (fill in as you go)
```

Keep scratch discipline: don't polish, record what broke and what passed in
`RESULTS.md` as you go.
