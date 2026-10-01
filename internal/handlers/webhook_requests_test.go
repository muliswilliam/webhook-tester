package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"webhook-tester/internal/metrics"
	"webhook-tester/internal/models"
	"webhook-tester/internal/service"
	"webhook-tester/internal/utils"
)

func newTestWebhookRequestHandler(t *testing.T) (*WebhookRequestHandler, *testWebhookRequestRepo, *testWebhookRepo, *testUserRepo, *service.AuthService) {
	t.Helper()
	return newTestWebhookRequestHandlerAt(t, testDomain)
}

// newTestWebhookRequestHandlerAt is newTestWebhookRequestHandler for an
// instance served at domain, which replays to the endpoint target.
func newTestWebhookRequestHandlerAt(t *testing.T, domain string) (*WebhookRequestHandler, *testWebhookRequestRepo, *testWebhookRepo, *testUserRepo, *service.AuthService) {
	t.Helper()
	reqRepo := newTestWebhookRequestRepo()
	whRepo := newTestWebhookRepo()
	userRepo := newTestUserRepo()
	authSvc := newTestAuthService(t, userRepo)

	reqSvc := service.NewWebhookRequestService(reqRepo)
	whSvc := service.NewWebhookService(whRepo, &testDeliveryRepo{}, domain, testForwardPolicy)

	var rec metrics.Recorder = &testMetricsRecorder{}
	forwarder := newTestForwarder(whSvc, rec)
	h := NewWebhookRequestHandler(reqSvc, authSvc, whSvc, forwarder, &rec, newTestLogger())
	return h, reqRepo, whRepo, userRepo, authSvc
}

func TestWebhookRequestHandler_GetRequest_WebhookNotFound(t *testing.T) {
	h, _, _, _, _ := newTestWebhookRequestHandler(t)

	router := chi.NewRouter()
	router.Get("/requests/{id}", h.GetRequest)

	req := httptest.NewRequest(http.MethodGet, "/requests/r1?address=missing", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestWebhookRequestHandler_GetRequest_WebhookLoadError(t *testing.T) {
	h, _, whRepo, _, _ := newTestWebhookRequestHandler(t)
	whRepo.getWithRequestsErr = assert.AnError

	router := chi.NewRouter()
	router.Get("/requests/{id}", h.GetRequest)

	req := httptest.NewRequest(http.MethodGet, "/requests/r1?address=wh1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// Someone else's webhook - or a request filed under a different webhook than
// the one named in ?address= - reads as not found.
func TestWebhookRequestHandler_GetRequest_Inaccessible(t *testing.T) {
	h, reqRepo, whRepo, userRepo, authSvc := newTestWebhookRequestHandler(t)
	owner := &models.User{Email: "owner@x.com"}
	other := &models.User{Email: "other@x.com"}
	userRepo.addUser(owner)
	userRepo.addUser(other)
	whRepo.put(&models.Webhook{ID: "owned", UserID: int(owner.ID)})
	whRepo.put(&models.Webhook{ID: "public"})
	reqRepo.put(&models.WebhookRequest{ID: "r-owned", WebhookID: "owned"})

	router := chi.NewRouter()
	router.Get("/requests/{id}", h.GetRequest)

	for name, tc := range map[string]struct {
		path   string
		cookie *http.Cookie
	}{
		"guest":              {path: "/requests/r-owned?address=owned"},
		"another user":       {path: "/requests/r-owned?address=owned", cookie: sessionCookieFor(t, authSvc, other)},
		"mismatched address": {path: "/requests/r-owned?address=public"},
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			if tc.cookie != nil {
				req.AddCookie(tc.cookie)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusNotFound, rec.Code)
		})
	}
}

// Deleting or replaying a request requires access to its webhook.
func TestWebhookRequestHandler_MutationsRequireAccess(t *testing.T) {
	h, reqRepo, whRepo, userRepo, _ := newTestWebhookRequestHandler(t)
	owner := &models.User{Email: "owner@x.com"}
	userRepo.addUser(owner)
	whRepo.put(&models.Webhook{ID: "owned", UserID: int(owner.ID)})
	reqRepo.put(&models.WebhookRequest{ID: "r1", WebhookID: "owned", Method: http.MethodGet})

	router := chi.NewRouter()
	router.Post("/requests/{id}/delete", h.DeleteRequest)
	router.Post("/requests/{id}/replay", h.ReplayRequest)

	for _, path := range []string{"/requests/r1/delete", "/requests/r1/replay"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, nil))
		assert.Equal(t, http.StatusNotFound, rec.Code, path)
	}
	_, stillThere := reqRepo.requests["r1"]
	assert.True(t, stillThere)
}

func TestWebhookRequestHandler_GetRequest_RequestNotFound(t *testing.T) {
	h, _, whRepo, _, _ := newTestWebhookRequestHandler(t)
	whRepo.put(&models.Webhook{ID: "wh1"})

	router := chi.NewRouter()
	router.Get("/requests/{id}", h.GetRequest)

	req := httptest.NewRequest(http.MethodGet, "/requests/missing?address=wh1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestWebhookRequestHandler_GetRequest_Guest(t *testing.T) {
	h, reqRepo, whRepo, _, _ := newTestWebhookRequestHandler(t)
	whRepo.put(&models.Webhook{ID: "wh1", Title: "Hook1"})
	reqRepo.put(&models.WebhookRequest{ID: "r1", WebhookID: "wh1", Method: "GET", Headers: datatypes.JSONMap{}})

	router := chi.NewRouter()
	router.Get("/requests/{id}", h.GetRequest)

	req := httptest.NewRequest(http.MethodGet, "/requests/r1?address=wh1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Hook1")
}

func TestWebhookRequestHandler_GetRequest_LoggedInUser(t *testing.T) {
	h, reqRepo, whRepo, userRepo, authSvc := newTestWebhookRequestHandler(t)
	user := &models.User{Email: "jane@x.com"}
	userRepo.addUser(user)
	whRepo.put(&models.Webhook{ID: "wh1", Title: "Hook1", UserID: int(user.ID)})
	whRepo.put(&models.Webhook{ID: "wh2", Title: "Hook2", UserID: int(user.ID)})
	reqRepo.put(&models.WebhookRequest{ID: "r1", WebhookID: "wh1", Method: "GET", Headers: datatypes.JSONMap{}})

	cookie := sessionCookieFor(t, authSvc, user)
	router := chi.NewRouter()
	router.Get("/requests/{id}", h.GetRequest)

	req := httptest.NewRequest(http.MethodGet, "/requests/r1?address=wh1", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestWebhookRequestHandler_GetRequest_LoggedInUser_ListError(t *testing.T) {
	h, reqRepo, whRepo, userRepo, authSvc := newTestWebhookRequestHandler(t)
	user := &models.User{Email: "jane@x.com"}
	userRepo.addUser(user)
	whRepo.put(&models.Webhook{ID: "wh1", UserID: int(user.ID)})
	reqRepo.put(&models.WebhookRequest{ID: "r1", WebhookID: "wh1"})
	whRepo.getAllByUserErr = assert.AnError

	cookie := sessionCookieFor(t, authSvc, user)
	router := chi.NewRouter()
	router.Get("/requests/{id}", h.GetRequest)

	req := httptest.NewRequest(http.MethodGet, "/requests/r1?address=wh1", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWebhookRequestHandler_DeleteRequest_RedirectsToWebhook(t *testing.T) {
	for _, referer := range []string{"", "http://example.com/?address=wh1", "http://example.com/requests/r1?address=wh1"} {
		t.Run(referer, func(t *testing.T) {
			h, reqRepo, whRepo, _, _ := newTestWebhookRequestHandler(t)
			whRepo.put(&models.Webhook{ID: "wh1"})
			reqRepo.put(&models.WebhookRequest{ID: "r1", WebhookID: "wh1"})

			router := chi.NewRouter()
			router.Post("/requests/{id}/delete", h.DeleteRequest)

			req := httptest.NewRequest(http.MethodPost, "/requests/r1/delete", nil)
			if referer != "" {
				req.Header.Set("Referer", referer)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			// Never back to the deleted request's own page, which would 404.
			require.Equal(t, http.StatusSeeOther, rec.Code)
			assert.Equal(t, "/?address=wh1", rec.Header().Get("Location"))
			assert.Equal(t, &utils.Flash{Kind: utils.FlashSuccess, Message: "Request deleted."}, flashFrom(t, rec))
			_, ok := reqRepo.requests["r1"]
			assert.False(t, ok)
		})
	}
}

// A signed-in user can view a guest webhook but not manage it, so they can't
// delete its requests one by one either, as they can't clear them all.
func TestWebhookRequestHandler_DeleteRequest_SignedInUserOnGuestWebhook(t *testing.T) {
	h, reqRepo, whRepo, userRepo, authSvc := newTestWebhookRequestHandler(t)
	user := &models.User{Email: "jane@x.com"}
	userRepo.addUser(user)
	whRepo.put(&models.Webhook{ID: "wh1"})
	reqRepo.put(&models.WebhookRequest{ID: "r1", WebhookID: "wh1"})

	router := chi.NewRouter()
	router.Post("/requests/{id}/delete", h.DeleteRequest)

	req := httptest.NewRequest(http.MethodPost, "/requests/r1/delete", nil)
	req.AddCookie(sessionCookieFor(t, authSvc, user))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	_, ok := reqRepo.requests["r1"]
	assert.True(t, ok, "the request is kept")
}

// The request page offers Delete only to viewers who can manage its webhook.
func TestWebhookRequestHandler_GetRequest_DeleteOnlyForManagers(t *testing.T) {
	h, reqRepo, whRepo, userRepo, authSvc := newTestWebhookRequestHandler(t)
	user := &models.User{Email: "jane@x.com"}
	userRepo.addUser(user)
	whRepo.put(&models.Webhook{ID: "guest1"})
	whRepo.put(&models.Webhook{ID: "own1", UserID: int(user.ID)})
	reqRepo.put(&models.WebhookRequest{ID: "r1", WebhookID: "guest1", Method: "GET", Headers: datatypes.JSONMap{}})
	reqRepo.put(&models.WebhookRequest{ID: "r2", WebhookID: "own1", Method: "GET", Headers: datatypes.JSONMap{}})

	router := chi.NewRouter()
	router.Get("/requests/{id}", h.GetRequest)
	page := func(target string, signedIn bool) string {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		if signedIn {
			req.AddCookie(sessionCookieFor(t, authSvc, user))
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		return rec.Body.String()
	}

	assert.Contains(t, page("/requests/r1?address=guest1", false), `action="/requests/r1/delete"`, "a guest on a guest webhook")
	assert.Contains(t, page("/requests/r2?address=own1", true), `action="/requests/r2/delete"`, "the owner")
	body := page("/requests/r1?address=guest1", true)
	assert.NotContains(t, body, `action="/requests/r1/delete"`, "a signed-in user on a guest webhook")
	assert.Contains(t, body, `action="/requests/r1/replay"`, "replaying to the endpoint is still offered")
}

func TestWebhookRequestHandler_DeleteRequest_ServiceError(t *testing.T) {
	h, reqRepo, whRepo, _, _ := newTestWebhookRequestHandler(t)
	whRepo.put(&models.Webhook{ID: "wh1"})
	reqRepo.put(&models.WebhookRequest{ID: "r1", WebhookID: "wh1"})
	reqRepo.deleteByIDErr = assert.AnError

	router := chi.NewRouter()
	router.Post("/requests/{id}/delete", h.DeleteRequest)

	req := httptest.NewRequest(http.MethodPost, "/requests/r1/delete", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWebhookRequestHandler_ReplayRequest_NotFound(t *testing.T) {
	h, _, _, _, _ := newTestWebhookRequestHandler(t)

	router := chi.NewRouter()
	router.Post("/requests/{id}/replay", h.ReplayRequest)

	req := httptest.NewRequest(http.MethodPost, "/requests/missing/replay", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestWebhookRequestHandler_ReplayRequest_InvalidMethod(t *testing.T) {
	h, reqRepo, whRepo, _, _ := newTestWebhookRequestHandler(t)
	whRepo.put(&models.Webhook{ID: "wh1"})
	reqRepo.put(&models.WebhookRequest{ID: "r1", WebhookID: "wh1", Method: "BAD METHOD", Body: ""})

	router := chi.NewRouter()
	router.Post("/requests/{id}/replay", h.ReplayRequest)

	req := httptest.NewRequest(http.MethodPost, "/requests/r1/replay", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWebhookRequestHandler_ReplayRequest_NetworkError(t *testing.T) {
	closedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closedServer.Close()
	h, reqRepo, whRepo, _, _ := newTestWebhookRequestHandlerAt(t, closedServer.URL)
	whRepo.put(&models.Webhook{ID: "wh1"})
	reqRepo.put(&models.WebhookRequest{ID: "r1", WebhookID: "wh1", Method: http.MethodGet})

	router := chi.NewRouter()
	router.Post("/requests/{id}/replay", h.ReplayRequest)

	req := httptest.NewRequest(http.MethodPost, "/requests/r1/replay", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, utils.FlashError, flashFrom(t, rec).Kind)
}

func TestWebhookRequestHandler_ReplayRequest_InvalidDomain(t *testing.T) {
	h, reqRepo, whRepo, _, _ := newTestWebhookRequestHandlerAt(t, "://bad")
	whRepo.put(&models.Webhook{ID: "wh1"})
	reqRepo.put(&models.WebhookRequest{ID: "r1", WebhookID: "wh1", Method: http.MethodGet})

	router := chi.NewRouter()
	router.Post("/requests/{id}/replay", h.ReplayRequest)

	req := httptest.NewRequest(http.MethodPost, "/requests/r1/replay", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWebhookRequestHandler_ReplayRequest_Success(t *testing.T) {
	var gotPath, gotQuery, gotHeader string
	var gotHeaders http.Header
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotHeader = r.Header.Get("X-Original")
		gotHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusCreated)
	}))
	defer target.Close()
	h, reqRepo, whRepo, _, _ := newTestWebhookRequestHandlerAt(t, target.URL)
	whRepo.put(&models.Webhook{ID: "wh1"})

	reqRepo.put(&models.WebhookRequest{
		ID:        "r1",
		WebhookID: "wh1",
		Method:    http.MethodGet,
		Path:      "/orders/42",
		Query:     datatypes.JSONMap{"foo": "bar"},
		Headers:   datatypes.JSONMap{"X-Original": "yes"},
		Body:      "",
	})

	router := chi.NewRouter()
	router.Post("/requests/{id}/replay", h.ReplayRequest)

	req := httptest.NewRequest(http.MethodPost, "/requests/r1/replay", nil)
	req.Host = "example.com"
	req.Header.Set("Referer", "http://example.com/?address=wh1")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Back to the page the replay started from, with a confirmation.
	require.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/?address=wh1", rec.Header().Get("Location"))
	assert.Equal(t, &utils.Flash{Kind: utils.FlashSuccess, Message: "Request replayed. The endpoint answered 201 Created."}, flashFrom(t, rec))
	assert.Equal(t, "/webhooks/wh1/orders/42", gotPath)
	assert.Equal(t, "foo=bar", gotQuery)
	assert.Equal(t, "yes", gotHeader)
	assert.NotContains(t, gotHeaders, "Accept-Encoding", "the replay adds no headers of its own")
}

func TestWebhookRequestHandler_ReplayRequest_UnknownTarget(t *testing.T) {
	h, reqRepo, whRepo, _, _ := newTestWebhookRequestHandler(t)
	whRepo.put(&models.Webhook{ID: "wh1"})
	reqRepo.put(&models.WebhookRequest{ID: "r1", WebhookID: "wh1", Method: http.MethodGet})

	router := chi.NewRouter()
	router.Post("/requests/{id}/replay", h.ReplayRequest)

	req := httptest.NewRequest(http.MethodPost, "/requests/r1/replay", strings.NewReader("target=elsewhere"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// Replaying to the forward URL needs a webhook that forwards: one with an
// owner and a forward URL. Otherwise nothing is sent and the flash says why.
func TestWebhookRequestHandler_ReplayRequest_ForwardNeedsForwardingWebhook(t *testing.T) {
	forwardURL := "https://api.example.com/hooks"
	for name, tc := range map[string]struct {
		webhook models.Webhook
		owned   bool
		want    string
	}{
		"guest webhook": {
			webhook: models.Webhook{ID: "wh1", ForwardURL: &forwardURL},
			want:    "Forwarding is only available for endpoints in an account.",
		},
		"no forward URL": {
			webhook: models.Webhook{ID: "wh1"},
			owned:   true,
			want:    "This endpoint has no forward URL. Set one in its settings first.",
		},
	} {
		t.Run(name, func(t *testing.T) {
			h, reqRepo, whRepo, userRepo, authSvc := newTestWebhookRequestHandler(t)
			wh := tc.webhook
			req := httptest.NewRequest(http.MethodPost, "/requests/r1/replay", strings.NewReader("target=forward"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if tc.owned {
				owner := &models.User{Email: "owner@x.com"}
				userRepo.addUser(owner)
				wh.UserID = int(owner.ID)
				req.AddCookie(sessionCookieFor(t, authSvc, owner))
			}
			whRepo.put(&wh)
			reqRepo.put(&models.WebhookRequest{ID: "r1", WebhookID: "wh1", Method: http.MethodGet})

			router := chi.NewRouter()
			router.Post("/requests/{id}/replay", h.ReplayRequest)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusSeeOther, rec.Code)
			assert.Equal(t, &utils.Flash{Kind: utils.FlashError, Message: tc.want}, flashFrom(t, rec))
		})
	}
}
