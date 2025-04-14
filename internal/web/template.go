package web

import (
	"html/template"
	"path/filepath"
)

var Templates = template.Must(template.ParseGlob(filepath.Join("internal", "web", "templates", "*.html")))
