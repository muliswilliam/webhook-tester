package utils

import (
	"html/template"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"webhook-tester/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// The sidebar's data-init="@get('/webhook-stream/{id}?active=1')" call is a
// single-quoted JS string literal that Datastar compiles via new Function(...).
// Go's html/template does not trim whitespace around actions by default, so a
// template edit that reformats the {{ if }}/{{ end }} onto their own lines
// (e.g. an automated formatter re-wrapping a long attribute) can leak a raw
// newline into that literal. That's a SyntaxError in JS, so the whole @get()
// call throws before it runs - the active webhook's SSE connection silently
// never opens. Assert the rendered literal has no embedded whitespace for
// both the active and an inactive webhook.
func TestRenderHtmlHomeWebhookStreamDataInitHasNoEmbeddedWhitespace(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	active := newTestWebhook()
	inactive := models.Webhook{ID: "wh-2"}

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
		CSRFField: template.HTML(`<input type="hidden">`),
		Webhooks:  []models.Webhook{active, inactive},
		Webhook:   active,
		Domain:    "example.com",
	}

	RenderHtml(w, r, "home", data)
	body := w.Body.String()

	activeLiteral := jsStringLiteralAfter(t, body, "@get('", 0)
	assert.Equal(t, "/webhook-stream/"+active.ID+"?active=1", activeLiteral)

	inactiveLiteral := jsStringLiteralAfter(t, body, "@get('", strings.Index(body, activeLiteral))
	assert.Equal(t, "/webhook-stream/"+inactive.ID, inactiveLiteral)

	// A long-lived dashboard connection must not give up after Datastar's
	// default retryMaxCount of 10 (~3 minutes of backoff) - it should keep
	// retrying indefinitely across outages/restarts, like the native
	// EventSource it replaced.
	assert.Contains(t, body, "retryMaxCount: Infinity")
}

// jsStringLiteralAfter returns the contents of the next '...' literal
// following an occurrence of marker at or after fromIndex in body.
func jsStringLiteralAfter(t *testing.T, body, marker string, fromIndex int) string {
	t.Helper()
	rel := strings.Index(body[fromIndex:], marker)
	require.NotEqual(t, -1, rel, "marker %q not found after index %d in:\n%s", marker, fromIndex, body)
	start := fromIndex + rel + len(marker)
	// The literal's closing quote, not "')" - a call can carry trailing
	// arguments (e.g. an options object) between the quote and the ")".
	end := strings.IndexByte(body[start:], '\'')
	require.NotEqual(t, -1, end, "unterminated string literal after %q in:\n%s", marker, body)
	literal := body[start : start+end]
	assert.NotContains(t, literal, "\n", "JS string literal must not contain a raw newline")
	assert.NotContains(t, literal, "\r", "JS string literal must not contain a raw carriage return")
	return literal
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
