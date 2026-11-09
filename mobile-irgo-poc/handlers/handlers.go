package handlers

import (
	"database/sql"
	"sync/atomic"

	"mobile-irgo-poc/internal/db"
	"mobile-irgo-poc/internal/web"
	"github.com/stukennedy/irgo/pkg/router"
)

var patchesReceived atomic.Int64

// Mount registers all handlers on the router.
func Mount(r *router.Router, d *sql.DB) {
	// Re-read the SQLite row and patch it back as an html/template fragment,
	// converted via SSE with zero templ involvement.
	r.DSGet("/api/card", func(ctx *router.Context) error {
		value, err := db.ReadSeed(d)
		if err != nil {
			return ctx.SSE().ConsoleError(err)
		}

		sse := ctx.SSE()
		frag, err := web.CardFragment(web.Vars{RowValue: value, DBPath: db.Path()})
		if err != nil {
			return sse.ConsoleError(err)
		}

		// Increment counter signal so the page can prove the channel is alive,
		// and report the db path so the on-device cwd/question self-answers.
		n := patchesReceived.Add(1)
		if err := sse.PatchSignals(map[string]any{
			"sqliteRows": n,
			"dbPath": db.Path(),
		}); err != nil {
			return err
		}
		return sse.PatchHTMLByID("row-out", frag)
	})
}
