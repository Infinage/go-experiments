// seeddb writes a SQLite db pre-loaded with the flashcards sample deck, for
// pre-seeding test devices (adb push — see README "Mobile build").
package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/infinage/go-experiments/flashcards/internal/store"
	"github.com/infinage/go-experiments/flashcards/internal/web"
)

func main() {
	out := flag.String("out", "tmp/seed/flashcards.db", "output db path")
	flag.Parse()

	dir := filepath.Dir(*out)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatal(err)
		}
	}
	_ = os.Remove(*out + "-shm")
	_ = os.Remove(*out + "-wal")

	db, err := store.Open(*out)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	n := 0
	for _, c := range web.SampleCards() {
		if _, err := db.CreateCard(c[0], c[1], c[2]); err != nil {
			log.Fatal(err)
		}
		n++
	}
	log.Printf("seeded %d cards into %s", n, *out)
}
