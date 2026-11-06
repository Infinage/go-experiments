package main

import (
	"embed"
	"flag"
	"io/fs"
	"log"
	"mime"
	"net/http"

	"github.com/infinage/go-experiments/flashcards/internal/store"
	"github.com/infinage/go-experiments/flashcards/internal/web"
)

//go:embed assets
var assets embed.FS

func main() {
	// http.FileServer has no built-in type for web app manifests.
	mime.AddExtensionType(".webmanifest", "application/manifest+json")

	addr := flag.String("addr", ":8080", "listen address")
	dbPath := flag.String("db", "flashcards.db", "SQLite database path")
	flag.Parse()

	db, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("Open db: %v", err)
	}
	defer db.Close()

	sub, err := fs.Sub(assets, "assets")
	if err != nil {
		log.Fatal(err)
	}
	h, err := web.New(db, sub)
	if err != nil {
		log.Fatalf("Init web: %v", err)
	}

	log.Printf("Listening on %s", *addr)
	if err := http.ListenAndServe(*addr, h); err != nil {
		log.Fatalf("Serve fail: %v", err)
	}
}
