package main

import (
	"embed"
	"errors"
	"html/template"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"net/http"
)

//go:embed assets/*
var assets embed.FS

// handleAPIMerge parses the incoming request containing images, merges the
// images into a PDF and serves the PDF for download.
func handleAPIMerge(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Error parsing form: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()

	files := r.MultipartForm.File["images"]
	if len(files) == 0 {
		http.Error(w, "No files uploaded", http.StatusBadRequest)
		return
	}

	var images []image.Image
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			http.Error(w, "Error reading file contents: "+err.Error(),
				http.StatusInternalServerError)
			return
		}

		image2Merge, _, err := image.Decode(file)
		file.Close()
		if err != nil {
			http.Error(w, "Error decoding file content as image: "+err.Error(),
				http.StatusBadRequest)
			return
		}

		// Rewrite all images to standard 8 bit RGBA canvas
		bounds := image2Merge.Bounds()
		img8bit := image.NewRGBA(bounds)
		draw.Draw(img8bit, bounds, image2Merge, bounds.Min, draw.Src)
		images = append(images, img8bit)
	}

	pdfWriter, err := images2pdf(images)
	if err != nil {
		http.Error(w, "Merge PDF fail: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=merged.pdf")
	_, err = pdfWriter.WriteTo(w)
	if err != nil {
		log.Println("File write to response failed: ", err)
	}
}

func main() {
	templ, err := template.ParseFS(assets, "assets/index.html")
	if err != nil {
		log.Fatal("Template parse: ", err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /assets/", http.FileServerFS(assets))
	mux.HandleFunc("POST /api/merge", handleAPIMerge)
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		err := templ.Execute(w, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	err = http.ListenAndServe(":8080", mux)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal("Server error: ", err)
	}
}
