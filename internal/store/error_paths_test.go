package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"webhook-tester/internal/models"
)

func TestGormWebhookRepo_Insert_DuplicateIDErrors(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebookRepo(db, testLogger())

	wh := &models.Webhook{ID: "dup", Title: "first"}
	require.NoError(t, repo.Insert(wh))

	dup := &models.Webhook{ID: "dup", Title: "second"}
	err := repo.Insert(dup)
	assert.Error(t, err)
}

func TestGormWebhookRepo_ErrorsAfterConnectionClosed(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebookRepo(db, testLogger())

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	assert.Error(t, repo.Insert(&models.Webhook{ID: "x"}))
	assert.Error(t, repo.Update(&models.Webhook{ID: "x"}))

	_, err = repo.GetAll()
	assert.Error(t, err)

	_, err = repo.GetAllByUser(1)
	assert.Error(t, err)
}

func TestGormWebhookRepo_Delete_OwnershipCheckGenericError(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebookRepo(db, testLogger())
	require.NoError(t, repo.Insert(&models.Webhook{ID: "wh-1", Title: "a", UserID: 5}))

	// Dropping the webhooks table entirely means the transaction still
	// begins successfully, but the ownership-check SELECT fails with a
	// generic (non-ErrRecordNotFound) error, exercising the "error
	// checking webhook ownership" logging branch (the else side of the
	// errors.Is(err, gorm.ErrRecordNotFound) check).
	require.NoError(t, db.Migrator().DropTable(&models.Webhook{}))

	err := repo.Delete("wh-1", 5)
	assert.Error(t, err)
}

func TestGormWebhookRepo_Delete_RequestsDeleteFails(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebookRepo(db, testLogger())
	require.NoError(t, repo.Insert(&models.Webhook{ID: "wh-1", Title: "a", UserID: 5}))

	// Dropping the webhook_requests table makes the "delete associated
	// requests" step of the transaction fail even though the ownership
	// check succeeds, exercising the "failed to delete webhook requests"
	// logging branch.
	require.NoError(t, db.Migrator().DropTable(&models.WebhookRequest{}))

	err := repo.Delete("wh-1", 5)
	assert.Error(t, err)
}

func TestGormWebhookRepo_Delete_WebhookDeleteFails(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebookRepo(db, testLogger())
	require.NoError(t, repo.Insert(&models.Webhook{ID: "wh-1", Title: "a", UserID: 5}))

	// A BEFORE DELETE trigger that always aborts makes the final "delete
	// the webhook itself" step fail, exercising the "failed to delete
	// webhook" logging branch.
	require.NoError(t, db.Exec(`
		CREATE TRIGGER block_webhook_delete
		BEFORE DELETE ON webhooks
		BEGIN
			SELECT RAISE(ABORT, 'blocked');
		END;
	`).Error)

	err := repo.Delete("wh-1", 5)
	assert.Error(t, err)
}

func TestGormWebhookRepo_CleanPublic_DeleteRequestsFails(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebookRepo(db, testLogger())

	wh := &models.Webhook{ID: "pub-1", Title: "a", UserID: 0, CreatedAt: time.Now().UTC().Add(-2 * time.Hour)}
	require.NoError(t, db.Create(wh).Error)

	// Dropping the webhook_requests table makes the request-deletion step
	// inside CleanPublic's transaction fail, exercising both the inner
	// ("Error deleting webhooks") and outer ("error cleaning public
	// webhooks") logging branches.
	require.NoError(t, db.Migrator().DropTable(&models.WebhookRequest{}))

	ids, err := repo.CleanPublic(time.Hour)
	assert.Error(t, err)
	assert.Nil(t, ids)
}

func TestGormWebhookRequestRepo_Insert_DuplicateIDErrors(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebhookRequestRepo(db, testLogger())

	require.NoError(t, db.Create(&models.Webhook{ID: "wh-1"}).Error)
	require.NoError(t, repo.Insert(&models.WebhookRequest{ID: "dup", WebhookID: "wh-1", Method: "GET"}))

	err := repo.Insert(&models.WebhookRequest{ID: "dup", WebhookID: "wh-1", Method: "POST"})
	assert.Error(t, err)
}

// Dropping the deliveries table makes the first step of each delete path
// fail, which must abort the transaction and leave the requests and webhooks
// in place.
func TestDeletePaths_DeliveriesDeleteFails(t *testing.T) {
	db := newTestDB(t)
	webhooks := NewGormWebookRepo(db, testLogger())
	requests := NewGormWebhookRequestRepo(db, testLogger())
	require.NoError(t, db.Create(&models.Webhook{ID: "owned", Title: "a", UserID: 5}).Error)
	require.NoError(t, db.Create(&models.Webhook{ID: "pub", Title: "b", CreatedAt: time.Now().UTC().Add(-2 * time.Hour)}).Error)
	require.NoError(t, db.Create(&models.WebhookRequest{ID: "req-1", WebhookID: "owned"}).Error)
	require.NoError(t, db.Migrator().DropTable(&models.Delivery{}))

	assert.Error(t, requests.DeleteByID("req-1"))
	assert.Error(t, requests.DeleteByWebhook("owned"))
	assert.Error(t, webhooks.Delete("owned", 5))
	ids, err := webhooks.CleanPublic(time.Hour)
	assert.Error(t, err)
	assert.Nil(t, ids)

	var reqCount, whCount int64
	require.NoError(t, db.Model(&models.WebhookRequest{}).Count(&reqCount).Error)
	require.NoError(t, db.Model(&models.Webhook{}).Count(&whCount).Error)
	assert.Equal(t, int64(1), reqCount)
	assert.Equal(t, int64(2), whCount)
}

func TestGormWebhookRequestRepo_ErrorsAfterConnectionClosed(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebhookRequestRepo(db, testLogger())

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	assert.Error(t, repo.Insert(&models.WebhookRequest{ID: "req-1"}))

	_, err = repo.GetByID("req-1")
	assert.Error(t, err)

	_, err = repo.ListByWebhook("wh-1")
	assert.Error(t, err)

	assert.Error(t, repo.DeleteByID("req-1"))
	assert.Error(t, repo.DeleteByWebhook("wh-1"))
}

func TestGormUserRepo_Create_DuplicateEmailErrors(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormUserRepo(db, testLogger())

	require.NoError(t, repo.Create(&models.User{Email: "dup@example.com"}))
	err := repo.Create(&models.User{Email: "dup@example.com"})
	assert.Error(t, err)
}

func TestGormUserRepo_Update_DuplicateEmailErrors(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormUserRepo(db, testLogger())

	require.NoError(t, repo.Create(&models.User{Email: "one@example.com"}))
	u2 := &models.User{Email: "two@example.com"}
	require.NoError(t, repo.Create(u2))

	u2.Email = "one@example.com"
	err := repo.Update(u2)
	assert.Error(t, err)
}

func TestGormUserRepo_ErrorsAfterConnectionClosed(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormUserRepo(db, testLogger())

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	_, err = repo.GetByEmail("jane@example.com")
	assert.Error(t, err)

	_, err = repo.GetByID(1)
	assert.Error(t, err)

	_, err = repo.GetByResetToken("tok")
	assert.Error(t, err)

	_, err = repo.GetByAPIKey("key")
	assert.Error(t, err)
}
