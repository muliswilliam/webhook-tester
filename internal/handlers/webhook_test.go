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
)

func newTestWebhookHandler(t *testing.T) (*WebhookHandler, *testWebhookRepo, *testWebhookRequestRepo, *testUserRepo, *testMetricsRecorder, *service.AuthService) {
	t.Helper()
	whRepo := newTestWebhookRepo()
	reqRepo := newTestWebhookRequestRepo()
	userRepo := newTestUserRepo()
	metricsRec := &testMetricsRecorder{}
	authSvc := newTestAuthService(t, userRepo)

	whSvc := service.NewWebhookService(whRepo)
	reqSvc := service.NewWebhookRequestService(reqRepo)

	h := NewWebhookHandler(whSvc, reqSvc, authSvc, newTestLogger(), metricsRec)
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
	h, _, _, userRepo, _, authSvc := newTestWebhookHandler(t)
	user := &models.User{Email: "a@b.com"}
	userRepo.addUser(user)
	cookie := sessionCookieFor(t, authSvc, user)

	req := httptest.NewRequest(http.MethodPost, "/create-webhook?a=%zz", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
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

func TestWebhookHandler_Create_DefaultResponseCode_InvalidHeaders(t *testing.T) {
	h, whRepo, _, userRepo, _, authSvc := newTestWebhookHandler(t)
	user := &models.User{Email: "a@b.com"}
	userRepo.addUser(user)
	cookie := sessionCookieFor(t, authSvc, user)

	form := url.Values{
		"title":            {"hook2"},
		"response_headers": {"not-json"},
	}
	req := httptest.NewRequest(http.MethodPost, "/create-webhook", strings.NewReader(form.Encode()))
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
	w := whRepo.webhooks["wh1"]
	require.NotNil(t, w)
	assert.Equal(t, "updated", w.Title)
	assert.Equal(t, 201, w.ResponseCode)
	assert.Equal(t, uint(5), w.ResponseDelay)
	assert.Equal(t, "new-payload", *w.Payload)
	assert.True(t, w.NotifyOnEvent)
}

func TestWebhookHandler_UpdateWebhook_InvalidHeadersJSON(t *testing.T) {
	h, whRepo, _, _, _, _ := newTestWebhookHandler(t)
	whRepo.put(&models.Webhook{ID: "wh1"})

	router := routerWithParam("/update-webhook/{id}", http.MethodPost, h.UpdateWebhook)
	form := url.Values{"title": {"t"}, "response_headers": {"not-json"}}
	req := httptest.NewRequest(http.MethodPost, "/update-webhook/wh1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
}

func TestWebhookHandler_UpdateWebhook_ParseFormError(t *testing.T) {
	h, whRepo, _, _, _, _ := newTestWebhookHandler(t)
	whRepo.put(&models.Webhook{ID: "wh1"})

	req := httptest.NewRequest(http.MethodPost, "/update-webhook/wh1?a=%zz", nil)
	req = withURLParam(req, "id", "wh1")
	rec := httptest.NewRecorder()

	h.UpdateWebhook(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
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

func TestWebhookHandler_HandleWebhookRequest_NotFound(t *testing.T) {
	h, _, _, _, _, _ := newTestWebhookHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/webhooks/missing", nil)
	rec := httptest.NewRecorder()

	h.HandleWebhookRequest(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestWebhookHandler_HandleWebhookRequest_ServiceError(t *testing.T) {
	h, whRepo, _, _, _, _ := newTestWebhookHandler(t)
	whRepo.getErr = assert.AnError

	req := httptest.NewRequest(http.MethodGet, "/webhooks/wh1", nil)
	rec := httptest.NewRecorder()

	h.HandleWebhookRequest(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWebhookHandler_HandleWebhookRequest_Success_DefaultContentType(t *testing.T) {
	h, whRepo, _, _, metricsRec, _ := newTestWebhookHandler(t)
	payload := "hello"
	whRepo.put(&models.Webhook{ID: "wh1", ResponseCode: http.StatusCreated, Payload: &payload})

	req := httptest.NewRequest(http.MethodPost, "/webhooks/wh1?foo=bar", bytes.NewBufferString(`{"a":1}`))
	req.Header.Set("X-Custom", "yes")
	rec := httptest.NewRecorder()

	h.HandleWebhookRequest(rec, req)

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

	h.HandleWebhookRequest(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/plain", rec.Header().Get("Content-Type"))
	assert.Equal(t, "abc", rec.Header().Get("X-Custom-Resp"))
	assert.Equal(t, "plain-body", rec.Body.String())
}

func TestWebhookHandler_HandleWebhookRequest_CreateRequestError(t *testing.T) {
	h, whRepo, _, _, metricsRec, _ := newTestWebhookHandler(t)
	whRepo.put(&models.Webhook{ID: "wh1"})
	whRepo.insertRequestErr = assert.AnError

	req := httptest.NewRequest(http.MethodGet, "/webhooks/wh1", nil)
	rec := httptest.NewRecorder()

	h.HandleWebhookRequest(rec, req)

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
	h.HandleWebhookRequest(w, req)
}

func TestWebhookHandler_HandleWebhookRequest_BroadcastsToStream(t *testing.T) {
	h, whRepo, _, _, _, _ := newTestWebhookHandler(t)
	whRepo.put(&models.Webhook{ID: "wh-stream", ResponseCode: http.StatusOK})

	ch := make(chan string, 1)
	mu.Lock()
	webhookStreams["wh-stream"] = append(webhookStreams["wh-stream"], ch)
	mu.Unlock()
	defer func() {
		mu.Lock()
		delete(webhookStreams, "wh-stream")
		mu.Unlock()
	}()

	req := httptest.NewRequest(http.MethodGet, "/webhooks/wh-stream", nil)
	rec := httptest.NewRecorder()
	h.HandleWebhookRequest(rec, req)

	select {
	case msg := <-ch:
		assert.Contains(t, msg, "wh-stream")
	case <-time.After(time.Second):
		t.Fatal("expected a message to be broadcast to the stream channel")
	}
}

// flushableRecorder wraps httptest.ResponseRecorder to satisfy http.Flusher,
// since StreamWebhookEvents calls Flush() unconditionally. It also guards
// Write/body access with a mutex, since the handler writes from its own
// goroutine while the test concurrently polls the body.
type flushableRecorder struct {
	mu  sync.Mutex
	rec *httptest.ResponseRecorder
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

func (f *flushableRecorder) Flush() {}

func (f *flushableRecorder) bodyContains(s string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return strings.Contains(f.rec.Body.String(), s)
}

func TestWebhookHandler_StreamWebhookEvents_MessageReceived(t *testing.T) {
	h, _, _, _, _, _ := newTestWebhookHandler(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	router := chi.NewRouter()
	router.Get("/webhook-stream/{id}", h.StreamWebhookEvents)

	req := httptest.NewRequest(http.MethodGet, "/webhook-stream/stream1", nil).WithContext(ctx)
	rec := newFlushableRecorder()

	done := make(chan struct{})
	go func() {
		router.ServeHTTP(rec, req)
		close(done)
	}()

	// Wait until the handler has registered its channel.
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(webhookStreams["stream1"]) == 1
	}, time.Second, 5*time.Millisecond)

	mu.Lock()
	ch := webhookStreams["stream1"][0]
	mu.Unlock()
	ch <- "hello-event"

	require.Eventually(t, func() bool {
		return rec.bodyContains("hello-event")
	}, time.Second, 5*time.Millisecond)

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handler did not return after context cancellation")
	}

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, webhookStreams["stream1"], 0)
}

// brokenWriter implements http.ResponseWriter and http.Flusher, but always
// fails on Write - used to exercise StreamWebhookEvents' write-error branch.
type brokenWriter struct {
	header http.Header
}

func (b *brokenWriter) Header() http.Header        { return b.header }
func (b *brokenWriter) Write([]byte) (int, error)  { return 0, errWriteFailed }
func (b *brokenWriter) WriteHeader(statusCode int) {}
func (b *brokenWriter) Flush()                     {}

var errWriteFailed = errors.New("write failed")

func TestWebhookHandler_StreamWebhookEvents_WriteError(t *testing.T) {
	h, _, _, _, _, _ := newTestWebhookHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/webhook-stream/stream3", nil)
	req = withURLParam(req, "id", "stream3")
	w := &brokenWriter{header: http.Header{}}

	done := make(chan struct{})
	go func() {
		h.StreamWebhookEvents(w, req)
		close(done)
	}()

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(webhookStreams["stream3"]) == 1
	}, time.Second, 5*time.Millisecond)

	mu.Lock()
	ch := webhookStreams["stream3"][0]
	mu.Unlock()
	ch <- "hello"

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handler did not return after write error")
	}
}

func TestWebhookHandler_StreamWebhookEvents_ClientDisconnect(t *testing.T) {
	h, _, _, _, _, _ := newTestWebhookHandler(t)

	ctx, cancel := context.WithCancel(context.Background())

	req := httptest.NewRequest(http.MethodGet, "/webhook-stream/stream2", nil).WithContext(ctx)
	req = withURLParam(req, "id", "stream2")
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		h.StreamWebhookEvents(rec, req)
		close(done)
	}()

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(webhookStreams["stream2"]) == 1
	}, time.Second, 5*time.Millisecond)

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handler did not return after context cancellation")
	}

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, webhookStreams["stream2"], 0)
}
