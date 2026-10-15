package main

import (
	"bytes"
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"image/png"
	"log"
	"net/http"

	queens "github.com/infinage/linkedin/queens/src/pkg"
	"github.com/starfederation/datastar-go/datastar"
)

//go:embed assets/*
var assets embed.FS

func routes(tmpl *template.Template) *http.ServeMux {
	mux := http.NewServeMux()

	// Static endpoints
	mux.Handle("GET /assets/", http.FileServerFS(assets))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		err := tmpl.ExecuteTemplate(w, "index.html", nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	// API endpoints
	mux.HandleFunc("POST /api/solve", func(w http.ResponseWriter, r *http.Request) {
		const maxBytes = 2 << 24
		err := r.ParseMultipartForm(maxBytes)
		if err != nil {
			http.Error(w, fmt.Sprintf("Parse form: %v", err), http.StatusBadRequest)
			return
		}

		file, _, err := r.FormFile("upload")
		if err != nil {
			http.Error(w, fmt.Sprintf("Upload file: %v", err), http.StatusBadRequest)
			return
		}

		sse := datastar.NewSSE(w, r)
		data := map[string]string{"_error": "", "_result": ""}

		img, err := png.Decode(file)
		if err != nil {
			data["_error"] = fmt.Sprintf("png decode: %v", err)
			sse.MarshalAndPatchSignals(data)
			return
		}

		b, err := queens.NewBoardFromImage(img)
		if err != nil {
			data["_error"] = fmt.Sprintf("NewBoardFromImage: %v", err)
			sse.MarshalAndPatchSignals(data)
			return
		}

		if err = b.Solve(); err != nil {
			data["_error"] = fmt.Sprintf("solve board: %v", err)
			sse.MarshalAndPatchSignals(data)
			return
		}

		oimg, err := b.Paint()
		if err != nil {
			data["_error"] = fmt.Sprintf("paint: %v", err)
			sse.MarshalAndPatchSignals(data)
			return
		}

		var buf bytes.Buffer
		err = png.Encode(&buf, oimg)
		if err != nil {
			data["_error"] = fmt.Sprintf("png encode: %v", err)
			sse.MarshalAndPatchSignals(data)
			return
		}

		bufBase64 := base64.StdEncoding.EncodeToString(buf.Bytes())
		data["_result"] = "data:image/png;base64," + bufBase64
		sse.MarshalAndPatchSignals(data)
	})

	return mux
}

func main() {
	tmpl, err := template.ParseFS(assets, "assets/index.html")
	if err != nil {
		log.Fatal("Template Init failed: ", err)
	}

	mux := routes(tmpl)
	if err := http.ListenAndServe(":8080", mux); err != nil &&
		!errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
