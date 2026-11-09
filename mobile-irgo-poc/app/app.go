// Package app provides the shared application setup.
// This is imported by both main.go (desktop) and mobile/mobile.go (mobile).
package app

import (
	"database/sql"
	"io/fs"
	"log"
	"net/http"

	"mobile-irgo-poc/handlers"
	"mobile-irgo-poc/internal/db"
	"mobile-irgo-poc/internal/web"
	"mobile-irgo-poc/static"
	"github.com/stukennedy/irgo/pkg/router"
)

// DB is opened once at router construction; the POC treats it as app-global.
var DB *sql.DB

// NewRouter creates a new router with all app routes configured.
func NewRouter() *router.Router {
	d, err := db.Open()
	if err != nil {
		log.Fatalf("db open failed (toolchain/driver probe): %v", err)
	}
	DB = d

	r := router.New()

	// Serve embedded static files (works for both web and mobile)
	staticFS, _ := fs.Sub(static.Files, ".")
	r.Static("/static", http.FS(staticFS))

	// Home page — html/template only, no templ anywhere in this path.
	r.GET("/", func(ctx *router.Context) (string, error) {
		row, err := db.ReadSeed(d)
		if err != nil {
			return "", err
		}
		return web.Page(web.Vars{RowValue: row, DBPath: db.Path()})
	})

	// Mount SSE handlers
	handlers.Mount(r, d)

	return r
}
