package utils

import (
	"encoding/hex"
	"html/template"
	"net/http/httptest"
	"regexp"
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

// testRequestRow and testRequestCounter mirror the handlers package's views.
type testRequestRow struct {
	Request   models.WebhookRequest
	CSRFField template.HTML
	IsNew     bool
}

type testRequestCounter struct {
	WebhookID string
	Count     int64
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
		ContentType     string
		RequestRows     []testRequestRow
		RequestCounter  testRequestCounter
		Domain          string
		Year            int
	}{
		CSRFField:       template.HTML(`<input type="hidden">`),
		User:            models.User{},
		Webhooks:        []models.Webhook{webhook},
		Webhook:         webhook,
		ResponseHeaders: "",
		RequestRows:     []testRequestRow{{Request: webhook.Requests[0]}},
		RequestCounter:  testRequestCounter{WebhookID: webhook.ID, Count: int64(len(webhook.Requests))},
		Domain:          "example.com",
		Year:            2026,
	}

	RenderHtml(w, r, "home", data)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "My webhook")
}

// The sidebar's data-init="@get('/webhook-stream/{id}?...')" call is a
// single-quoted JS string literal that Datastar compiles via new Function(...).
// Go's html/template does not trim whitespace around actions by default, so a
// template reformat (prettier re-wrapping the long attribute) can leak a raw
// newline into that literal: a JS SyntaxError that silently stops the SSE
// connection from ever opening. Assert the literal is exact for both the
// active and an inactive webhook.
func TestRenderHtmlHomeWebhookStreamDataInit(t *testing.T) {
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
		ContentType     string
		RequestRows     []testRequestRow
		RequestCounter  testRequestCounter
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

	// Each stream resumes after the newest request the page rendered.
	activeLiteral := jsStringLiteralAfter(t, body, "@get('", 0)
	assert.Equal(t, "/webhook-stream/"+active.ID+"?since="+models.LatestCursor(active.Requests).String()+"&active", activeLiteral)

	inactiveLiteral := jsStringLiteralAfter(t, body, "@get('", strings.Index(body, activeLiteral)+len(activeLiteral))
	assert.Equal(t, "/webhook-stream/"+inactive.ID+"?since=", inactiveLiteral)

	// A long-lived dashboard connection must retry indefinitely, including
	// after the server ends the stream (e.g. evicting a slow client), and
	// stay open in background tabs so no live update is deferred. It sends no
	// signals: the stream needs none, and every row adds some, which would
	// otherwise grow the URL without bound.
	assert.Contains(t, body, "{retry: 'always', retryMaxCount: Infinity, openWhenHidden: true, filterSignals: {include: /^$/}}")
}

func TestRenderHtmlHomeAssetsAreContentHashed(t *testing.T) {
	w := httptest.NewRecorder()
	RenderHtmlWithoutLayout(w, httptest.NewRequest("GET", "/login", nil), "login", nil)
	body := w.Body.String()

	for _, asset := range []string{"css/tailwind.css", "js/vendor/datastar.js", "js/script.js"} {
		url := assetURL(asset)
		assert.Regexp(t, `^/static/`+regexp.QuoteMeta(asset)+`\?v=[0-9a-f]{12}$`, url)
		assert.Contains(t, body, url)
	}
}

func TestAssetURL_MissingFileHasNoVersion(t *testing.T) {
	assert.Equal(t, "/static/nope.js", assetURL("nope.js"))
}

func TestPrettyJSON(t *testing.T) {
	assert.Equal(t, "{\n  \"a\": [\n    1,\n    2\n  ]\n}", prettyJSON(`{"a":[1,2]}`))
	assert.Equal(t, "", prettyJSON("not json"), "non-JSON")
	assert.Equal(t, "", prettyJSON(`"already"`), "formatting changes nothing")
	assert.Equal(t, "", prettyJSON(""), "empty")
}

func TestRenderPartialMainRequestRowBody(t *testing.T) {
	for name, tc := range map[string]struct {
		body          string
		wantFormatted bool
	}{
		"json":     {body: `{"event":"x"}`, wantFormatted: true},
		"not json": {body: "plain text", wantFormatted: false},
	} {
		t.Run(name, func(t *testing.T) {
			html, err := RenderPartialToString("main-request-row", testRequestRow{
				Request: models.WebhookRequest{ID: "req-1", WebhookID: "wh-1", Method: "POST", Body: tc.body},
			})
			require.NoError(t, err)

			key := hex.EncodeToString([]byte("req-1"))
			assert.Contains(t, html, "data-ref:body-raw_"+key)
			if tc.wantFormatted {
				assert.Contains(t, html, "&#34;event&#34;: &#34;x&#34;", "indented body")
				assert.Contains(t, html, `aria-label="Body format"`)
				assert.Contains(t, html, `data-show="!$rawBody_`+key+`"`, "formatted by default")
			} else {
				assert.NotContains(t, html, "body-formatted")
				assert.NotContains(t, html, "Body format")
			}
		})
	}
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
