package main

import (
	"embed"
	"flag"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/infinage/go-experiments/flashcards/internal/store"
	"github.com/infinage/go-experiments/flashcards/internal/web"
)

//go:embed assets
var assets embed.FS

// assetHandler is the web handler handed to the Wails asset server on
// Android; android_jni.go's nativeServeAssetFull drives it directly so the
// Java layer gets the real status/Content-Type (wails' own serveAsset drops
// headers and guesses MIME from file extension). Nil on desktop only.
var assetHandler http.Handler

func main() {
	// http.FileServer has no built-in type for web app manifests.
	mime.AddExtensionType(".webmanifest", "application/manifest+json")

	addr := flag.String("addr", ":8080", "listen address")
	dbPath := flag.String("db", "flashcards.db", "SQLite database path")
	flag.Parse()

	// On Android the app runs inside a Wails webview: no shell, no sockets for
	// the app UID, and stderr is lost — so ignore the flags and force the
	// app-private db path, and route Go logs to a file that can be fetched
	// with `task apk:go-log` (adb shell run-as ... cat cache/...).
	if runtime.GOOS == "android" {
		// Fresh installs create the app dirs lazily; SQLite does not create
		// parent dirs itself (SQLITE_CANTOPEN 14 otherwise).
		os.MkdirAll("/data/data/com.infinage.flashcards/files", 0o755)
		// os.TempDir() defaults to /data/local/tmp on Android, which is adb-shell
		// territory and not app-writable — media staging (newStaging) must live in
		// the app-private cache dir instead.
		os.MkdirAll("/data/data/com.infinage.flashcards/cache", 0o700)
		os.Setenv("TMPDIR", "/data/data/com.infinage.flashcards/cache")
		if f, err := os.OpenFile(
			"/data/data/com.infinage.flashcards/cache/flashcards-go.log",
			os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644,
		); err == nil {
			defer f.Close()
			log.SetOutput(f)
		}
		*dbPath = "/data/data/com.infinage.flashcards/files/flashcards.db"
	}

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

	if runtime.GOOS == "android" {
		// Android has no app-visible ports: hand the web handler to the Wails
		// asset server and drive the webview event loop instead of blocking in
		// http.ListenAndServe.
		assetHandler = h // visible to android_jni.go for nativeServeAssetFull
		app := application.New(application.Options{
			Name:   "flashcards",
			Assets: application.AssetOptions{Handler: h},
		})
		app.Window.NewWithOptions(application.WebviewWindowOptions{
			Title:            "Flashcards",
			BackgroundColour: application.NewRGB(255, 255, 255),
			URL:              "/",
		})
		if err := app.Run(); err != nil {
			log.Fatalf("Wails run: %v", err)
		}
		return
	}

	log.Printf("Listening on %s", *addr)
	if err := http.ListenAndServe(*addr, h); err != nil {
		log.Fatalf("Serve fail: %v", err)
	}
}
