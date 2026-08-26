package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"webhook-tester/internal/models"
	"webhook-tester/internal/service"
)

func newTestHomeHandler(t *testing.T) (*HomeHandler, *testWebhookRepo, *testUserRepo, *testMetricsRecorder, *service.AuthService) {
	t.Helper()
	whRepo := newTestWebhookRepo()
	userRepo := newTestUserRepo()
	metricsRec := &testMetricsRecorder{}
	authSvc := newTestAuthService(t, userRepo)
	whSvc := service.NewWebhookService(whRepo)

	h := NewHomeHandler(whSvc, authSvc, newTestLogger(), metricsRec)
	return h, whRepo, userRepo, metricsRec, authSvc
}

func TestHomeHandler_Guest_NoCookie_CreatesDefaultWebhook(t *testing.T) {
	h, whRepo, _, metricsRec, _ := newTestHomeHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	h.Home(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, metricsRec.webhooksCreated)
	assert.Len(t, whRepo.webhooks, 1)

	var sawGuestCookie bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionIdName {
			sawGuestCookie = true
			assert.NotEmpty(t, c.Value)
		}
	}
	assert.True(t, sawGuestCookie)
	assert.Contains(t, rec.Body.String(), "Temporary workspace")
}

func TestHomeHandler_Guest_ExistingCookie(t *testing.T) {
	h, whRepo, _, metricsRec, _ := newTestHomeHandler(t)
	whRepo.put(&models.Webhook{ID: "wh1", Title: "Existing"})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionIdName, Value: "wh1"})
	rec := httptest.NewRecorder()

	h.Home(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 0, metricsRec.webhooksCreated)
	assert.Contains(t, rec.Body.String(), "Existing")
}

func TestHomeHandler_Guest_CookieLoadFailure_RedirectsAndInvalidatesCookie(t *testing.T) {
	h, _, _, _, _ := newTestHomeHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionIdName, Value: "does-not-exist"})
	rec := httptest.NewRecorder()

	h.Home(rec, req)

	require.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/", rec.Header().Get("Location"))

	var invalidated bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionIdName && c.MaxAge == -1 {
			invalidated = true
		}
	}
	assert.True(t, invalidated)
}

func TestHomeHandler_AddressQueryParam(t *testing.T) {
	h, whRepo, _, _, _ := newTestHomeHandler(t)
	whRepo.put(&models.Webhook{ID: "wh-cookie", Title: "Cookie Hook"})
	whRepo.put(&models.Webhook{ID: "wh-address", Title: "Address Hook"})

	req := httptest.NewRequest(http.MethodGet, "/?address=wh-address", nil)
	req.AddCookie(&http.Cookie{Name: sessionIdName, Value: "wh-cookie"})
	rec := httptest.NewRecorder()

	h.Home(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Address Hook")
}

func TestHomeHandler_AddressQueryParam_LoadFailure(t *testing.T) {
	h, _, _, _, _ := newTestHomeHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/?address=missing", nil)
	req.AddCookie(&http.Cookie{Name: sessionIdName, Value: "missing"})
	rec := httptest.NewRecorder()

	h.Home(rec, req)

	// Active webhook load fails too, but Home still renders successfully with
	// an empty active webhook (guest cookie webhook load already redirected
	// away before reaching the `address` branch in this scenario since both
	// point at the same missing ID).
	assert.Equal(t, http.StatusSeeOther, rec.Code)
}

func TestHomeHandler_LoggedInUser_ListsOwnWebhooks(t *testing.T) {
	h, whRepo, userRepo, _, authSvc := newTestHomeHandler(t)
	user := &models.User{Email: "jane@x.com", APIKey: "key1"}
	userRepo.addUser(user)
	whRepo.put(&models.Webhook{ID: "wh1", Title: "Mine", UserID: int(user.ID)})
	whRepo.put(&models.Webhook{ID: "wh2", Title: "NotMine", UserID: 999})

	cookie := sessionCookieFor(t, authSvc, user)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	h.Home(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Mine")
	assert.NotContains(t, rec.Body.String(), "NotMine")
}

func TestHomeHandler_LoggedInUser_NoCookieCreated(t *testing.T) {
	h, whRepo, userRepo, metricsRec, authSvc := newTestHomeHandler(t)
	user := &models.User{Email: "jane@x.com"}
	userRepo.addUser(user)
	cookie := sessionCookieFor(t, authSvc, user)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	h.Home(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 0, metricsRec.webhooksCreated)
	assert.Len(t, whRepo.webhooks, 0)

	for _, c := range rec.Result().Cookies() {
		assert.NotEqual(t, sessionIdName, c.Name, "logged-in users should not receive a guest cookie")
	}
}

func TestHomeHandler_Guest_NoCookie_CreateDefaultWebhookError(t *testing.T) {
	h, whRepo, _, metricsRec, _ := newTestHomeHandler(t)
	whRepo.insertErr = assert.AnError

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	h.Home(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, 0, metricsRec.webhooksCreated)
}

func TestHomeHandler_LoggedInUser_AddressLoadError(t *testing.T) {
	h, whRepo, userRepo, _, authSvc := newTestHomeHandler(t)
	user := &models.User{Email: "jane@x.com"}
	userRepo.addUser(user)
	whRepo.put(&models.Webhook{ID: "wh1", Title: "Mine", UserID: int(user.ID)})

	cookie := sessionCookieFor(t, authSvc, user)
	req := httptest.NewRequest(http.MethodGet, "/?address=missing", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	h.Home(rec, req)

	// The webhook lookup for `address` fails, but since the caller is logged
	// in, Home does not redirect - it just renders with an empty active
	// webhook. Note this endpoint does not verify that `address` belongs to
	// the logged-in user, so a valid ID for someone else's webhook would be
	// loaded here just the same (see reported IDOR-style concern).
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Mine") // sidebar still lists the user's own webhooks
}

func TestHomeHandler_ResponseHeadersMarshalled(t *testing.T) {
	h, whRepo, _, _, _ := newTestHomeHandler(t)
	whRepo.put(&models.Webhook{
		ID:              "wh1",
		Title:           "HeadersHook",
		ResponseHeaders: datatypes.JSONMap{"X-Test": "abc"},
	})

	req := httptest.NewRequest(http.MethodGet, "/?address=wh1", nil)
	rec := httptest.NewRecorder()

	h.Home(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "X-Test")
}
