package utils

import (
	"html/template"
	"net/http"
	"path/filepath"
)

func RenderHtml(w http.ResponseWriter, r *http.Request, tmplName string, data interface{}) {
	tmplRoot := filepath.Join("internal", "web", "templates")
	tmplPath := filepath.Join(tmplRoot, tmplName) + ".html"
	templates := template.Must(template.ParseFiles(filepath.Join(tmplRoot, "base.html"), filepath.Join(tmplRoot, "layout.html"), tmplPath))
	err := templates.Execute(w, data)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}
