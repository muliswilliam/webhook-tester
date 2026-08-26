package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLegalHandler_PrivacyPolicy(t *testing.T) {
	h := NewLegalHandler()
	req := httptest.NewRequest(http.MethodGet, "/privacy", nil)
	rec := httptest.NewRecorder()

	h.PrivacyPolicy(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Privacy Policy")
}

func TestLegalHandler_TermsAndConditions(t *testing.T) {
	h := NewLegalHandler()
	req := httptest.NewRequest(http.MethodGet, "/terms", nil)
	rec := httptest.NewRecorder()

	h.TermsAndConditions(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Terms & Conditions")
}
