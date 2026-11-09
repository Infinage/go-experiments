// Package web renders the POC pages with html/template — deliberately NOT templ.
// Goal I2: prove datastar morphs fragments against an html/template-rendered page.
package web

import (
	"embed"
	"fmt"
	"html/template"
)

//go:embed tpl/*.html
var tplFS embed.FS

var tpl = template.Must(template.ParseFS(tplFS, "tpl/*.html"))

// Vars is the data passed to every template.
type Vars struct {
	RowValue string
	DBPath   string
}

// Page renders the full initial page as an HTML string.
func Page(v Vars) (string, error) {
	return tmplToString(tpl.Lookup("page.html"), v)
}

// CardFragment renders the SSE-patchable #row-out fragment.
func CardFragment(v Vars) (string, error) {
	return tmplToString(tpl.Lookup("cardfragment.html"), v)
}

func tmplToString(t *template.Template, v Vars) (string, error) {
	var b []byte
	w := &sliceWriter{b: &b}
	if t == nil {
		return "", fmt.Errorf("web: nil template")
	}
	err := t.Execute(w, v)
	return string(b), err
}

type sliceWriter struct{ b *[]byte }

func (s *sliceWriter) Write(p []byte) (int, error) { *s.b = append(*s.b, p...); return len(p), nil }
