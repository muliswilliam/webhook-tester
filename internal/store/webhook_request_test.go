package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"webhook-tester/internal/models"
)

func TestGormWebhookRequestRepo_InsertAndGetByID(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebhookRequestRepo(db, testLogger())

	require.NoError(t, db.Create(&models.Webhook{ID: "wh-1"}).Error)

	req := &models.WebhookRequest{ID: "req-1", WebhookID: "wh-1", Method: "GET", Body: "hello"}
	require.NoError(t, repo.Insert(req))

	got, err := repo.GetByID("req-1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "wh-1", got.WebhookID)
	assert.Equal(t, "hello", got.Body)
}

func TestGormWebhookRequestRepo_GetByID_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebhookRequestRepo(db, testLogger())

	got, err := repo.GetByID("missing")
	require.Error(t, err)
	assert.Nil(t, got)
}

func TestGormWebhookRequestRepo_ListByWebhook(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebhookRequestRepo(db, testLogger())

	require.NoError(t, db.Create(&models.Webhook{ID: "wh-1"}).Error)
	require.NoError(t, db.Create(&models.Webhook{ID: "wh-2"}).Error)

	now := time.Now().UTC()
	require.NoError(t, repo.Insert(&models.WebhookRequest{ID: "req-old", WebhookID: "wh-1", Method: "GET", ReceivedAt: now.Add(-time.Hour)}))
	require.NoError(t, repo.Insert(&models.WebhookRequest{ID: "req-new", WebhookID: "wh-1", Method: "POST", ReceivedAt: now}))
	require.NoError(t, repo.Insert(&models.WebhookRequest{ID: "req-other", WebhookID: "wh-2", Method: "PUT", ReceivedAt: now}))

	list, err := repo.ListByWebhook("wh-1")
	require.NoError(t, err)
	require.Len(t, list, 2)
	// Ordered received_at DESC.
	assert.Equal(t, "req-new", list[0].ID)
	assert.Equal(t, "req-old", list[1].ID)
}

func TestGormWebhookRequestRepo_ListByWebhook_NoResults(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebhookRequestRepo(db, testLogger())

	list, err := repo.ListByWebhook("does-not-exist")
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestGormWebhookRequestRepo_DeleteByID(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebhookRequestRepo(db, testLogger())

	require.NoError(t, db.Create(&models.Webhook{ID: "wh-1"}).Error)
	require.NoError(t, repo.Insert(&models.WebhookRequest{ID: "req-1", WebhookID: "wh-1", Method: "GET"}))

	require.NoError(t, repo.DeleteByID("req-1"))

	_, err := repo.GetByID("req-1")
	require.Error(t, err)
}

func TestGormWebhookRequestRepo_DeleteByID_NonExistentIsNoop(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebhookRequestRepo(db, testLogger())

	// GORM's Delete does not return ErrRecordNotFound when nothing matches.
	require.NoError(t, repo.DeleteByID("does-not-exist"))
}

func TestGormWebhookRequestRepo_DeleteByWebhook(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebhookRequestRepo(db, testLogger())

	require.NoError(t, db.Create(&models.Webhook{ID: "wh-1"}).Error)
	require.NoError(t, db.Create(&models.Webhook{ID: "wh-2"}).Error)
	require.NoError(t, repo.Insert(&models.WebhookRequest{ID: "req-1", WebhookID: "wh-1", Method: "GET"}))
	require.NoError(t, repo.Insert(&models.WebhookRequest{ID: "req-2", WebhookID: "wh-1", Method: "POST"}))
	require.NoError(t, repo.Insert(&models.WebhookRequest{ID: "req-3", WebhookID: "wh-2", Method: "PUT"}))

	require.NoError(t, repo.DeleteByWebhook("wh-1"))

	list1, err := repo.ListByWebhook("wh-1")
	require.NoError(t, err)
	assert.Empty(t, list1)

	list2, err := repo.ListByWebhook("wh-2")
	require.NoError(t, err)
	assert.Len(t, list2, 1)
}

func TestGormWebhookRequestRepo_DeleteByWebhook_NoMatches(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebhookRequestRepo(db, testLogger())

	require.NoError(t, repo.DeleteByWebhook("does-not-exist"))
}
