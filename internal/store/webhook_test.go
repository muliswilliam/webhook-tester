package store

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"webhook-tester/internal/models"
)

func TestGormWebhookRepo_InsertAndGet(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebookRepo(db, testLogger())

	wh := &models.Webhook{ID: "wh-1", Title: "hello", UserID: 0}
	require.NoError(t, repo.Insert(wh))

	got, err := repo.Get("wh-1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "wh-1", got.ID)
	assert.Equal(t, "hello", got.Title)
}

func TestGormWebhookRepo_Get_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebookRepo(db, testLogger())

	got, err := repo.Get("does-not-exist")
	require.Error(t, err)
	// Get returns a non-nil zero-value webhook alongside the error.
	require.NotNil(t, got)
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
}

func TestGormWebhookRepo_GetByUser(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebookRepo(db, testLogger())

	wh := &models.Webhook{ID: "wh-1", Title: "hello", UserID: 42}
	require.NoError(t, repo.Insert(wh))

	got, err := repo.GetByUser("wh-1", 42)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "wh-1", got.ID)
}

func TestGormWebhookRepo_GetByUser_NotFoundWrongUser(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebookRepo(db, testLogger())

	wh := &models.Webhook{ID: "wh-1", Title: "hello", UserID: 42}
	require.NoError(t, repo.Insert(wh))

	got, err := repo.GetByUser("wh-1", 99)
	require.Error(t, err)
	require.NotNil(t, got)
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
}

func TestGormWebhookRepo_InsertRequest(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebookRepo(db, testLogger())

	wh := &models.Webhook{ID: "wh-1", Title: "hello"}
	require.NoError(t, repo.Insert(wh))

	wr := &models.WebhookRequest{ID: "req-1", WebhookID: "wh-1", Method: "GET"}
	require.NoError(t, repo.InsertRequest(wr))

	var got models.WebhookRequest
	require.NoError(t, db.First(&got, "id = ?", "req-1").Error)
	assert.Equal(t, "wh-1", got.WebhookID)
}

func TestGormWebhookRepo_GetAll(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebookRepo(db, testLogger())

	wh1 := &models.Webhook{ID: "wh-1", Title: "a", UserID: 0}
	wh2 := &models.Webhook{ID: "wh-2", Title: "b", UserID: 7}
	require.NoError(t, repo.Insert(wh1))
	require.NoError(t, repo.Insert(wh2))

	require.NoError(t, repo.InsertRequest(&models.WebhookRequest{ID: "req-1", WebhookID: "wh-1", Method: "GET"}))

	all, err := repo.GetAll()
	require.NoError(t, err)
	assert.Len(t, all, 2)

	// Preload("Requests") should have populated the associated requests.
	var found bool
	for _, w := range all {
		if w.ID == "wh-1" {
			found = true
			assert.Len(t, w.Requests, 1)
		}
	}
	assert.True(t, found)
}

func TestGormWebhookRepo_GetAllByUser(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebookRepo(db, testLogger())

	now := time.Now().UTC()
	wh1 := &models.Webhook{ID: "wh-1", Title: "a", UserID: 5, CreatedAt: now.Add(-time.Hour)}
	wh2 := &models.Webhook{ID: "wh-2", Title: "b", UserID: 5, CreatedAt: now}
	wh3 := &models.Webhook{ID: "wh-3", Title: "c", UserID: 9, CreatedAt: now}
	require.NoError(t, repo.Insert(wh1))
	require.NoError(t, repo.Insert(wh2))
	require.NoError(t, repo.Insert(wh3))

	webhooks, err := repo.GetAllByUser(5)
	require.NoError(t, err)

	// GetAllByUser orders by created_at DESC, so the more recently created
	// webhook (wh2, inserted second) should come first.
	require.Len(t, webhooks, 2)
	assert.Equal(t, "wh-2", webhooks[0].ID)
	assert.Equal(t, "wh-1", webhooks[1].ID)
}

// The request cap applies per webhook: one busy webhook must not crowd out
// another's requests, and each keeps its newest ones.
func TestGormWebhookRepo_GetAllByUser_CapsRequestsPerWebhook(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebookRepo(db, testLogger())

	require.NoError(t, repo.Insert(&models.Webhook{ID: "busy", UserID: 5}))
	require.NoError(t, repo.Insert(&models.Webhook{ID: "quiet", UserID: 5}))

	base := time.Now().UTC()
	busy := make([]models.WebhookRequest, requestsPerWebhookLimit+5)
	for i := range busy {
		busy[i] = models.WebhookRequest{
			ID:         fmt.Sprintf("busy-%05d", i),
			WebhookID:  "busy",
			ReceivedAt: base.Add(time.Duration(i) * time.Second),
		}
	}
	require.NoError(t, db.CreateInBatches(busy, 200).Error)
	require.NoError(t, repo.InsertRequest(&models.WebhookRequest{ID: "quiet-1", WebhookID: "quiet", ReceivedAt: base.Add(-time.Hour)}))

	webhooks, err := repo.GetAllByUser(5)
	require.NoError(t, err)
	byID := map[string]models.Webhook{}
	for _, wh := range webhooks {
		byID[wh.ID] = wh
	}

	require.Len(t, byID["busy"].Requests, requestsPerWebhookLimit)
	assert.Equal(t, busy[len(busy)-1].ID, byID["busy"].Requests[0].ID, "newest first")
	require.Len(t, byID["quiet"].Requests, 1)
}

func TestGormWebhookRepo_GetAllByUser_NoResults(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebookRepo(db, testLogger())

	webhooks, err := repo.GetAllByUser(123)
	require.NoError(t, err)
	assert.Empty(t, webhooks)
}

func TestGormWebhookRepo_Update(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebookRepo(db, testLogger())

	wh := &models.Webhook{ID: "wh-1", Title: "before"}
	require.NoError(t, repo.Insert(wh))

	wh.Title = "after"
	require.NoError(t, repo.Update(wh))

	got, err := repo.Get("wh-1")
	require.NoError(t, err)
	assert.Equal(t, "after", got.Title)
}

func TestGormWebhookRepo_Delete_Success(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebookRepo(db, testLogger())

	wh := &models.Webhook{ID: "wh-1", Title: "a", UserID: 5}
	require.NoError(t, repo.Insert(wh))
	require.NoError(t, repo.InsertRequest(&models.WebhookRequest{ID: "req-1", WebhookID: "wh-1", Method: "GET"}))
	require.NoError(t, repo.InsertRequest(&models.WebhookRequest{ID: "req-2", WebhookID: "wh-1", Method: "POST"}))

	require.NoError(t, repo.Delete("wh-1", 5))

	var whCount int64
	require.NoError(t, db.Model(&models.Webhook{}).Where("id = ?", "wh-1").Count(&whCount).Error)
	assert.Equal(t, int64(0), whCount)

	var reqCount int64
	require.NoError(t, db.Model(&models.WebhookRequest{}).Where("webhook_id = ?", "wh-1").Count(&reqCount).Error)
	assert.Equal(t, int64(0), reqCount)
}

func TestGormWebhookRepo_Delete_NotFoundOrUnauthorized(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebookRepo(db, testLogger())

	wh := &models.Webhook{ID: "wh-1", Title: "a", UserID: 5}
	require.NoError(t, repo.Insert(wh))
	require.NoError(t, repo.InsertRequest(&models.WebhookRequest{ID: "req-1", WebhookID: "wh-1", Method: "GET"}))

	// Wrong user id - unauthorized.
	err := repo.Delete("wh-1", 99)
	require.Error(t, err)
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))

	// Non-existent id.
	err = repo.Delete("does-not-exist", 5)
	require.Error(t, err)
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))

	// Nothing should have been deleted.
	var whCount int64
	require.NoError(t, db.Model(&models.Webhook{}).Where("id = ?", "wh-1").Count(&whCount).Error)
	assert.Equal(t, int64(1), whCount)

	var reqCount int64
	require.NoError(t, db.Model(&models.WebhookRequest{}).Where("webhook_id = ?", "wh-1").Count(&reqCount).Error)
	assert.Equal(t, int64(1), reqCount)
}

func TestGormWebhookRepo_GetWithRequests(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebookRepo(db, testLogger())

	wh := &models.Webhook{ID: "wh-1", Title: "a"}
	require.NoError(t, repo.Insert(wh))

	now := time.Now().UTC()
	require.NoError(t, repo.InsertRequest(&models.WebhookRequest{
		ID: "req-older", WebhookID: "wh-1", Method: "GET", ReceivedAt: now.Add(-time.Hour),
	}))
	require.NoError(t, repo.InsertRequest(&models.WebhookRequest{
		ID: "req-newer", WebhookID: "wh-1", Method: "POST", ReceivedAt: now,
	}))

	got, err := repo.GetWithRequests("wh-1")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Requests, 2)
	// Ordered newest first.
	assert.Equal(t, "req-newer", got.Requests[0].ID)
	assert.Equal(t, "req-older", got.Requests[1].ID)
}

func TestGormWebhookRepo_GetWithRequests_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebookRepo(db, testLogger())

	got, err := repo.GetWithRequests("does-not-exist")
	require.Error(t, err)
	require.NotNil(t, got)
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
}

// TestGormWebhookRepo_CleanPublic verifies that CleanPublic deletes public
// (user_id = 0) webhooks older than the given duration, along with their
// requests, while leaving newer public webhooks and any owned webhook alone.
func TestGormWebhookRepo_CleanPublic(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebookRepo(db, testLogger())

	now := time.Now().UTC()

	// "old" webhook: created well before the cutoff (now - 1h) -> should be deleted.
	oldPublic := &models.Webhook{ID: "old-public", Title: "old", UserID: 0}
	require.NoError(t, db.Create(oldPublic).Error)
	require.NoError(t, db.Model(&models.Webhook{}).Where("id = ?", "old-public").
		Update("created_at", now.Add(-2*time.Hour)).Error)

	// "new" webhook: created after the cutoff (now - 1h) -> should be kept.
	newPublic := &models.Webhook{ID: "new-public", Title: "new", UserID: 0}
	require.NoError(t, db.Create(newPublic).Error)
	require.NoError(t, db.Model(&models.Webhook{}).Where("id = ?", "new-public").
		Update("created_at", now).Error)

	// A non-public (owned) webhook older than the cutoff should never be
	// touched, since user_id != 0.
	owned := &models.Webhook{ID: "owned", Title: "owned", UserID: 5}
	require.NoError(t, db.Create(owned).Error)
	require.NoError(t, db.Model(&models.Webhook{}).Where("id = ?", "owned").
		Update("created_at", now.Add(-2*time.Hour)).Error)

	require.NoError(t, repo.InsertRequest(&models.WebhookRequest{ID: "req-old", WebhookID: "old-public", Method: "GET"}))
	require.NoError(t, repo.InsertRequest(&models.WebhookRequest{ID: "req-new", WebhookID: "new-public", Method: "GET"}))

	// Threshold duration of 1 hour -> beforeDate = now - 1h.
	deleted, err := repo.CleanPublic(time.Hour)
	require.NoError(t, err)
	assert.Equal(t, []string{"old-public"}, deleted)

	var remaining []models.Webhook
	require.NoError(t, db.Find(&remaining).Error)
	ids := make(map[string]bool)
	for _, w := range remaining {
		ids[w.ID] = true
	}
	assert.False(t, ids["old-public"], "old public webhook should have been deleted")
	assert.True(t, ids["new-public"], "new public webhook should be kept")
	assert.True(t, ids["owned"], "owned webhook should never be touched regardless of age")

	var reqOldCount int64
	require.NoError(t, db.Model(&models.WebhookRequest{}).Where("id = ?", "req-old").Count(&reqOldCount).Error)
	assert.Equal(t, int64(0), reqOldCount, "request for the deleted public webhook should also be deleted")

	var reqNewCount int64
	require.NoError(t, db.Model(&models.WebhookRequest{}).Where("id = ?", "req-new").Count(&reqNewCount).Error)
	assert.Equal(t, int64(1), reqNewCount, "request for the kept public webhook should remain")
}

func TestGormWebhookRepo_GetRequestsAfter(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebookRepo(db, testLogger())

	base := time.Now().UTC().Truncate(time.Microsecond)
	reqs := []*models.WebhookRequest{
		{ID: "b", WebhookID: "wh", ReceivedAt: base},
		{ID: "a", WebhookID: "wh", ReceivedAt: base.Add(time.Second)},
		// Same timestamp as "a": ordered after it by ID.
		{ID: "c", WebhookID: "wh", ReceivedAt: base.Add(time.Second)},
		{ID: "other", WebhookID: "other-wh", ReceivedAt: base.Add(time.Hour)},
	}
	for _, wr := range reqs {
		require.NoError(t, repo.InsertRequest(wr))
	}
	idsOf := func(rs []models.WebhookRequest) []string {
		var ids []string
		for _, r := range rs {
			ids = append(ids, r.ID)
		}
		return ids
	}

	all, err := repo.GetRequestsAfter("wh", models.RequestCursor{})
	require.NoError(t, err)
	assert.Equal(t, []string{"b", "a", "c"}, idsOf(all), "zero cursor returns everything, oldest first")

	after, err := repo.GetRequestsAfter("wh", models.CursorAt(*reqs[1]))
	require.NoError(t, err)
	assert.Equal(t, []string{"c"}, idsOf(after))

	none, err := repo.GetRequestsAfter("wh", models.CursorAt(*reqs[2]))
	require.NoError(t, err)
	assert.Empty(t, none)
}

func TestGormWebhookRepo_CleanPublic_NoMatches(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormWebookRepo(db, testLogger())

	// No public webhooks exist at all - should be a no-op, no error.
	deleted, err := repo.CleanPublic(24 * time.Hour)
	require.NoError(t, err)
	assert.Empty(t, deleted)
}
