package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"webhook-tester/internal/models"
)

type fakeUserRepo struct {
	usersByID map[uint]*models.User
	nextID    uint

	createErr          error
	getByIDErr         error
	getByEmailErr      error
	getByResetTokenErr error
	updateErr          error
	getByAPIKeyErr     error
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{usersByID: make(map[uint]*models.User)}
}

func (f *fakeUserRepo) Create(user *models.User) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.nextID++
	user.ID = f.nextID
	f.usersByID[user.ID] = user
	return nil
}

func (f *fakeUserRepo) GetByID(id uint) (*models.User, error) {
	if f.getByIDErr != nil {
		return nil, f.getByIDErr
	}
	u, ok := f.usersByID[id]
	if !ok {
		return nil, assert.AnError
	}
	return u, nil
}

func (f *fakeUserRepo) GetByEmail(email string) (*models.User, error) {
	if f.getByEmailErr != nil {
		return nil, f.getByEmailErr
	}
	for _, u := range f.usersByID {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, assert.AnError
}

func (f *fakeUserRepo) GetByResetToken(token string) (*models.User, error) {
	if f.getByResetTokenErr != nil {
		return nil, f.getByResetTokenErr
	}
	for _, u := range f.usersByID {
		if u.ResetToken != "" && u.ResetToken == token {
			return u, nil
		}
	}
	return nil, assert.AnError
}

func (f *fakeUserRepo) Update(user *models.User) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.usersByID[user.ID] = user
	return nil
}

func (f *fakeUserRepo) GetByAPIKey(key string) (*models.User, error) {
	if f.getByAPIKeyErr != nil {
		return nil, f.getByAPIKeyErr
	}
	for _, u := range f.usersByID {
		if u.APIKey == key {
			return u, nil
		}
	}
	return nil, assert.AnError
}

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	return db
}

func newTestAuthService(t *testing.T) (*AuthService, *fakeUserRepo) {
	t.Helper()
	repo := newFakeUserRepo()
	db := newTestDB(t)
	svc := NewAuthService(repo, db, "test-secret-key-32-bytes-long!!")
	return svc, repo
}

func TestAuthService_Register_Success(t *testing.T) {
	svc, repo := newTestAuthService(t)

	user, err := svc.Register("alice@example.com", "s3cretPW", "Alice")
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "alice@example.com", user.Email)
	assert.NotEmpty(t, user.APIKey)
	assert.NotEqual(t, "s3cretPW", user.Password)
	assert.Len(t, repo.usersByID, 1)
}

func TestAuthService_Register_HashPasswordError(t *testing.T) {
	svc, _ := newTestAuthService(t)
	tooLong := make([]byte, 73)
	for i := range tooLong {
		tooLong[i] = 'a'
	}

	_, err := svc.Register("longpw@example.com", string(tooLong), "Long")
	require.Error(t, err)
}

func TestAuthService_Register_CreateError(t *testing.T) {
	svc, repo := newTestAuthService(t)
	repo.createErr = assert.AnError

	_, err := svc.Register("mia@example.com", "s3cretPW", "Mia")
	require.ErrorIs(t, err, assert.AnError)
}

func TestAuthService_Register_DuplicateEmail(t *testing.T) {
	svc, repo := newTestAuthService(t)
	repo.usersByID[1] = &models.User{Email: "bob@example.com"}
	repo.nextID = 1

	_, err := svc.Register("bob@example.com", "s3cretPW", "Bob")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email already taken")
}

func TestAuthService_Authenticate(t *testing.T) {
	svc, repo := newTestAuthService(t)
	user, err := svc.Register("carl@example.com", "s3cretPW", "Carl")
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		u, err := svc.Authenticate("carl@example.com", "s3cretPW")
		require.NoError(t, err)
		assert.Equal(t, user.ID, u.ID)
	})

	t.Run("unknown email", func(t *testing.T) {
		_, err := svc.Authenticate("nobody@example.com", "s3cretPW")
		require.Error(t, err)
		assert.Equal(t, "invalid credentials", err.Error())
	})

	t.Run("wrong password", func(t *testing.T) {
		_, err := svc.Authenticate("carl@example.com", "wrongpassword")
		require.Error(t, err)
		assert.Equal(t, "invalid credentials", err.Error())
	})

	_ = repo
}

func TestAuthService_SessionRoundTrip(t *testing.T) {
	svc, repo := newTestAuthService(t)
	user := &models.User{Email: "dana@example.com"}
	require.NoError(t, repo.Create(user))

	initReq := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	err := svc.CreateSession(rec, initReq, user)
	require.NoError(t, err)

	result := rec.Result()
	cookies := result.Cookies()
	require.NotEmpty(t, cookies)

	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "_webhook_tester_session_id" {
			sessionCookie = c
		}
	}
	require.NotNil(t, sessionCookie)

	authedReq := httptest.NewRequest(http.MethodGet, "/", nil)
	authedReq.AddCookie(sessionCookie)

	uid, err := svc.Authorize(authedReq)
	require.NoError(t, err)
	assert.Equal(t, user.ID, uid)

	authedReq2 := httptest.NewRequest(http.MethodGet, "/", nil)
	authedReq2.AddCookie(sessionCookie)
	current, err := svc.GetCurrentUser(authedReq2)
	require.NoError(t, err)
	assert.Equal(t, user.ID, current.ID)

	clearReq := httptest.NewRequest(http.MethodGet, "/", nil)
	clearReq.AddCookie(sessionCookie)
	clearRec := httptest.NewRecorder()
	svc.ClearSession(clearRec, clearReq)
	clearCookies := clearRec.Result().Cookies()
	require.NotEmpty(t, clearCookies)
	assert.Equal(t, -1, clearCookies[0].MaxAge)
}

func TestAuthService_Authorize_NoCookie(t *testing.T) {
	svc, _ := newTestAuthService(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	_, err := svc.Authorize(req)
	require.Error(t, err)
	assert.Equal(t, "unauthorized", err.Error())
}

func TestAuthService_Authorize_GarbageCookie(t *testing.T) {
	svc, _ := newTestAuthService(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "_webhook_tester_session_id", Value: "garbage-not-valid"})

	_, err := svc.Authorize(req)
	require.Error(t, err)
	assert.Equal(t, "unauthorized", err.Error())
}

func TestAuthService_GetCurrentUser_Unauthorized(t *testing.T) {
	svc, _ := newTestAuthService(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	_, err := svc.GetCurrentUser(req)
	require.Error(t, err)
	assert.Equal(t, "unauthorized", err.Error())
}

func TestAuthService_ClearSession_NoSession(t *testing.T) {
	svc, _ := newTestAuthService(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	svc.ClearSession(rec, req)
}

func TestAuthService_ForgotPassword(t *testing.T) {
	svc, repo := newTestAuthService(t)

	t.Run("unknown email", func(t *testing.T) {
		_, err := svc.ForgotPassword("ghost@example.com", "http://example.com")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "user not found")
	})

	t.Run("success", func(t *testing.T) {
		user := &models.User{Email: "eve@example.com"}
		require.NoError(t, repo.Create(user))

		url, err := svc.ForgotPassword("eve@example.com", "http://example.com")
		require.NoError(t, err)
		assert.Contains(t, url, "reset-password?token=")
		assert.NotEmpty(t, user.ResetToken)
		assert.True(t, user.ResetTokenExpiry.After(time.Now()))
	})

	t.Run("repo update error", func(t *testing.T) {
		user := &models.User{Email: "fay@example.com"}
		require.NoError(t, repo.Create(user))
		repo.updateErr = assert.AnError

		_, err := svc.ForgotPassword("fay@example.com", "http://example.com")
		require.ErrorIs(t, err, assert.AnError)
		repo.updateErr = nil
	})
}

func TestAuthService_ValidateResetToken(t *testing.T) {
	svc, repo := newTestAuthService(t)

	t.Run("unknown token", func(t *testing.T) {
		_, err := svc.ValidateResetToken("nope")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid or expired token")
	})

	t.Run("expired token", func(t *testing.T) {
		user := &models.User{Email: "frank@example.com", ResetToken: "expiredtoken", ResetTokenExpiry: time.Now().Add(-time.Hour)}
		require.NoError(t, repo.Create(user))

		_, err := svc.ValidateResetToken("expiredtoken")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid or expired token")
	})

	t.Run("valid token", func(t *testing.T) {
		user := &models.User{Email: "grace@example.com", ResetToken: "validtoken", ResetTokenExpiry: time.Now().Add(time.Hour)}
		require.NoError(t, repo.Create(user))

		u, err := svc.ValidateResetToken("validtoken")
		require.NoError(t, err)
		assert.Equal(t, user.ID, u.ID)
	})
}

func TestAuthService_ResetPassword(t *testing.T) {
	svc, repo := newTestAuthService(t)

	t.Run("invalid or expired token", func(t *testing.T) {
		err := svc.ResetPassword("nope", "NewPassw0rd")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid or expired reset link")
	})

	t.Run("expired token", func(t *testing.T) {
		user := &models.User{Email: "henry@example.com", ResetToken: "expiredtoken2", ResetTokenExpiry: time.Now().Add(-time.Hour)}
		require.NoError(t, repo.Create(user))

		err := svc.ResetPassword("expiredtoken2", "NewPassw0rd")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid or expired reset link")
	})

	t.Run("password rule failure", func(t *testing.T) {
		user := &models.User{Email: "ivy@example.com", ResetToken: "weaktoken", ResetTokenExpiry: time.Now().Add(time.Hour)}
		require.NoError(t, repo.Create(user))

		err := svc.ResetPassword("weaktoken", "weak")
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "invalid or expired reset link")
	})

	t.Run("success", func(t *testing.T) {
		user := &models.User{Email: "jack@example.com", ResetToken: "goodtoken", ResetTokenExpiry: time.Now().Add(time.Hour), Password: "oldhash"}
		require.NoError(t, repo.Create(user))

		err := svc.ResetPassword("goodtoken", "NewPassw0rd")
		require.NoError(t, err)
		assert.Empty(t, user.ResetToken)
		assert.True(t, user.ResetTokenExpiry.IsZero())
		assert.NotEqual(t, "oldhash", user.Password)
	})

	t.Run("hash password error", func(t *testing.T) {
		user := &models.User{Email: "liam@example.com", ResetToken: "hashfailtoken", ResetTokenExpiry: time.Now().Add(time.Hour)}
		require.NoError(t, repo.Create(user))

		tooLong := "Aa1" + string(make([]byte, 73))
		err := svc.ResetPassword("hashfailtoken", tooLong)
		require.Error(t, err)
	})
}

func TestAuthService_ValidateAPIKey(t *testing.T) {
	svc, repo := newTestAuthService(t)
	user := &models.User{Email: "kate@example.com", APIKey: "key123"}
	require.NoError(t, repo.Create(user))

	u, err := svc.ValidateAPIKey("key123")
	require.NoError(t, err)
	assert.Equal(t, user.ID, u.ID)

	_, err = svc.ValidateAPIKey("missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid API key")
}
