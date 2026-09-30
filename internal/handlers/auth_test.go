package handlers

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
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

// authFixture is an AuthHandler wired to in-memory fakes.
type authFixture struct {
	h           *AuthHandler
	userRepo    *testUserRepo
	webhookRepo *testWebhookRepo
	metrics     *testMetricsRecorder
	authSvc     *service.AuthService
	mailer      *testMailer
	logs        *syncBuffer
}

func newAuthFixture(t *testing.T) *authFixture {
	t.Helper()
	f := &authFixture{
		userRepo:    newTestUserRepo(),
		webhookRepo: newTestWebhookRepo(),
		metrics:     &testMetricsRecorder{},
		mailer:      newTestMailer(),
		logs:        &syncBuffer{},
	}
	f.authSvc = newTestAuthService(t, f.userRepo)
	f.h = NewAuthHandler(f.authSvc, service.NewWebhookService(f.webhookRepo), f.mailer, log.New(f.logs, "", 0), f.metrics)
	return f
}

func newTestAuthHandler(t *testing.T) (*AuthHandler, *testUserRepo, *testMetricsRecorder, *service.AuthService) {
	t.Helper()
	f := newAuthFixture(t)
	return f.h, f.userRepo, f.metrics, f.authSvc
}

// testMailer records sent emails on a channel, since password reset emails
// are sent in the background.
type testMailer struct {
	sent chan sentMail
}

type sentMail struct {
	to, subject, body string
}

func newTestMailer() *testMailer {
	return &testMailer{sent: make(chan sentMail, 10)}
}

func (m *testMailer) Send(to, subject, body string) error {
	m.sent <- sentMail{to: to, subject: subject, body: body}
	return nil
}

// syncBuffer is a bytes.Buffer safe for the background email goroutine to
// log into while a test reads it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func postForm(target string, form url.Values) *http.Request {
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

// flashFrom returns the flash notice a response queued, or nil.
func flashFrom(t *testing.T, rec *httptest.ResponseRecorder) *utils.Flash {
	t.Helper()
	next := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range rec.Result().Cookies() {
		next.AddCookie(c)
	}
	return utils.PopFlash(httptest.NewRecorder(), next)
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
	f := newAuthFixture(t)

	rec := httptest.NewRecorder()
	f.h.RegisterPost(rec, postForm("/register", url.Values{"name": {"Jane"}, "email": {"jane@x.com"}, "password": {"Passw0rd!"}}))

	require.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/", rec.Header().Get("Location"))
	assert.Equal(t, 1, f.metrics.signUps)
	_, ok := f.userRepo.byEmail["jane@x.com"]
	assert.True(t, ok)
	assert.True(t, hasCookie(rec, testSessionCookieName), "expected the new user to be signed in")
	flash := flashFrom(t, rec)
	require.NotNil(t, flash)
	assert.Equal(t, utils.FlashSuccess, flash.Kind)
}

func TestAuthHandler_RegisterPost_ClaimsGuestWorkspace(t *testing.T) {
	f := newAuthFixture(t)
	f.webhookRepo.put(&models.Webhook{ID: "guest-wh"})

	req := postForm("/register", url.Values{"name": {"Jane"}, "email": {"jane@x.com"}, "password": {"Passw0rd!"}})
	req.AddCookie(&http.Cookie{Name: sessionIdName, Value: "guest-wh"})
	rec := httptest.NewRecorder()
	f.h.RegisterPost(rec, req)

	require.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/?address=guest-wh", rec.Header().Get("Location"))
	user := f.userRepo.byEmail["jane@x.com"]
	assert.Equal(t, int(user.ID), f.webhookRepo.webhooks["guest-wh"].UserID)
	assert.True(t, guestCookieCleared(rec))
	assert.Contains(t, flashFrom(t, rec).Message, "saved to your account")
}

func TestAuthHandler_RegisterPost_ExpiredGuestWorkspace(t *testing.T) {
	f := newAuthFixture(t)

	req := postForm("/register", url.Values{"name": {"Jane"}, "email": {"jane@x.com"}, "password": {"Passw0rd!"}})
	req.AddCookie(&http.Cookie{Name: sessionIdName, Value: "gone"})
	rec := httptest.NewRecorder()
	f.h.RegisterPost(rec, req)

	require.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/", rec.Header().Get("Location"))
	assert.True(t, guestCookieCleared(rec))
}

func hasCookie(rec *httptest.ResponseRecorder, name string) bool {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name && c.MaxAge >= 0 {
			return true
		}
	}
	return false
}

func guestCookieCleared(rec *httptest.ResponseRecorder) bool {
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionIdName && c.MaxAge == -1 {
			return true
		}
	}
	return false
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
	assert.Contains(t, rec.Body.String(), `value="nouser@x.com"`, "expected the email to be kept")
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

func TestAuthHandler_LoginPost_ClaimsGuestWorkspace(t *testing.T) {
	f := newAuthFixture(t)
	hash, err := utils.HashPassword("Passw0rd!")
	require.NoError(t, err)
	user := &models.User{Email: "jane@x.com", Password: hash}
	f.userRepo.addUser(user)
	f.webhookRepo.put(&models.Webhook{ID: "guest-wh"})

	req := postForm("/login", url.Values{"email": {"jane@x.com"}, "password": {"Passw0rd!"}})
	req.AddCookie(&http.Cookie{Name: sessionIdName, Value: "guest-wh"})
	rec := httptest.NewRecorder()
	f.h.LoginPost(rec, req)

	require.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/?address=guest-wh", rec.Header().Get("Location"))
	assert.Equal(t, int(user.ID), f.webhookRepo.webhooks["guest-wh"].UserID)
	assert.True(t, guestCookieCleared(rec))
}

func TestAuthHandler_LoginPost_CreateSessionError(t *testing.T) {
	userRepo := newTestUserRepo()
	metricsRec := &testMetricsRecorder{}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	authSvc := service.NewAuthService(userRepo, db, "test-secret")
	h := NewAuthHandler(authSvc, service.NewWebhookService(newTestWebhookRepo()), newTestMailer(), newTestLogger(), metricsRec)

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
	f := newAuthFixture(t)

	rec := httptest.NewRecorder()
	f.h.ForgotPasswordPost(rec, postForm("/forgot-password", url.Values{"email": {"nouser@x.com"}}))

	// Same answer as for a registered email, so accounts can't be enumerated.
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "If an account exists for that email")
	assert.Eventually(t, func() bool { return strings.Contains(f.logs.String(), "forgot password") }, time.Second, 5*time.Millisecond)
	assert.Empty(t, f.mailer.sent)
}

func TestAuthHandler_ForgotPasswordPost_Success(t *testing.T) {
	f := newAuthFixture(t)
	f.userRepo.addUser(&models.User{Email: "jane@x.com"})
	t.Setenv("DOMAIN", "http://example.com")

	rec := httptest.NewRecorder()
	f.h.ForgotPasswordPost(rec, postForm("/forgot-password", url.Values{"email": {" jane@x.com "}}))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "If an account exists for that email")
	select {
	case mail := <-f.mailer.sent:
		assert.Equal(t, "jane@x.com", mail.to)
		assert.Equal(t, "Reset your Webhook Tester password", mail.subject)
		token := f.userRepo.byEmail["jane@x.com"].ResetToken
		require.NotEmpty(t, token)
		assert.Contains(t, mail.body, "http://example.com/reset-password?token="+token)
	case <-time.After(time.Second):
		t.Fatal("expected a password reset email")
	}
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
	assert.Equal(t, &utils.Flash{Kind: utils.FlashSuccess, Message: "Password updated. Sign in with your new password."}, flashFrom(t, rec))

	u, ok := userRepo.byEmail["jane@x.com"]
	require.True(t, ok)
	assert.True(t, utils.CheckPasswordHash("NewPassw0rd!", u.Password))
	assert.Empty(t, u.ResetToken)
}
