package store

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"webhook-tester/internal/models"
)

func TestGormUserRepo_CreateAndGetByID(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormUserRepo(db, testLogger())

	u := &models.User{FullName: "Jane Doe", Email: "jane@example.com"}
	require.NoError(t, repo.Create(u))
	require.NotZero(t, u.ID)

	got, err := repo.GetByID(u.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "jane@example.com", got.Email)
}

func TestGormUserRepo_GetByID_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormUserRepo(db, testLogger())

	got, err := repo.GetByID(999)
	require.Error(t, err)
	assert.Nil(t, got)
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
}

func TestGormUserRepo_GetByEmail(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormUserRepo(db, testLogger())

	u := &models.User{FullName: "Jane Doe", Email: "jane@example.com"}
	require.NoError(t, repo.Create(u))

	got, err := repo.GetByEmail("jane@example.com")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, u.ID, got.ID)
}

func TestGormUserRepo_GetByEmail_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormUserRepo(db, testLogger())

	got, err := repo.GetByEmail("missing@example.com")
	require.Error(t, err)
	assert.Nil(t, got)
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
}

func TestGormUserRepo_GetByResetToken(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormUserRepo(db, testLogger())

	u := &models.User{FullName: "Jane Doe", Email: "jane@example.com", ResetToken: "tok-123"}
	require.NoError(t, repo.Create(u))

	got, err := repo.GetByResetToken("tok-123")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, u.ID, got.ID)
}

func TestGormUserRepo_GetByResetToken_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormUserRepo(db, testLogger())

	got, err := repo.GetByResetToken("does-not-exist")
	require.Error(t, err)
	// GetByResetToken returns a non-nil zero-value user alongside the error.
	require.NotNil(t, got)
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
}

func TestGormUserRepo_GetByAPIKey(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormUserRepo(db, testLogger())

	u := &models.User{FullName: "Jane Doe", Email: "jane@example.com", APIKey: "key-123"}
	require.NoError(t, repo.Create(u))

	got, err := repo.GetByAPIKey("key-123")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, u.ID, got.ID)
}

func TestGormUserRepo_GetByAPIKey_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormUserRepo(db, testLogger())

	got, err := repo.GetByAPIKey("does-not-exist")
	require.Error(t, err)
	// GetByAPIKey returns a non-nil zero-value user alongside the error.
	require.NotNil(t, got)
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
}

func TestGormUserRepo_Update(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormUserRepo(db, testLogger())

	u := &models.User{FullName: "Jane Doe", Email: "jane@example.com", ResetToken: "old-token"}
	require.NoError(t, repo.Create(u))

	u.ResetToken = "new-token"
	require.NoError(t, repo.Update(u))

	got, err := repo.GetByID(u.ID)
	require.NoError(t, err)
	assert.Equal(t, "new-token", got.ResetToken)
}
