package middlewares_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"webhook-tester/internal/middlewares"
	"webhook-tester/internal/models"
	"webhook-tester/internal/service"
)

// fakeUserRepo is an in-memory implementation of repository.UserRepository,
// backed by a map keyed by API key. Only GetByAPIKey is exercised by the
// tests below; the other methods are stubbed out.
type fakeUserRepo struct {
	byAPIKey map[string]*models.User
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{byAPIKey: make(map[string]*models.User)}
}

func (f *fakeUserRepo) Create(_ *models.User) error {
	return errors.New("not implemented")
}

func (f *fakeUserRepo) GetByID(_ uint) (*models.User, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeUserRepo) GetByEmail(_ string) (*models.User, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeUserRepo) GetByResetToken(_ string) (*models.User, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeUserRepo) Update(_ *models.User) error {
	return errors.New("not implemented")
}

func (f *fakeUserRepo) GetByAPIKey(key string) (*models.User, error) {
	user, ok := f.byAPIKey[key]
	if !ok {
		return nil, errors.New("user not found")
	}
	return user, nil
}

// newTestAuthService builds a real *service.AuthService backed by the fake
// repo above and an in-memory sqlite gorm.DB (used only for the session
// store, which auto-migrates its own table and isn't touched by
// ValidateAPIKey).
func newTestAuthService(t *testing.T, repo *fakeUserRepo) *service.AuthService {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	return service.NewAuthService(repo, db, "test-secret")
}

func TestRequireAPIKey_MissingHeader(t *testing.T) {
	repo := newFakeUserRepo()
	authSvc := newTestAuthService(t, repo)

	called := false
	final := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := middlewares.RequireAPIKey(authSvc)(final)

	req := httptest.NewRequest(http.MethodGet, "/webhooks", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, called, "wrapped handler should not be called when API key is missing")
}

func TestRequireAPIKey_InvalidKey(t *testing.T) {
	repo := newFakeUserRepo()
	authSvc := newTestAuthService(t, repo)

	called := false
	final := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := middlewares.RequireAPIKey(authSvc)(final)

	req := httptest.NewRequest(http.MethodGet, "/webhooks", nil)
	req.Header.Set("X-API-Key", "does-not-exist")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, called, "wrapped handler should not be called when API key is invalid")
}

func TestRequireAPIKey_ValidKey(t *testing.T) {
	repo := newFakeUserRepo()
	expectedUser := &models.User{
		FullName: "Jane Doe",
		Email:    "jane@example.com",
		APIKey:   "valid-key-123",
	}
	expectedUser.ID = 42
	repo.byAPIKey["valid-key-123"] = expectedUser

	authSvc := newTestAuthService(t, repo)

	var gotUser *models.User
	called := false
	final := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		gotUser = middlewares.GetAPIAuthenticatedUser(r)
		w.WriteHeader(http.StatusOK)
	})

	handler := middlewares.RequireAPIKey(authSvc)(final)

	req := httptest.NewRequest(http.MethodGet, "/webhooks", nil)
	req.Header.Set("X-API-Key", "valid-key-123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, called, "wrapped handler should be called when API key is valid")
	require.NotNil(t, gotUser)
	assert.Equal(t, expectedUser.ID, gotUser.ID)
	assert.Equal(t, expectedUser.Email, gotUser.Email)
	assert.Equal(t, expectedUser.FullName, gotUser.FullName)
}

func TestGetAPIAuthenticatedUser_NoUserInContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/webhooks", nil)

	user := middlewares.GetAPIAuthenticatedUser(req)

	assert.Nil(t, user)
}
