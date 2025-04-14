package handlers

import (
	"net/http"
	"webhook-tester/internal/web"
)

func Home(w http.ResponseWriter, r *http.Request) {
	err := web.Templates.ExecuteTemplate(w, "base.html", nil)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}
