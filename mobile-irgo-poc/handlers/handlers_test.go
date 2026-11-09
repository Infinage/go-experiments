package handlers_test

import (
	"testing"

	"mobile-irgo-poc/app"
	irgotest "github.com/stukennedy/irgo/pkg/testing"
)

func TestHomePage(t *testing.T) {
	r := app.NewRouter()
	client := irgotest.NewClient(r.Handler())

	// Must render via html/template and show the seeded SQLite row.
	resp := client.Get("/")
	resp.AssertOK(t)
	resp.AssertHTML(t)
	resp.AssertContains(t, "sqlite-roundtrip-ok")
}

func TestHomePageDatastar(t *testing.T) {
	r := app.NewRouter()
	client := irgotest.NewClient(r.Handler())

	// Datastar SSE request patches the html/template fragment.
	resp := client.Datastar().Get("/api/card")
	resp.AssertOK(t)
	resp.AssertSSE(t)
	resp.AssertContains(t, "sqlite-roundtrip-ok")
}
