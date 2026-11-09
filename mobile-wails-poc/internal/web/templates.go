package web

import (
	"html/template"
	"io"
	"io/fs"
)

// pageTemplates parses the two POC templates out of the embedded assets.
type pageTemplates struct {
	index    *template.Template
	fragment *template.Template
}

func parseTemplates(assets fs.FS) (*pageTemplates, error) {
	index, err := template.ParseFS(assets, "assets/index.html")
	if err != nil {
		return nil, err
	}
	fragment, err := template.ParseFS(assets, "assets/fragment.html")
	if err != nil {
		return nil, err
	}
	return &pageTemplates{index: index, fragment: fragment}, nil
}

// execute renders the named page; both templates share the view struct.
func (p *pageTemplates) execute(w io.Writer, name string, data view) error {
	t := p.index
	if name == "fragment.html" {
		t = p.fragment
	}
	return t.Execute(w, data)
}
