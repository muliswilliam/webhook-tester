package handlers

import (
	"net/http"
	"webhook-tester/internal/web/view"
)

type LegalHandler struct{}

func NewLegalHandler() *LegalHandler {
	return &LegalHandler{}
}

func (h *LegalHandler) PrivacyPolicy(w http.ResponseWriter, r *http.Request) {
	view.RenderHTMLWithoutLayout(w, r, "policy", nil)
}

func (h *LegalHandler) TermsAndConditions(w http.ResponseWriter, r *http.Request) {
	view.RenderHTMLWithoutLayout(w, r, "terms", nil)
}
