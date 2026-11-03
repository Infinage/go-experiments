package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/infinage/go-experiments/flashcards/src/pkg"
)

//go:embed assets/html
var htmlFS embed.FS

// staticFS holds the publicly servable assets. Templates in htmlFS are
// deliberately excluded: they are internal build artifacts, not pages.
//go:embed assets/css assets/js
var staticFS embed.FS

func main() {
	st, err := flashcards.NewStore("flashcards.db")
	if err != nil {
		log.Fatal(err)
	}
	if err := st.Init(context.Background()); err != nil {
		log.Fatal(err)
	}

	// Demo seeding: with FLASHCARDS_SEED_DEMO=1, an empty database is filled
	// with the documented dummy accounts (see README) for manual testing.
	// Databases that already have users are left untouched.
	if os.Getenv("FLASHCARDS_SEED_DEMO") == "1" {
		seeded, err := st.SeedDemoData(context.Background())
		if err != nil {
			log.Fatal(err)
		}
		if seeded {
			log.Println("seeded demo accounts (demo, reviewer, test) — see README")
		}
	}

	html, err := fs.Sub(htmlFS, "assets/html")
	if err != nil {
		log.Fatal(err)
	}
	// The embed patterns include the assets/ prefix; drop it so /assets/css/…
	// maps to css/… under the file server.
	static, err := fs.Sub(staticFS, "assets")
	if err != nil {
		log.Fatal(err)
	}
	srv, err := flashcards.NewServer(st, html, static)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("listening on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", srv.Handler()))
}
