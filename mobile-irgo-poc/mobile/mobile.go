// Package mobile provides gomobile bindings for iOS and Android.
// This package re-exports the irgo mobile bridge functions and initializes
// your app's routes when the mobile app starts.
package mobile

import (
	"log"
	"net/http"

	irgomobile "github.com/stukennedy/irgo/mobile"
	"mobile-irgo-poc/app"
	"mobile-irgo-poc/internal/db"
)

var appRouter http.Handler

// Response is a gomobile-compatible response type.
// This is defined here so gomobile can export it.
type Response struct {
	Status  int
	Headers string
	Body    []byte
}

// BodyString returns the body as a string.
func (r *Response) BodyString() string {
	return string(r.Body)
}

// SetDBPath points the POC database at an app-writable location.
// Must be called by native code BEFORE Initialize — Android's cwd is
// not writable, so the default "irgo.db" relative path cannot be used.
func SetDBPath(p string) {
	db.SetPath(p)
}

// Initialize sets up the mobile bridge and app routes.
// Called automatically by native code at app startup.
func Initialize() {
	log.Println("poc-init: start")
	// Set up app routes using shared router setup
	r := app.NewRouter()
	appRouter = r.Handler()
	log.Println("poc-init: router built")

	// Initialize the irgo bridge with our handler
	irgomobile.SetHandler(appRouter)
	log.Println("poc-init: handler set")
	irgomobile.Initialize()
	log.Println("poc-init: done")
}

// HandleRequest processes an HTTP request from the WebView.
// This is called by native code (Swift/Kotlin) for each request.
func HandleRequest(method, url, headers string, body []byte) *Response {
	log.Printf("poc-hr: %s %s", method, url)
	coreResp := irgomobile.HandleRequest(method, url, headers, body)
	log.Printf("poc-hr: %d %s body=%d", coreResp.Status, url, len(coreResp.Body))
	return &Response{
		Status:  coreResp.Status,
		Headers: coreResp.Headers,
		Body:    coreResp.Body,
	}
}

// HandleRequestSimple processes a simple GET request.
func HandleRequestSimple(method, url string) *Response {
	coreResp := irgomobile.HandleRequestSimple(method, url)
	return &Response{
		Status:  coreResp.Status,
		Headers: coreResp.Headers,
		Body:    coreResp.Body,
	}
}

// RenderInitialPage returns the initial HTML for the WebView.
func RenderInitialPage() string {
	return irgomobile.RenderInitialPage()
}

// IsReady returns true if the bridge is initialized.
func IsReady() bool {
	return irgomobile.IsReady()
}

// Shutdown cleans up the bridge.
func Shutdown() {
	irgomobile.Shutdown()
}
