package handlers

import (
	"net/http"
	"time"
	"webhook-tester/internal/utils"
)

type LegalHandler struct{}

func NewLegalHandler() *LegalHandler {
	return &LegalHandler{}
}

func (h *LegalHandler) PrivacyPolicy(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Year int
	}{
		Year: time.Now().Year(),
	}
	utils.RenderHtmlWithoutLayout(w, r, "policy", data)
}

func (h *LegalHandler) TermsAndConditions(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Year int
	}{
		Year: time.Now().Year(),
	}
	utils.RenderHtmlWithoutLayout(w, r, "terms", data)
}
