package web

import (
	"html/template"
	"net/http"
	"path/filepath"
)

func Render(w http.ResponseWriter, tmplName string, data interface{}) {
	tmplRoot := filepath.Join("internal", "web", "templates")
	tmplPath := filepath.Join(tmplRoot, tmplName) + ".html"
	templates := template.Must(template.ParseFiles(filepath.Join(tmplRoot, "base.html"), tmplPath))
	err := templates.Execute(w, data)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}
