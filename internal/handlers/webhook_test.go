package handlers

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"webhook-tester/internal/models"
	"webhook-tester/internal/service"
	"webhook-tester/internal/utils"
)

func newTestWebhookHandler(t *testing.T) (*WebhookHandler, *testWebhookRepo, *testWebhookRequestRepo, *testUserRepo, *testMetricsRecorder, *service.AuthService) {
	t.Helper()
	whRepo := newTestWebhookRepo()
	reqRepo := newTestWebhookRequestRepo()
	userRepo := newTestUserRepo()
	metricsRec := &testMetricsRecorder{}
	authSvc := newTestAuthService(t, userRepo)

	whSvc := service.NewWebhookService(whRepo, &testDeliveryRepo{})
	reqSvc := service.NewWebhookRequestService(reqRepo)

	forwarder := newTestForwarder(whSvc, metricsRec)
	h := NewWebhookHandler(whSvc, reqSvc, authSvc, forwarder, newTestLogger(), metricsRec)
	return h, whRepo, reqRepo, userRepo, metricsRec, authSvc
}

func TestWebhookHandler_Create_Unauthorized(t *testing.T) {
	h, _, _, _, metricsRec, _ := newTestWebhookHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/create-webhook", strings.NewReader("title=t"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, 0, metricsRec.webhooksCreated)
}

func TestWebhookHandler_Create_ParseFormError(t *testing.T) {
	h, whRepo, _, userRepo, _, authSvc := newTestWebhookHandler(t)
	user := &models.User{Email: "a@b.com"}
	userRepo.addUser(user)
	cookie := sessionCookieFor(t, authSvc, user)

	req := httptest.NewRequest(http.MethodPost, "/create-webhook?a=%zz", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, utils.FlashError, flashFrom(t, rec).Kind)
	assert.Empty(t, whRepo.webhooks)
}

func TestWebhookHandler_Create_Success(t *testing.T) {
	h, whRepo, _, userRepo, metricsRec, authSvc := newTestWebhookHandler(t)
	user := &models.User{Email: "a@b.com"}
	userRepo.addUser(user)
	cookie := sessionCookieFor(t, authSvc, user)

	form := url.Values{
		"title":            {"my hook"},
		"content_type":     {"application/json"},
		"response_delay":   {"0"},
		"payload":          {`{"ok":true}`},
		"notify_on_event":  {"true"},
		"response_headers": {`{"X-Test":"1"}`},
	}
	req := httptest.NewRequest(http.MethodPost, "/create-webhook", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	require.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, 1, metricsRec.webhooksCreated)
	assert.Len(t, whRepo.webhooks, 1)
	for _, w := range whRepo.webhooks {
		assert.Equal(t, "my hook", w.Title)
		assert.Equal(t, http.StatusOK, w.ResponseCode)
		assert.Equal(t, int(user.ID), w.UserID)
	}
}

func TestWebhookHandler_Create_DefaultResponseCode(t *testing.T) {
	h, whRepo, _, userRepo, _, authSvc := newTestWebhookHandler(t)
	user := &models.User{Email: "a@b.com"}
	userRepo.addUser(user)
	cookie := sessionCookieFor(t, authSvc, user)

	req := httptest.NewRequest(http.MethodPost, "/create-webhook", strings.NewReader("title=hook2"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	require.Equal(t, http.StatusSeeOther, rec.Code)
	require.Len(t, whRepo.webhooks, 1)
	for _, w := range whRepo.webhooks {
		assert.Equal(t, http.StatusOK, w.ResponseCode) // defaulted since response_code missing
	}
}

func TestWebhookHandler_Create_RejectsInvalidInput(t *testing.T) {
	cases := map[string]url.Values{
		"missing title":       {"title": {" "}},
		"code too high":       {"title": {"t"}, "response_code": {"1000"}},
		"code too low":        {"title": {"t"}, "response_code": {"99"}},
		"code not a number":   {"title": {"t"}, "response_code": {"abc"}},
		"negative delay":      {"title": {"t"}, "response_delay": {"-5"}},
		"delay too long":      {"title": {"t"}, "response_delay": {"30001"}},
		"headers not JSON":    {"title": {"t"}, "response_headers": {"not-json"}},
		"bad header name":     {"title": {"t"}, "response_headers": {`{"Bad Name":"1"}`}},
		"non-string header":   {"title": {"t"}, "response_headers": {`{"X-A":1}`}},
		"server-owned header": {"title": {"t"}, "response_headers": {`{"Content-Length":"1"}`}},
	}
	for name, form := range cases {
		t.Run(name, func(t *testing.T) {
			h, whRepo, _, userRepo, metricsRec, authSvc := newTestWebhookHandler(t)
			user := &models.User{Email: "a@b.com"}
			userRepo.addUser(user)

			req := httptest.NewRequest(http.MethodPost, "/create-webhook", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Referer", "http://example.com/?address=current")
			req.Host = "example.com"
			req.AddCookie(sessionCookieFor(t, authSvc, user))
			rec := httptest.NewRecorder()

			h.Create(rec, req)

			require.Equal(t, http.StatusSeeOther, rec.Code)
			assert.Equal(t, "/?address=current", rec.Header().Get("Location"), "back to the page the form was on")
			flash := flashFrom(t, rec)
			require.NotNil(t, flash)
			assert.Equal(t, utils.FlashError, flash.Kind)
			assert.Empty(t, whRepo.webhooks)
			assert.Equal(t, 0, metricsRec.webhooksCreated)
		})
	}
}

func TestWebhookPageURL(t *testing.T) {
	assert.Equal(t, "/?address=abc_-1", webhookPageURL("abc_-1"))
	assert.Equal(t, "/?address=a%26b%3Dc+d", webhookPageURL("a&b=c d"), "the id is query-escaped")
}

func TestBackURL(t *testing.T) {
	cases := map[string]string{
		"":                                   "/",
		"http://example.com/?address=abc":    "/?address=abc",
		"http://example.com/requests/r1?x=1": "/requests/r1?x=1",
		"http://evil.example/?address=abc":   "/",
		"not a url with spaces and %zz":      "/",
		"http://example.com":                 "/",
	}
	for referer, want := range cases {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Host = "example.com"
		if referer != "" {
			req.Header.Set("Referer", referer)
		}
		assert.Equal(t, want, backURL(req), referer)
	}
}

func TestWebhookHandler_Create_ServiceError(t *testing.T) {
	h, whRepo, _, userRepo, metricsRec, authSvc := newTestWebhookHandler(t)
	user := &models.User{Email: "a@b.com"}
	userRepo.addUser(user)
	cookie := sessionCookieFor(t, authSvc, user)
	whRepo.insertErr = assert.AnError

	req := httptest.NewRequest(http.MethodPost, "/create-webhook", strings.NewReader("title=t"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, 0, metricsRec.webhooksCreated)
}

// serveWebhook dispatches req through the production webhook routes, which
// set the {id} and subpath URL params HandleWebhookRequest reads.
func serveWebhook(h *WebhookHandler, w http.ResponseWriter, req *http.Request) {
	r := chi.NewRouter()
	r.Route("/webhooks", func(r chi.Router) {
		r.HandleFunc("/{id}", h.HandleWebhookRequest)
		r.HandleFunc("/{id}/*", h.HandleWebhookRequest)
	})
	r.ServeHTTP(w, req)
}

func routerWithParam(pattern string, method string, handlerFn http.HandlerFunc) chi.Router {
	r := chi.NewRouter()
	switch method {
	case http.MethodGet:
		r.Get(pattern, handlerFn)
	case http.MethodPost:
		r.Post(pattern, handlerFn)
	}
	return r
}

func TestWebhookHandler_DeleteRequests(t *testing.T) {
	h, whRepo, reqRepo, userRepo, _, authSvc := newTestWebhookHandler(t)
	user := &models.User{Email: "owner@b.com"}
	userRepo.addUser(user)
	cookie := sessionCookieFor(t, authSvc, user)

	owned := &models.Webhook{ID: "wh1", UserID: int(user.ID)}
	whRepo.put(owned)
	reqRepo.requests["r1"] = &models.WebhookRequest{ID: "r1", WebhookID: "wh1"}

	router := routerWithParam("/delete-requests/{id}", http.MethodPost, h.DeleteRequests)

	req := httptest.NewRequest(http.MethodPost, "/delete-requests/wh1", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/?address=wh1", rec.Header().Get("Location"))
	assert.Len(t, reqRepo.requests, 0)
}

func TestWebhookHandler_DeleteRequests_GuestFallback(t *testing.T) {
	h, whRepo, reqRepo, _, _, _ := newTestWebhookHandler(t)
	whRepo.put(&models.Webhook{ID: "wh-guest", UserID: 0})
	reqRepo.requests["r1"] = &models.WebhookRequest{ID: "r1", WebhookID: "wh-guest"}

	router := routerWithParam("/delete-requests/{id}", http.MethodPost, h.DeleteRequests)
	req := httptest.NewRequest(http.MethodPost, "/delete-requests/wh-guest", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
}

func TestWebhookHandler_DeleteRequests_EmptyID(t *testing.T) {
	h, _, _, _, _, _ := newTestWebhookHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/delete-requests/", nil)
	req = withURLParam(req, "id", "")
	rec := httptest.NewRecorder()

	h.DeleteRequests(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestWebhookHandler_DeleteRequests_NotFound(t *testing.T) {
	h, _, _, _, _, _ := newTestWebhookHandler(t)
	router := routerWithParam("/delete-requests/{id}", http.MethodPost, h.DeleteRequests)
	req := httptest.NewRequest(http.MethodPost, "/delete-requests/missing", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestWebhookHandler_DeleteRequests_ServiceError(t *testing.T) {
	h, whRepo, reqRepo, _, _, _ := newTestWebhookHandler(t)
	whRepo.put(&models.Webhook{ID: "wh1", UserID: 0})
	reqRepo.deleteByWebhookErr = assert.AnError

	router := routerWithParam("/delete-requests/{id}", http.MethodPost, h.DeleteRequests)
	req := httptest.NewRequest(http.MethodPost, "/delete-requests/wh1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWebhookHandler_DeleteWebhook_Success(t *testing.T) {
	h, whRepo, _, _, _, _ := newTestWebhookHandler(t)
	whRepo.put(&models.Webhook{ID: "wh1", UserID: 0})

	router := routerWithParam("/delete-webhook/{id}", http.MethodPost, h.DeleteWebhook)
	req := httptest.NewRequest(http.MethodPost, "/delete-webhook/wh1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/", rec.Header().Get("Location"))
	assert.Len(t, whRepo.webhooks, 0)
}

func TestWebhookHandler_DeleteWebhook_EmptyID(t *testing.T) {
	h, _, _, _, _, _ := newTestWebhookHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/delete-webhook/", nil)
	req = withURLParam(req, "id", "")
	rec := httptest.NewRecorder()

	h.DeleteWebhook(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestWebhookHandler_DeleteWebhook_NotFound(t *testing.T) {
	h, _, _, _, _, _ := newTestWebhookHandler(t)
	router := routerWithParam("/delete-webhook/{id}", http.MethodPost, h.DeleteWebhook)
	req := httptest.NewRequest(http.MethodPost, "/delete-webhook/missing", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestWebhookHandler_UpdateWebhook_Success(t *testing.T) {
	h, whRepo, _, _, _, _ := newTestWebhookHandler(t)
	ct := "text/plain"
	pl := "old"
	whRepo.put(&models.Webhook{ID: "wh1", UserID: 0, ContentType: &ct, Payload: &pl})

	router := routerWithParam("/update-webhook/{id}", http.MethodPost, h.UpdateWebhook)
	form := url.Values{
		"title":            {"updated"},
		"content_type":     {"application/json"},
		"response_code":    {"201"},
		"response_delay":   {"5"},
		"payload":          {"new-payload"},
		"notify_on_event":  {"true"},
		"response_headers": {`{"X-A":"1"}`},
	}
	req := httptest.NewRequest(http.MethodPost, "/update-webhook/wh1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/?address=wh1", rec.Header().Get("Location"))
	assert.Equal(t, utils.FlashSuccess, flashFrom(t, rec).Kind)
	w := whRepo.webhooks["wh1"]
	require.NotNil(t, w)
	assert.Equal(t, "updated", w.Title)
	assert.Equal(t, "1", w.ResponseHeaders["X-A"])
	assert.Equal(t, 201, w.ResponseCode)
	assert.Equal(t, uint(5), w.ResponseDelay)
	assert.Equal(t, "new-payload", *w.Payload)
	assert.True(t, w.NotifyOnEvent)
}

func TestWebhookHandler_UpdateWebhook_RejectsInvalidInput(t *testing.T) {
	cases := map[string]url.Values{
		"headers not JSON": {"title": {"t"}, "response_headers": {"not-json"}},
		"code too high":    {"title": {"t"}, "response_code": {"5000"}},
		"negative delay":   {"title": {"t"}, "response_delay": {"-1"}},
	}
	for name, form := range cases {
		t.Run(name, func(t *testing.T) {
			h, whRepo, _, _, _, _ := newTestWebhookHandler(t)
			whRepo.put(&models.Webhook{ID: "wh1", Title: "old", ResponseCode: 200, ResponseHeaders: datatypes.JSONMap{"X-Keep": "1"}})

			router := routerWithParam("/update-webhook/{id}", http.MethodPost, h.UpdateWebhook)
			req := httptest.NewRequest(http.MethodPost, "/update-webhook/wh1", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusSeeOther, rec.Code)
			assert.Equal(t, "/?address=wh1", rec.Header().Get("Location"))
			assert.Equal(t, utils.FlashError, flashFrom(t, rec).Kind)
			w := whRepo.webhooks["wh1"]
			assert.Equal(t, "old", w.Title, "a rejected update changes nothing")
			assert.Equal(t, 200, w.ResponseCode)
			assert.Equal(t, "1", w.ResponseHeaders["X-Keep"])
		})
	}
}

func TestWebhookHandler_UpdateWebhook_ParseFormError(t *testing.T) {
	h, whRepo, _, _, _, _ := newTestWebhookHandler(t)
	whRepo.put(&models.Webhook{ID: "wh1"})

	req := httptest.NewRequest(http.MethodPost, "/update-webhook/wh1?a=%zz", nil)
	req = withURLParam(req, "id", "wh1")
	rec := httptest.NewRecorder()

	h.UpdateWebhook(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, utils.FlashError, flashFrom(t, rec).Kind)
}

func TestWebhookHandler_UpdateWebhook_EmptyID(t *testing.T) {
	h, _, _, _, _, _ := newTestWebhookHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/update-webhook/", nil)
	req = withURLParam(req, "id", "")
	rec := httptest.NewRecorder()

	h.UpdateWebhook(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestWebhookHandler_UpdateWebhook_NotFound(t *testing.T) {
	h, _, _, _, _, _ := newTestWebhookHandler(t)
	router := routerWithParam("/update-webhook/{id}", http.MethodPost, h.UpdateWebhook)
	req := httptest.NewRequest(http.MethodPost, "/update-webhook/missing", strings.NewReader("title=t"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestWebhookHandler_UpdateWebhook_ServiceError(t *testing.T) {
	h, whRepo, _, _, _, _ := newTestWebhookHandler(t)
	whRepo.put(&models.Webhook{ID: "wh1"})
	whRepo.updateErr = assert.AnError

	router := routerWithParam("/update-webhook/{id}", http.MethodPost, h.UpdateWebhook)
	req := httptest.NewRequest(http.MethodPost, "/update-webhook/wh1", strings.NewReader("title=t"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// postUpdateForm submits the edit form for wh1, signed in when cookie is set.
func postUpdateForm(h *WebhookHandler, form url.Values, cookie *http.Cookie) *httptest.ResponseRecorder {
	router := routerWithParam("/update-webhook/{id}", http.MethodPost, h.UpdateWebhook)
	req := httptest.NewRequest(http.MethodPost, "/update-webhook/wh1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestWebhookHandler_UpdateWebhook_ForwardURL(t *testing.T) {
	t.Setenv("DOMAIN", "https://tester.example.com")
	h, whRepo, _, userRepo, _, authSvc := newTestWebhookHandler(t)
	user := &models.User{Email: "a@b.com"}
	userRepo.addUser(user)
	cookie := sessionCookieFor(t, authSvc, user)
	whRepo.put(&models.Webhook{ID: "wh1", Title: "t", ResponseCode: 200, UserID: int(user.ID)})
	forwardURL := func() *string { return whRepo.webhooks["wh1"].ForwardURL }

	// Set it; surrounding whitespace is trimmed.
	rec := postUpdateForm(h, url.Values{"title": {"t"}, "forward_url": {" https://api.example.com/hooks "}}, cookie)
	require.Equal(t, utils.FlashSuccess, flashFrom(t, rec).Kind)
	require.NotNil(t, forwardURL())
	assert.Equal(t, "https://api.example.com/hooks", *forwardURL())

	// Invalid values are rejected with the reason and change nothing.
	for bad, wantErr := range map[string]string{
		"ftp://files.example.com":               "absolute http or https URL",
		"api.example.com/hooks":                 "absolute http or https URL",
		"https://tester.example.com/webhooks/x": "own webhook endpoints",
	} {
		rec = postUpdateForm(h, url.Values{"title": {"t"}, "forward_url": {bad}}, cookie)
		flash := flashFrom(t, rec)
		assert.Equal(t, utils.FlashError, flash.Kind, bad)
		assert.Contains(t, flash.Message, wantErr, bad)
		assert.Equal(t, "https://api.example.com/hooks", *forwardURL(), bad)
	}

	// Submitting it blank clears it.
	rec = postUpdateForm(h, url.Values{"title": {"t"}, "forward_url": {""}}, cookie)
	require.Equal(t, utils.FlashSuccess, flashFrom(t, rec).Kind)
	assert.Nil(t, forwardURL())
}

func TestWebhookHandler_UpdateWebhook_GuestCantSetForwardURL(t *testing.T) {
	h, whRepo, _, _, _, _ := newTestWebhookHandler(t)
	whRepo.put(&models.Webhook{ID: "wh1", Title: "t", ResponseCode: 200})

	rec := postUpdateForm(h, url.Values{"title": {"renamed"}, "forward_url": {"https://api.example.com/hooks"}}, nil)

	assert.Equal(t, utils.FlashSuccess, flashFrom(t, rec).Kind, "the rest of the form still saves")
	assert.Equal(t, "renamed", whRepo.webhooks["wh1"].Title)
	assert.Nil(t, whRepo.webhooks["wh1"].ForwardURL, "a guest webhook never gets a forward URL")
}

func TestWebhookHandler_Create_WithForwardURL(t *testing.T) {
	h, whRepo, _, userRepo, _, authSvc := newTestWebhookHandler(t)
	user := &models.User{Email: "a@b.com"}
	userRepo.addUser(user)

	form := url.Values{"title": {"fwd"}, "forward_url": {"https://api.example.com/hooks"}}
	req := httptest.NewRequest(http.MethodPost, "/create-webhook", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sessionCookieFor(t, authSvc, user))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	require.Equal(t, http.StatusSeeOther, rec.Code)
	require.Len(t, whRepo.webhooks, 1)
	for _, w := range whRepo.webhooks {
		require.NotNil(t, w.ForwardURL)
		assert.Equal(t, "https://api.example.com/hooks", *w.ForwardURL)
	}
}

func TestWebhookHandler_HandleWebhookRequest_NotFound(t *testing.T) {
	h, _, _, _, _, _ := newTestWebhookHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/webhooks/missing", nil)
	rec := httptest.NewRecorder()

	serveWebhook(h, rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestWebhookHandler_HandleWebhookRequest_ServiceError(t *testing.T) {
	h, whRepo, _, _, _, _ := newTestWebhookHandler(t)
	whRepo.getErr = assert.AnError

	req := httptest.NewRequest(http.MethodGet, "/webhooks/wh1", nil)
	rec := httptest.NewRecorder()

	serveWebhook(h, rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWebhookHandler_HandleWebhookRequest_Success_DefaultContentType(t *testing.T) {
	h, whRepo, _, _, metricsRec, _ := newTestWebhookHandler(t)
	payload := "hello"
	whRepo.put(&models.Webhook{ID: "wh1", ResponseCode: http.StatusCreated, Payload: &payload})

	req := httptest.NewRequest(http.MethodPost, "/webhooks/wh1?foo=bar", bytes.NewBufferString(`{"a":1}`))
	req.Header.Set("X-Custom", "yes")
	rec := httptest.NewRecorder()

	serveWebhook(h, rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.Equal(t, "hello", rec.Body.String())
	assert.Equal(t, []string{"wh1"}, metricsRec.webhookRequests)
}

func TestWebhookHandler_HandleWebhookRequest_CustomContentTypeAndHeaders(t *testing.T) {
	h, whRepo, _, _, _, _ := newTestWebhookHandler(t)
	ct := "text/plain"
	payload := "plain-body"
	whRepo.put(&models.Webhook{
		ID:              "wh1",
		ResponseCode:    http.StatusOK,
		ContentType:     &ct,
		Payload:         &payload,
		ResponseHeaders: datatypes.JSONMap{"X-Custom-Resp": "abc"},
		ResponseDelay:   1,
	})

	req := httptest.NewRequest(http.MethodGet, "/webhooks/wh1", nil)
	rec := httptest.NewRecorder()

	serveWebhook(h, rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/plain", rec.Header().Get("Content-Type"))
	assert.Equal(t, "abc", rec.Header().Get("X-Custom-Resp"))
	assert.Equal(t, "plain-body", rec.Body.String())
}

func TestWebhookHandler_HandleWebhookRequest_RecordsSubpath(t *testing.T) {
	h, whRepo, _, _, _, _ := newTestWebhookHandler(t)
	whRepo.put(&models.Webhook{ID: "wh1", ResponseCode: http.StatusOK})

	req := httptest.NewRequest(http.MethodPost, "/webhooks/wh1/orders/42?x=1", nil)
	rec := httptest.NewRecorder()
	serveWebhook(h, rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, whRepo.insertedRequests, 1)
	assert.Equal(t, "/orders/42", whRepo.insertedRequests[0].Path)
	assert.Equal(t, "1", whRepo.insertedRequests[0].Query["x"])
}

// The capture keeps what was sent: the escaped subpath, the raw query, and
// every value of a repeated header or query parameter.
func TestWebhookHandler_HandleWebhookRequest_RecordsRequestFaithfully(t *testing.T) {
	for name, tc := range map[string]struct {
		target   string
		wantPath string
	}{
		"escaped slash":         {target: "/webhooks/wh1/files/a%2Fb?b=x%20y&a=1&a=2", wantPath: "/files/a%2Fb"},
		"default escapes":       {target: "/webhooks/wh1/files/a%20b?b=x%20y&a=1&a=2", wantPath: "/files/a%20b"},
		"plain path":            {target: "/webhooks/wh1/files?b=x%20y&a=1&a=2", wantPath: "/files"},
		"bare trailing slash":   {target: "/webhooks/wh1/?b=x%20y&a=1&a=2", wantPath: "/"},
		"escaped and unescaped": {target: "/webhooks/wh1/a%2Fb/c%20d/%7Ee?b=x%20y&a=1&a=2", wantPath: "/a%2Fb/c%20d/%7Ee"},
	} {
		t.Run(name, func(t *testing.T) {
			h, whRepo, _, _, _, _ := newTestWebhookHandler(t)
			whRepo.put(&models.Webhook{ID: "wh1", ResponseCode: http.StatusOK})

			req := httptest.NewRequest(http.MethodPost, tc.target, nil)
			req.Header.Add("X-Multi", "one")
			req.Header.Add("X-Multi", "two, three")
			req.Header.Set("X-Single", "only")
			serveWebhook(h, httptest.NewRecorder(), req)

			require.Len(t, whRepo.insertedRequests, 1)
			wr := whRepo.insertedRequests[0]
			assert.Equal(t, tc.wantPath, wr.Path)
			assert.Equal(t, "b=x%20y&a=1&a=2", wr.RawQuery)
			assert.Equal(t, []string{"one", "two, three"}, wr.Headers["X-Multi"], "repeated values aren't joined")
			assert.Equal(t, "only", wr.Headers["X-Single"])
			assert.Equal(t, []string{"1", "2"}, wr.Query["a"])
			assert.Equal(t, "x y", wr.Query["b"])
		})
	}
}

func TestWebhookHandler_HandleWebhookRequest_InvalidStoredResponseCode(t *testing.T) {
	h, whRepo, _, _, _, _ := newTestWebhookHandler(t)
	// Saved before validation existed; WriteHeader would panic on it.
	whRepo.put(&models.Webhook{ID: "wh1", ResponseCode: 1000})

	req := httptest.NewRequest(http.MethodGet, "/webhooks/wh1", nil)
	rec := httptest.NewRecorder()
	serveWebhook(h, rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWebhookHandler_HandleWebhookRequest_CreateRequestError(t *testing.T) {
	h, whRepo, _, _, metricsRec, _ := newTestWebhookHandler(t)
	whRepo.put(&models.Webhook{ID: "wh1"})
	whRepo.insertRequestErr = assert.AnError

	req := httptest.NewRequest(http.MethodGet, "/webhooks/wh1", nil)
	rec := httptest.NewRecorder()

	serveWebhook(h, rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, 0, len(metricsRec.webhookRequests))
}

func TestWebhookHandler_HandleWebhookRequest_PayloadWriteError(t *testing.T) {
	h, whRepo, _, _, _, _ := newTestWebhookHandler(t)
	payload := "hello"
	whRepo.put(&models.Webhook{ID: "wh1", ResponseCode: http.StatusOK, Payload: &payload})

	req := httptest.NewRequest(http.MethodGet, "/webhooks/wh1", nil)
	w := &brokenWriter{header: http.Header{}}

	// Should not panic even though the payload write fails; the handler just
	// logs the error.
	serveWebhook(h, w, req)
}

func TestWebhookHandler_HandleWebhookRequest_BroadcastsToStream(t *testing.T) {
	h, whRepo, _, _, _, _ := newTestWebhookHandler(t)
	whRepo.put(&models.Webhook{ID: "wh-stream", ResponseCode: http.StatusOK})

	sub := h.webhookSvc.Subscribe("wh-stream")
	defer sub.Close()

	req := httptest.NewRequest(http.MethodGet, "/webhooks/wh-stream", nil)
	rec := httptest.NewRecorder()
	serveWebhook(h, rec, req)

	select {
	case evt := <-sub.Events:
		assert.Equal(t, "wh-stream", evt.Request.WebhookID)
		assert.False(t, evt.Request.ReceivedAt.IsZero())
	case <-time.After(time.Second):
		t.Fatal("expected the captured request to be published")
	}
}

// flushableRecorder wraps httptest.ResponseRecorder to satisfy http.Flusher,
// since StreamWebhookEvents flushes. It guards all access with a mutex, since
// the handler writes from its own goroutine while the test polls.
type flushableRecorder struct {
	mu      sync.Mutex
	rec     *httptest.ResponseRecorder
	flushed bool
}

func newFlushableRecorder() *flushableRecorder {
	return &flushableRecorder{rec: httptest.NewRecorder()}
}

func (f *flushableRecorder) Header() http.Header {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.rec.Header()
}

func (f *flushableRecorder) Write(b []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.rec.Write(b)
}

func (f *flushableRecorder) WriteHeader(statusCode int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rec.WriteHeader(statusCode)
}

func (f *flushableRecorder) Flush() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.flushed = true
}

func (f *flushableRecorder) body() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.rec.Body.String()
}

// streamRun is a StreamWebhookEvents request served in the background.
type streamRun struct {
	rec    *flushableRecorder
	cancel context.CancelFunc
	done   chan struct{}
}

// startStream serves req on StreamWebhookEvents in the background and waits
// until the stream has subscribed and flushed its headers.
func startStream(t *testing.T, h *WebhookHandler, req *http.Request) *streamRun {
	t.Helper()
	ctx, cancel := context.WithCancel(req.Context())
	t.Cleanup(cancel)

	router := chi.NewRouter()
	router.Get("/webhook-stream/{id}", h.StreamWebhookEvents)

	run := &streamRun{rec: newFlushableRecorder(), cancel: cancel, done: make(chan struct{})}
	go func() {
		router.ServeHTTP(run.rec, req.WithContext(ctx))
		close(run.done)
	}()
	require.Eventually(t, func() bool {
		run.rec.mu.Lock()
		defer run.rec.mu.Unlock()
		return run.rec.flushed
	}, time.Second, 5*time.Millisecond, "stream never started")
	return run
}

func (s *streamRun) waitFor(t *testing.T, substrings ...string) {
	t.Helper()
	require.Eventually(t, func() bool {
		body := s.rec.body()
		for _, sub := range substrings {
			if !strings.Contains(body, sub) {
				return false
			}
		}
		return true
	}, time.Second, 5*time.Millisecond, "stream body missing %q:\n%s", substrings, s.rec.body())
}

func (s *streamRun) requireEnded(t *testing.T) {
	t.Helper()
	select {
	case <-s.done:
	case <-time.After(time.Second):
		t.Fatal("stream handler did not return")
	}
}

func recordRequest(t *testing.T, h *WebhookHandler, webhookID, id string) models.WebhookRequest {
	t.Helper()
	wh, err := h.webhookSvc.GetWebhook(webhookID)
	require.NoError(t, err)
	wr := models.WebhookRequest{ID: id, WebhookID: webhookID, Method: "POST", Body: `{"a":1}`}
	require.NoError(t, h.webhookSvc.RecordRequest(wh, &wr))
	return wr
}

func TestWebhookHandler_StreamWebhookEvents_ActivePatchesMainPanel(t *testing.T) {
	h, whRepo, _, _, _, _ := newTestWebhookHandler(t)
	whRepo.put(&models.Webhook{ID: "wh"})

	run := startStream(t, h, httptest.NewRequest(http.MethodGet, "/webhook-stream/wh?active", nil))
	wr := recordRequest(t, h, "wh", "req-active")

	run.waitFor(t,
		"selector #request-log-wh", "selector #request-log-list-wh", "Replay request", "new-badge",
		"1 captured request", "id: "+models.CursorAt(wr).String())

	run.cancel()
	run.requireEnded(t)
}

func TestWebhookHandler_StreamWebhookEvents_NonActiveSkipsMainPanel(t *testing.T) {
	h, whRepo, _, _, _, _ := newTestWebhookHandler(t)
	whRepo.put(&models.Webhook{ID: "wh"})

	run := startStream(t, h, httptest.NewRequest(http.MethodGet, "/webhook-stream/wh", nil))
	wr := recordRequest(t, h, "wh", "req-non-active")

	// The sidebar row carries the event ID when it is the only patch.
	run.waitFor(t, "req-non-active", "id: "+models.CursorAt(wr).String())
	body := run.rec.body()
	assert.NotContains(t, body, "Replay request")
	assert.NotContains(t, body, "request-log-list-wh")
	assert.NotContains(t, body, "captured request")
}

func TestWebhookHandler_StreamWebhookEvents_ReplaysMissedRequests(t *testing.T) {
	t0 := time.Now().UTC().Truncate(time.Microsecond)
	seen := models.WebhookRequest{ID: "req-seen", WebhookID: "wh", ReceivedAt: t0}
	missed := models.WebhookRequest{ID: "req-missed", WebhookID: "wh", ReceivedAt: t0.Add(time.Second)}

	for name, setCursor := range map[string]func(r *http.Request){
		"since query": func(r *http.Request) {
			r.URL.RawQuery = "active&since=" + url.QueryEscape(models.CursorAt(seen).String())
		},
		// A reconnect's Last-Event-ID wins over the page's stale ?since=.
		"last event id": func(r *http.Request) {
			r.URL.RawQuery = "active&since="
			r.Header.Set("Last-Event-ID", models.CursorAt(seen).String())
		},
	} {
		t.Run(name, func(t *testing.T) {
			h, whRepo, _, _, _, _ := newTestWebhookHandler(t)
			whRepo.put(&models.Webhook{ID: "wh", Requests: []models.WebhookRequest{seen, missed}})

			req := httptest.NewRequest(http.MethodGet, "/webhook-stream/wh", nil)
			setCursor(req)
			run := startStream(t, h, req)

			run.waitFor(t, "req-missed", "2 captured requests", "id: "+models.CursorAt(missed).String())
			assert.NotContains(t, run.rec.body(), "req-seen")

			// Live streaming continues after the replay.
			recordRequest(t, h, "wh", "req-live")
			run.waitFor(t, "req-live", "3 captured requests")
		})
	}
}

// recordDelivery records a delivery the target answered with status, started
// at startedAt, as the forwarder does.
func recordDelivery(t *testing.T, h *WebhookHandler, webhookID, requestID, id string, status int, startedAt time.Time) models.Delivery {
	t.Helper()
	d := models.Delivery{
		ID: id, RequestID: requestID, WebhookID: webhookID, Trigger: models.DeliveryTriggerAuto,
		TargetURL: "https://hooks.example.com/in", Outcome: models.DeliveryOutcomeForStatus(status), StatusCode: &status, StartedAt: startedAt,
	}
	require.NoError(t, h.webhookSvc.RecordDelivery(&d))
	return d
}

func TestWebhookHandler_StreamWebhookEvents_ActivePatchesDeliveries(t *testing.T) {
	h, whRepo, _, userRepo, _, authSvc := newTestWebhookHandler(t)
	owner := &models.User{Email: "owner@example.com"}
	userRepo.addUser(owner)
	forwardURL := "https://hooks.example.com/in"
	whRepo.put(&models.Webhook{ID: "wh", UserID: int(owner.ID), ForwardURL: &forwardURL})

	req := httptest.NewRequest(http.MethodGet, "/webhook-stream/wh?active", nil)
	req.AddCookie(sessionCookieFor(t, authSvc, owner))
	run := startStream(t, h, req)
	recordRequest(t, h, "wh", "req-1")
	// The streamed row offers the replay to the forward URL.
	run.waitFor(t, "selector #request-log-list-wh", "Replay to forward URL")

	recordDelivery(t, h, "wh", "req-1", "del-1", http.StatusBadGateway, time.Now().UTC())
	run.waitFor(t,
		`id="delivery-badge-req-1"`, "Last delivery: 502 Bad Gateway",
		`id="delivery-list-req-1"`, `id="delivery-del-1"`)
}

// Deliveries are listed by when they started, however they finish: an
// automatic delivery that outlasts a later replay stays below it, and the
// badge keeps showing the replay.
func TestWebhookHandler_StreamWebhookEvents_DeliveriesInStartOrder(t *testing.T) {
	h, whRepo, _, _, _, _ := newTestWebhookHandler(t)
	whRepo.put(&models.Webhook{ID: "wh"})

	run := startStream(t, h, httptest.NewRequest(http.MethodGet, "/webhook-stream/wh?active", nil))
	recordRequest(t, h, "wh", "req-1")
	started := time.Now().UTC()
	recordDelivery(t, h, "wh", "req-1", "del-replay", http.StatusOK, started.Add(time.Second))
	recordDelivery(t, h, "wh", "req-1", "del-auto", http.StatusInternalServerError, started)

	// The last list patch has both deliveries, newest started first.
	run.waitFor(t, `id="delivery-del-auto"`)
	body := run.rec.body()
	lastList := body[strings.LastIndex(body, `id="delivery-list-req-1"`):]
	replay, auto := strings.Index(lastList, `id="delivery-del-replay"`), strings.Index(lastList, `id="delivery-del-auto"`)
	require.NotEqual(t, -1, replay, "the replay is still listed")
	assert.Less(t, replay, auto, "the replay started last, so it stays on top")

	lastBadge := body[strings.LastIndex(body, `id="delivery-badge-req-1"`):]
	lastBadge = lastBadge[:strings.Index(lastBadge, `id="delivery-list-req-1"`)] // the badge is patched before the list
	assert.Contains(t, lastBadge, "Last delivery: 200 OK")
}

func TestWebhookHandler_StreamWebhookEvents_RequestPagePatchesOnlyItsDeliveries(t *testing.T) {
	h, whRepo, _, _, _, _ := newTestWebhookHandler(t)
	whRepo.put(&models.Webhook{ID: "wh"})

	run := startStream(t, h, httptest.NewRequest(http.MethodGet, "/webhook-stream/wh?request=req-shown", nil))
	recordDelivery(t, h, "wh", "req-other", "del-other", http.StatusOK, time.Now().UTC())
	recordDelivery(t, h, "wh", "req-shown", "del-shown", http.StatusOK, time.Now().UTC())

	run.waitFor(t, `id="delivery-list-req-shown"`, `id="delivery-del-shown"`)
	body := run.rec.body()
	assert.NotContains(t, body, "del-other")
	assert.NotContains(t, body, "delivery-badge-", "the request page has no request list")

	// New captures still reach the sidebar, but not a main request list.
	recordRequest(t, h, "wh", "req-new")
	run.waitFor(t, "selector #request-log-wh", "req-new")
	assert.NotContains(t, run.rec.body(), "request-log-list-wh")
}

func TestWebhookHandler_StreamWebhookEvents_SidebarSkipsDeliveries(t *testing.T) {
	h, whRepo, _, _, _, _ := newTestWebhookHandler(t)
	whRepo.put(&models.Webhook{ID: "wh"})

	run := startStream(t, h, httptest.NewRequest(http.MethodGet, "/webhook-stream/wh", nil))
	recordDelivery(t, h, "wh", "req-1", "del-1", http.StatusOK, time.Now().UTC())
	// Events are handled in order, so once the capture is streamed the
	// delivery before it has been skipped.
	recordRequest(t, h, "wh", "req-2")
	run.waitFor(t, "req-2")
	assert.NotContains(t, run.rec.body(), "del-1")
}

// Missed requests are replayed with their deliveries.
func TestWebhookHandler_StreamWebhookEvents_ReplaysMissedDeliveries(t *testing.T) {
	h, whRepo, _, _, _, _ := newTestWebhookHandler(t)
	status := http.StatusOK
	missed := models.WebhookRequest{
		ID: "req-missed", WebhookID: "wh", ReceivedAt: time.Now().UTC(),
		Deliveries: []models.Delivery{{ID: "del-missed", RequestID: "req-missed", WebhookID: "wh", Outcome: models.DeliveryOutcomeForStatus(status), StatusCode: &status}},
	}
	whRepo.put(&models.Webhook{ID: "wh", Requests: []models.WebhookRequest{missed}})

	run := startStream(t, h, httptest.NewRequest(http.MethodGet, "/webhook-stream/wh?active&since=", nil))
	run.waitFor(t, "req-missed", `id="delivery-del-missed"`, "Last delivery: 200 OK")

	recordRequest(t, h, "wh", "req-live")
	run.waitFor(t, "req-live")
}

func TestWebhookHandler_StreamWebhookEvents_ReplayErrorEndsStream(t *testing.T) {
	h, whRepo, _, _, _, _ := newTestWebhookHandler(t)
	whRepo.put(&models.Webhook{ID: "wh"})
	whRepo.getRequestsAfterErr = errors.New("db down")

	rec := httptest.NewRecorder()
	req := withURLParam(httptest.NewRequest(http.MethodGet, "/webhook-stream/wh", nil), "id", "wh")
	h.StreamWebhookEvents(rec, req)

	// No 204: the client should retry.
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Body.String())
}

// Streams the client must not retry end with 204 No Content.
func TestWebhookHandler_StreamWebhookEvents_NoContent(t *testing.T) {
	h, whRepo, _, userRepo, _, authSvc := newTestWebhookHandler(t)
	owner := &models.User{Email: "owner@example.com"}
	userRepo.addUser(owner)
	whRepo.put(&models.Webhook{ID: "public"})
	whRepo.put(&models.Webhook{ID: "owned", UserID: int(owner.ID)})

	for name, req := range map[string]*http.Request{
		"missing webhook":        httptest.NewRequest(http.MethodGet, "/webhook-stream/missing", nil),
		"someone else's webhook": httptest.NewRequest(http.MethodGet, "/webhook-stream/owned", nil),
		"malformed since cursor": httptest.NewRequest(http.MethodGet, "/webhook-stream/public?since=nope", nil),
		"malformed last event id": func() *http.Request {
			r := httptest.NewRequest(http.MethodGet, "/webhook-stream/public", nil)
			r.Header.Set("Last-Event-ID", "12:")
			return r
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			router := chi.NewRouter()
			router.Get("/webhook-stream/{id}", h.StreamWebhookEvents)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusNoContent, rec.Code)
		})
	}

	t.Run("owner may stream", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/webhook-stream/owned", nil)
		req.AddCookie(sessionCookieFor(t, authSvc, owner))
		run := startStream(t, h, req)
		recordRequest(t, h, "owned", "req-owned")
		run.waitFor(t, "req-owned")
	})
}

func TestWebhookHandler_StreamWebhookEvents_EndsWhenWebhookDeleted(t *testing.T) {
	h, whRepo, _, _, _, _ := newTestWebhookHandler(t)
	whRepo.put(&models.Webhook{ID: "wh"})

	run := startStream(t, h, httptest.NewRequest(http.MethodGet, "/webhook-stream/wh", nil))
	require.NoError(t, h.webhookSvc.DeleteWebhook("wh", 0))
	run.requireEnded(t)
}

// brokenWriter implements http.ResponseWriter and http.Flusher, but always
// fails on Write - used to exercise StreamWebhookEvents' write-error branch.
type brokenWriter struct {
	mu      sync.Mutex
	header  http.Header
	flushed bool
}

func (b *brokenWriter) Header() http.Header        { return b.header }
func (b *brokenWriter) Write([]byte) (int, error)  { return 0, errWriteFailed }
func (b *brokenWriter) WriteHeader(statusCode int) {}
func (b *brokenWriter) Flush() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.flushed = true
}

var errWriteFailed = errors.New("write failed")

func TestWebhookHandler_StreamWebhookEvents_WriteError(t *testing.T) {
	h, whRepo, _, _, _, _ := newTestWebhookHandler(t)
	whRepo.put(&models.Webhook{ID: "wh"})

	req := withURLParam(httptest.NewRequest(http.MethodGet, "/webhook-stream/wh", nil), "id", "wh")
	w := &brokenWriter{header: http.Header{}}

	done := make(chan struct{})
	go func() {
		h.StreamWebhookEvents(w, req)
		close(done)
	}()
	require.Eventually(t, func() bool {
		w.mu.Lock()
		defer w.mu.Unlock()
		return w.flushed
	}, time.Second, 5*time.Millisecond)

	recordRequest(t, h, "wh", "req-hello")

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handler did not return after write error")
	}
}

func TestWebhookHandler_StreamWebhookEvents_ClientDisconnect(t *testing.T) {
	h, whRepo, _, _, _, _ := newTestWebhookHandler(t)
	whRepo.put(&models.Webhook{ID: "wh"})

	run := startStream(t, h, httptest.NewRequest(http.MethodGet, "/webhook-stream/wh", nil))
	run.cancel()
	run.requireEnded(t)
}
