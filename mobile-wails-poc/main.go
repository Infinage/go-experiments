// mobile-wails-poc — throwaway feasibility probe.
// Go backend (SQLite + html/template + Datastar SSE) inside a Wails v3 webview.
// See README.md for goals and success criteria.
package main

import (
	"embed"
	"log"
	"os"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"

	"mobile-wails-poc/internal/db"
	"mobile-wails-poc/internal/web"
)

//go:embed all:assets
var assetsFS embed.FS

func main() {
	if runtime.GOOS == "android" {
		// Go's log output goes to stderr, which is lost on Android. Route it to
		// the app's own cache dir so a debug build can be inspected with
		// `adb shell run-as com.wails.app cat cache/poc-go.log`.
		if f, err := os.OpenFile("/data/data/com.wails.app/cache/poc-go.log",
			os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
			log.SetOutput(f)
		}
	}

	database, err := db.Open(envDbPath())
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	handler, err := web.New(database, assetsFS)
	if err != nil {
		log.Fatal(err)
	}

	app := application.New(application.Options{
		Name:   "mobile-wails-poc",
		Assets: application.AssetOptions{Handler: handler},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:             "Wails mobile POC",
		BackgroundColour:  application.NewRGB(27, 38, 54),
		URL:               "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// envDbPath lets the runbook force a database location; empty picks a per-OS default.
func envDbPath() string {
	if p := os.Getenv("POC_DB_PATH"); p != "" {
		return p
	}
	if runtime.GOOS == "android" {
		// os.UserCacheDir and /tmp are unwritable for an app uid on Android;
		// use the app's own files dir (matches applicationId com.wails.app).
		return "/data/data/com.wails.app/files/poc.db"
	}
	return ""
}
