package utils

import (
	"html/template"
	"net/http"
	"webhook-tester/internal/web/templates"
)

func RenderHtml(w http.ResponseWriter, r *http.Request, tmplName string, data interface{}) {
	files := []string{
		"base.html",
		"layout.html",
		tmplName + ".html",
	}
	tmpl := template.Must(template.ParseFS(templates.Templates, files...))

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func RenderHtmlWithoutLayout(w http.ResponseWriter, r *http.Request, tmplName string, data interface{}) {
	files := []string{
		"base.html",
		tmplName + ".html",
	}
	tmpl := template.Must(template.ParseFS(templates.Templates, files...))

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}
