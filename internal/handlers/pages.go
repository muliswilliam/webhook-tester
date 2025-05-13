package handlers

import (
	"net/http"
	"time"
	"webhook-tester/internal/utils"
)

func (h *Handler) PrivacyPolicy(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Year int
	}{
		Year: time.Now().Year(),
	}
	utils.RenderHtmlWithoutLayout(w, r, "policy", data)
}

func (h *Handler) TermsAndConditions(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Year int
	}{
		Year: time.Now().Year(),
	}
	utils.RenderHtmlWithoutLayout(w, r, "terms", data)
}

func (h *Handler) Landing(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Year int
	}{
		Year: time.Now().Year(),
	}
	utils.RenderHtml(w, r, "landing", data)
}