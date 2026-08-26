package handlers

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"webhook-tester/internal/models"
	"webhook-tester/internal/service"
	"webhook-tester/internal/utils"
)

func newTestAuthHandler(t *testing.T) (*AuthHandler, *testUserRepo, *testMetricsRecorder, *service.AuthService) {
	t.Helper()
	userRepo := newTestUserRepo()
	metricsRec := &testMetricsRecorder{}
	authSvc := newTestAuthService(t, userRepo)
	h := NewAuthHandler(authSvc, newTestLogger(), metricsRec)
	return h, userRepo, metricsRec, authSvc
}

func newTestAuthHandlerWithLogBuf(t *testing.T) (*AuthHandler, *testUserRepo, *testMetricsRecorder, *service.AuthService, *bytes.Buffer) {
	t.Helper()
	userRepo := newTestUserRepo()
	metricsRec := &testMetricsRecorder{}
	authSvc := newTestAuthService(t, userRepo)
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	h := NewAuthHandler(authSvc, logger, metricsRec)
	return h, userRepo, metricsRec, authSvc, &buf
}

func futureTime() time.Time {
	return time.Now().Add(time.Hour)
}

func TestAuthHandler_RegisterGet(t *testing.T) {
	h, _, _, _ := newTestAuthHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/register", nil)
	rec := httptest.NewRecorder()

	h.RegisterGet(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Create your workspace")
}

func TestAuthHandler_RegisterPost_ParseFormError(t *testing.T) {
	h, _, _, _ := newTestAuthHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/register?a=%zz", nil)
	rec := httptest.NewRecorder()

	h.RegisterPost(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAuthHandler_RegisterPost_WeakPassword(t *testing.T) {
	h, _, metricsRec, _ := newTestAuthHandler(t)
	form := url.Values{"name": {"Jane"}, "email": {"jane@x.com"}, "password": {"weak"}}
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.RegisterPost(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Password must be")
	assert.Equal(t, 0, metricsRec.signUps)
}

func TestAuthHandler_RegisterPost_DuplicateEmail(t *testing.T) {
	h, userRepo, metricsRec, _ := newTestAuthHandler(t)
	userRepo.addUser(&models.User{Email: "jane@x.com"})

	form := url.Values{"name": {"Jane"}, "email": {"jane@x.com"}, "password": {"Passw0rd!"}}
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.RegisterPost(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "already registered")
	assert.Equal(t, 0, metricsRec.signUps)
}

func TestAuthHandler_RegisterPost_UnexpectedError(t *testing.T) {
	h, userRepo, metricsRec, _ := newTestAuthHandler(t)
	userRepo.createErr = assert.AnError

	form := url.Values{"name": {"Jane"}, "email": {"jane@x.com"}, "password": {"Passw0rd!"}}
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.RegisterPost(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, 0, metricsRec.signUps)
}

func TestAuthHandler_RegisterPost_Success(t *testing.T) {
	h, userRepo, metricsRec, _ := newTestAuthHandler(t)

	form := url.Values{"name": {"Jane"}, "email": {"jane@x.com"}, "password": {"Passw0rd!"}}
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.RegisterPost(rec, req)

	require.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/login", rec.Header().Get("Location"))
	assert.Equal(t, 1, metricsRec.signUps)
	_, ok := userRepo.byEmail["jane@x.com"]
	assert.True(t, ok)
}

func TestAuthHandler_LoginGet(t *testing.T) {
	h, _, _, _ := newTestAuthHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rec := httptest.NewRecorder()

	h.LoginGet(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Sign in to your workspace")
}

func TestAuthHandler_LoginPost_ParseFormError(t *testing.T) {
	h, _, _, _ := newTestAuthHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/login?a=%zz", nil)
	rec := httptest.NewRecorder()

	h.LoginPost(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAuthHandler_LoginPost_InvalidCredentials(t *testing.T) {
	h, _, metricsRec, _ := newTestAuthHandler(t)

	form := url.Values{"email": {"nouser@x.com"}, "password": {"whatever"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.LoginPost(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invalid email or password")
	assert.Equal(t, 0, metricsRec.logins)
}

func TestAuthHandler_LoginPost_Success(t *testing.T) {
	h, userRepo, metricsRec, _ := newTestAuthHandler(t)
	hash, err := utils.HashPassword("Passw0rd!")
	require.NoError(t, err)
	userRepo.addUser(&models.User{Email: "jane@x.com", Password: hash})

	form := url.Values{"email": {"jane@x.com"}, "password": {"Passw0rd!"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: sessionIdName, Value: "guest-webhook-id"})
	rec := httptest.NewRecorder()

	h.LoginPost(rec, req)

	require.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/", rec.Header().Get("Location"))
	assert.Equal(t, 1, metricsRec.logins)

	var sawAuthCookie, sawInvalidatedGuestCookie bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == testSessionCookieName {
			sawAuthCookie = true
		}
		if c.Name == sessionIdName && c.MaxAge == -1 {
			sawInvalidatedGuestCookie = true
		}
	}
	assert.True(t, sawAuthCookie, "expected the real auth session cookie to be set")
	assert.True(t, sawInvalidatedGuestCookie, "expected the guest cookie to be invalidated on login")
}

func TestAuthHandler_LoginPost_CreateSessionError(t *testing.T) {
	userRepo := newTestUserRepo()
	metricsRec := &testMetricsRecorder{}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	authSvc := service.NewAuthService(userRepo, db, "test-secret")
	h := NewAuthHandler(authSvc, newTestLogger(), metricsRec)

	hash, err := utils.HashPassword("Passw0rd!")
	require.NoError(t, err)
	userRepo.addUser(&models.User{Email: "jane@x.com", Password: hash})

	// Break the session store's backing DB so CreateSession fails when it
	// tries to persist the new session row.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	form := url.Values{"email": {"jane@x.com"}, "password": {"Passw0rd!"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.LoginPost(rec, req)

	// LoginPost returns immediately after writing the 500 error response on
	// a CreateSession failure, so the login metric must NOT be incremented
	// for a login that did not actually succeed.
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, 0, metricsRec.logins)
}

func TestAuthHandler_Logout(t *testing.T) {
	h, userRepo, _, authSvc := newTestAuthHandler(t)
	user := &models.User{Email: "jane@x.com"}
	userRepo.addUser(user)
	cookie := sessionCookieFor(t, authSvc, user)

	req := httptest.NewRequest(http.MethodGet, "/logout", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	h.Logout(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/", rec.Header().Get("Location"))
}

func TestAuthHandler_ForgotPasswordGet(t *testing.T) {
	h, _, _, _ := newTestAuthHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/forgot-password", nil)
	rec := httptest.NewRecorder()

	h.ForgotPasswordGet(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthHandler_ForgotPasswordPost_ParseFormError(t *testing.T) {
	h, _, _, _ := newTestAuthHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/forgot-password?a=%zz", nil)
	rec := httptest.NewRecorder()

	h.ForgotPasswordPost(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAuthHandler_ForgotPasswordPost_UnknownUser(t *testing.T) {
	h, _, _, _, buf := newTestAuthHandlerWithLogBuf(t)

	form := url.Values{"email": {"nouser@x.com"}}
	req := httptest.NewRequest(http.MethodPost, "/forgot-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.ForgotPasswordPost(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), "Check your inbox")
	assert.Contains(t, buf.String(), "Forgot password error")
}

func TestAuthHandler_ForgotPasswordPost_Success(t *testing.T) {
	h, userRepo, _, _, buf := newTestAuthHandlerWithLogBuf(t)
	userRepo.addUser(&models.User{Email: "jane@x.com"})

	form := url.Values{"email": {"jane@x.com"}}
	req := httptest.NewRequest(http.MethodPost, "/forgot-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.ForgotPasswordPost(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Check your inbox")
	assert.Contains(t, buf.String(), "Password reset link")
}

func TestAuthHandler_ResetPasswordGet_MissingToken(t *testing.T) {
	h, _, _, _ := newTestAuthHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/reset-password", nil)
	rec := httptest.NewRecorder()

	h.ResetPasswordGet(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Missing token")
}

func TestAuthHandler_ResetPasswordGet_InvalidToken(t *testing.T) {
	h, _, _, _ := newTestAuthHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/reset-password?token=bogus", nil)
	rec := httptest.NewRecorder()

	h.ResetPasswordGet(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invalid or expired reset link")
}

func TestAuthHandler_ResetPasswordGet_ValidToken(t *testing.T) {
	h, userRepo, _, _ := newTestAuthHandler(t)
	userRepo.addUser(&models.User{Email: "jane@x.com", ResetToken: "tok123", ResetTokenExpiry: futureTime()})

	req := httptest.NewRequest(http.MethodGet, "/reset-password?token=tok123", nil)
	rec := httptest.NewRecorder()

	h.ResetPasswordGet(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `value="tok123"`)
}

func TestAuthHandler_ResetPasswordPost_ParseFormError(t *testing.T) {
	h, _, _, _ := newTestAuthHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/reset-password?a=%zz", nil)
	rec := httptest.NewRecorder()

	h.ResetPasswordPost(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAuthHandler_ResetPasswordPost_Mismatch(t *testing.T) {
	h, _, _, _ := newTestAuthHandler(t)
	form := url.Values{"token": {"tok"}, "password": {"Passw0rd!"}, "confirm_password": {"Other1234!"}}
	req := httptest.NewRequest(http.MethodPost, "/reset-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.ResetPasswordPost(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Passwords do not match")
}

func TestAuthHandler_ResetPasswordPost_ServiceError(t *testing.T) {
	h, _, _, _ := newTestAuthHandler(t)
	form := url.Values{"token": {"bad-token"}, "password": {"Passw0rd!"}, "confirm_password": {"Passw0rd!"}}
	req := httptest.NewRequest(http.MethodPost, "/reset-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.ResetPasswordPost(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid or expired")
}

func TestAuthHandler_ResetPasswordPost_Success(t *testing.T) {
	h, userRepo, _, _ := newTestAuthHandler(t)
	userRepo.addUser(&models.User{Email: "jane@x.com", ResetToken: "tok123", ResetTokenExpiry: futureTime()})

	form := url.Values{"token": {"tok123"}, "password": {"NewPassw0rd!"}, "confirm_password": {"NewPassw0rd!"}}
	req := httptest.NewRequest(http.MethodPost, "/reset-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.ResetPasswordPost(rec, req)

	require.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/login", rec.Header().Get("Location"))

	u, ok := userRepo.byEmail["jane@x.com"]
	require.True(t, ok)
	assert.True(t, utils.CheckPasswordHash("NewPassw0rd!", u.Password))
	assert.Empty(t, u.ResetToken)
}
