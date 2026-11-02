package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"

	"github.com/infinage/go-experiments/flashcards/src/pkg"
)

//go:embed assets/html
var htmlFS embed.FS

func main() {
	st, err := flashcards.NewStore("flashcards.db")
	if err != nil {
		log.Fatal(err)
	}
	if err := st.Init(context.Background()); err != nil {
		log.Fatal(err)
	}

	html, err := fs.Sub(htmlFS, "assets/html")
	if err != nil {
		log.Fatal(err)
	}
	srv, err := flashcards.NewServer(st, html)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("listening on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", srv.Handler()))
}
