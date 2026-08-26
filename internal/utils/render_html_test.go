package utils

import (
	"html/template"
	"net/http/httptest"
	"testing"
	"time"

	"webhook-tester/internal/models"

	"github.com/stretchr/testify/assert"
)

func newTestWebhook() models.Webhook {
	contentType := "application/json"
	payload := `{"message":"ok"}`
	return models.Webhook{
		ID:            "wh-1",
		Title:         "My webhook",
		ResponseCode:  200,
		ResponseDelay: 0,
		ContentType:   &contentType,
		Payload:       &payload,
		NotifyOnEvent: false,
		UserID:        0,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Requests: []models.WebhookRequest{
			{
				ID:         "req-1",
				WebhookID:  "wh-1",
				Method:     "POST",
				Body:       "{}",
				ReceivedAt: time.Now(),
			},
		},
	}
}

func TestRenderHtmlHomeHappyPath(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	webhook := newTestWebhook()

	data := struct {
		CSRFField       template.HTML
		User            models.User
		Webhooks        []models.Webhook
		Webhook         models.Webhook
		ResponseHeaders string
		RequestsCount   uint
		Domain          string
		Year            int
	}{
		CSRFField:       template.HTML(`<input type="hidden">`),
		User:            models.User{},
		Webhooks:        []models.Webhook{webhook},
		Webhook:         webhook,
		ResponseHeaders: "",
		RequestsCount:   uint(len(webhook.Requests)),
		Domain:          "example.com",
		Year:            2026,
	}

	RenderHtml(w, r, "home", data)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "My webhook")
}

func TestRenderHtmlRequestHappyPath(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	webhook := newTestWebhook()
	req := webhook.Requests[0]

	data := struct {
		ID        string
		Year      int
		User      models.User
		Webhooks  []models.Webhook
		Webhook   *models.Webhook
		Request   *models.WebhookRequest
		CSRFField template.HTML
	}{
		ID:        req.ID,
		Year:      2026,
		User:      models.User{},
		Webhooks:  []models.Webhook{webhook},
		Webhook:   &webhook,
		Request:   &req,
		CSRFField: template.HTML(`<input type="hidden">`),
	}

	RenderHtml(w, r, "request", data)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "req-1")
}

func TestRenderHtmlUnknownTemplatePanics(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	assert.Panics(t, func() {
		RenderHtml(w, r, "does-not-exist", nil)
	})
}

func TestRenderHtmlWithoutLayoutRegister(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/register", nil)

	data := struct {
		CSRFField template.HTML
		Error     string
		FullName  string
		Email     string
		Password  string
	}{
		CSRFField: template.HTML(`<input type="hidden">`),
	}

	RenderHtmlWithoutLayout(w, r, "register", data)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "Create account")
}

func TestRenderHtmlWithoutLayoutLogin(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/login", nil)

	data := struct {
		CSRFField template.HTML
		Error     string
	}{
		CSRFField: template.HTML(`<input type="hidden">`),
		Error:     "bad credentials",
	}

	RenderHtmlWithoutLayout(w, r, "login", data)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "bad credentials")
}

func TestRenderHtmlWithoutLayoutForgotPassword(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/forgot-password", nil)

	data := struct {
		CSRFField template.HTML
		Error     string
		Success   bool
	}{
		CSRFField: template.HTML(`<input type="hidden">`),
		Success:   true,
	}

	RenderHtmlWithoutLayout(w, r, "forgot-password", data)

	assert.Equal(t, 200, w.Code)
}

func TestRenderHtmlWithoutLayoutResetPassword(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/reset-password", nil)

	data := struct {
		CSRFField       template.HTML
		Error           string
		Token           string
		Password        string
		ConfirmPassword string
	}{
		CSRFField: template.HTML(`<input type="hidden">`),
		Token:     "abc123",
	}

	RenderHtmlWithoutLayout(w, r, "reset-password", data)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "abc123")
}

func TestRenderHtmlWithoutLayoutPolicy(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/policy", nil)

	data := struct {
		Year int
	}{Year: 2026}

	RenderHtmlWithoutLayout(w, r, "policy", data)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "2026")
}

func TestRenderHtmlWithoutLayoutTerms(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/terms", nil)

	data := struct {
		Year int
	}{Year: 2026}

	RenderHtmlWithoutLayout(w, r, "terms", data)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "2026")
}

// Execution begins writing static markup from base.html before any field
// is evaluated, so httptest.ResponseRecorder's status code is already locked
// to 200 by the time template.Execute hits the bad field access below and
// calls http.Error. These tests only assert that the error branch executes
// without panicking; the recorded status code cannot be used as a signal.
func TestRenderHtmlExecuteError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	assert.NotPanics(t, func() {
		RenderHtml(w, r, "home", 5)
	})
}

func TestRenderHtmlWithoutLayoutExecuteError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/register", nil)

	assert.NotPanics(t, func() {
		RenderHtmlWithoutLayout(w, r, "register", 5)
	})
}

func TestRenderHtmlWithoutLayoutUnknownTemplatePanics(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	assert.Panics(t, func() {
		RenderHtmlWithoutLayout(w, r, "does-not-exist", nil)
	})
}
