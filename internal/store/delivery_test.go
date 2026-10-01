package store

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	webhookdb "webhook-tester/internal/db"
	"webhook-tester/internal/models"
)

// seedRequest stores a webhook (if new) and one captured request on it.
func seedRequest(t *testing.T, db *gorm.DB, webhookID, requestID string) {
	t.Helper()
	require.NoError(t, db.FirstOrCreate(&models.Webhook{ID: webhookID}, "id = ?", webhookID).Error)
	require.NoError(t, db.Create(&models.WebhookRequest{ID: requestID, WebhookID: webhookID, Method: "POST"}).Error)
}

func deliveryIDs(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	var ids []string
	require.NoError(t, db.Model(&models.Delivery{}).Order("id").Pluck("id", &ids).Error)
	return ids
}

func TestGormDeliveryRepo_InsertAndListByRequest(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormDeliveryRepo(db, testLogger())
	seedRequest(t, db, "wh-1", "req-1")

	status := 500
	d := &models.Delivery{
		ID:                    "del-1",
		RequestID:             "req-1",
		WebhookID:             "wh-1",
		Trigger:               models.DeliveryTriggerAuto,
		TargetURL:             "https://example.com/hook/orders/42?a=1",
		StatusCode:            &status,
		DurationMs:            123,
		ResponseHeaders:       datatypes.JSONMap{"Content-Type": "text/plain"},
		ResponseBody:          "boom",
		ResponseBodyTruncated: true,
		StartedAt:             time.Now().UTC(),
	}
	require.NoError(t, repo.Insert(d))

	list, err := repo.ListByRequest("req-1")
	require.NoError(t, err)
	require.Len(t, list, 1)
	got := list[0]
	assert.Equal(t, "wh-1", got.WebhookID)
	assert.Equal(t, models.DeliveryTriggerAuto, got.Trigger)
	assert.Equal(t, "https://example.com/hook/orders/42?a=1", got.TargetURL)
	require.NotNil(t, got.StatusCode)
	assert.Equal(t, 500, *got.StatusCode)
	assert.Nil(t, got.Error)
	assert.Equal(t, int64(123), got.DurationMs)
	assert.Equal(t, "text/plain", got.ResponseHeaders["Content-Type"])
	assert.Equal(t, "boom", got.ResponseBody)
	assert.True(t, got.ResponseBodyTruncated)
}

func TestGormDeliveryRepo_Insert_GeneratesID(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormDeliveryRepo(db, testLogger())
	seedRequest(t, db, "wh-1", "req-1")

	msg := "connection refused"
	d := &models.Delivery{RequestID: "req-1", WebhookID: "wh-1", Trigger: models.DeliveryTriggerReplay, Error: &msg}
	require.NoError(t, repo.Insert(d))
	assert.NotEmpty(t, d.ID)

	list, err := repo.ListByRequest("req-1")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Nil(t, list[0].StatusCode)
	require.NotNil(t, list[0].Error)
	assert.Equal(t, "connection refused", *list[0].Error)
}

func TestGormDeliveryRepo_ListByRequest_NewestFirst(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormDeliveryRepo(db, testLogger())
	seedRequest(t, db, "wh-1", "req-1")
	seedRequest(t, db, "wh-1", "req-2")

	now := time.Now().UTC()
	require.NoError(t, repo.Insert(&models.Delivery{ID: "old", RequestID: "req-1", WebhookID: "wh-1", Trigger: models.DeliveryTriggerAuto, StartedAt: now.Add(-time.Hour)}))
	require.NoError(t, repo.Insert(&models.Delivery{ID: "new", RequestID: "req-1", WebhookID: "wh-1", Trigger: models.DeliveryTriggerReplay, StartedAt: now}))
	require.NoError(t, repo.Insert(&models.Delivery{ID: "mid", RequestID: "req-1", WebhookID: "wh-1", Trigger: models.DeliveryTriggerReplay, StartedAt: now.Add(-time.Minute)}))
	require.NoError(t, repo.Insert(&models.Delivery{ID: "other", RequestID: "req-2", WebhookID: "wh-1", Trigger: models.DeliveryTriggerAuto, StartedAt: now}))

	list, err := repo.ListByRequest("req-1")
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Equal(t, "new", list[0].ID)
	assert.Equal(t, "mid", list[1].ID)
	assert.Equal(t, "old", list[2].ID)
}

func TestGormDeliveryRepo_ListByRequest_NoResults(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormDeliveryRepo(db, testLogger())

	list, err := repo.ListByRequest("does-not-exist")
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestGormDeliveryRepo_ErrorsAfterConnectionClosed(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormDeliveryRepo(db, testLogger())
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	assert.Error(t, repo.Insert(&models.Delivery{ID: "d1", RequestID: "req-1", WebhookID: "wh-1"}))
	_, err = repo.ListByRequest("req-1")
	assert.Error(t, err)
}

// The delete paths for captured requests and webhooks also remove the
// deliveries, so no orphaned deliveries are left behind.

func TestGormWebhookRequestRepo_DeleteByID_DeletesDeliveries(t *testing.T) {
	db := newTestDB(t)
	deliveries := NewGormDeliveryRepo(db, testLogger())
	repo := NewGormWebhookRequestRepo(db, testLogger())
	seedRequest(t, db, "wh-1", "req-1")
	seedRequest(t, db, "wh-1", "req-2")
	require.NoError(t, deliveries.Insert(&models.Delivery{ID: "d1", RequestID: "req-1", WebhookID: "wh-1", Trigger: models.DeliveryTriggerAuto}))
	require.NoError(t, deliveries.Insert(&models.Delivery{ID: "d2", RequestID: "req-2", WebhookID: "wh-1", Trigger: models.DeliveryTriggerAuto}))

	require.NoError(t, repo.DeleteByID("req-1"))

	assert.Equal(t, []string{"d2"}, deliveryIDs(t, db))
}

func TestGormWebhookRequestRepo_DeleteByWebhook_DeletesDeliveries(t *testing.T) {
	db := newTestDB(t)
	deliveries := NewGormDeliveryRepo(db, testLogger())
	repo := NewGormWebhookRequestRepo(db, testLogger())
	seedRequest(t, db, "wh-1", "req-1")
	seedRequest(t, db, "wh-2", "req-2")
	require.NoError(t, deliveries.Insert(&models.Delivery{ID: "d1", RequestID: "req-1", WebhookID: "wh-1", Trigger: models.DeliveryTriggerAuto}))
	require.NoError(t, deliveries.Insert(&models.Delivery{ID: "d2", RequestID: "req-2", WebhookID: "wh-2", Trigger: models.DeliveryTriggerAuto}))

	require.NoError(t, repo.DeleteByWebhook("wh-1"))

	assert.Equal(t, []string{"d2"}, deliveryIDs(t, db))
}

func TestGormWebhookRepo_Delete_DeletesDeliveries(t *testing.T) {
	db := newTestDB(t)
	deliveries := NewGormDeliveryRepo(db, testLogger())
	repo := NewGormWebookRepo(db, testLogger())
	require.NoError(t, repo.Insert(&models.Webhook{ID: "wh-1", Title: "a", UserID: 5}))
	require.NoError(t, repo.Insert(&models.Webhook{ID: "wh-2", Title: "b", UserID: 5}))
	seedRequest(t, db, "wh-1", "req-1")
	seedRequest(t, db, "wh-2", "req-2")
	require.NoError(t, deliveries.Insert(&models.Delivery{ID: "d1", RequestID: "req-1", WebhookID: "wh-1", Trigger: models.DeliveryTriggerAuto}))
	require.NoError(t, deliveries.Insert(&models.Delivery{ID: "d2", RequestID: "req-2", WebhookID: "wh-2", Trigger: models.DeliveryTriggerAuto}))

	require.NoError(t, repo.Delete("wh-1", 5))

	assert.Equal(t, []string{"d2"}, deliveryIDs(t, db))
}

func TestGormWebhookRepo_Delete_Unauthorized_KeepsDeliveries(t *testing.T) {
	db := newTestDB(t)
	deliveries := NewGormDeliveryRepo(db, testLogger())
	repo := NewGormWebookRepo(db, testLogger())
	require.NoError(t, repo.Insert(&models.Webhook{ID: "wh-1", Title: "a", UserID: 5}))
	seedRequest(t, db, "wh-1", "req-1")
	require.NoError(t, deliveries.Insert(&models.Delivery{ID: "d1", RequestID: "req-1", WebhookID: "wh-1", Trigger: models.DeliveryTriggerAuto}))

	require.Error(t, repo.Delete("wh-1", 99))

	assert.Equal(t, []string{"d1"}, deliveryIDs(t, db))
}

// With foreign keys enforced, as Postgres does in production, every delete
// path must remove deliveries before the requests they reference.
func TestDeletePaths_RespectDeliveryForeignKey(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:?_foreign_keys=on"), &gorm.Config{})
	require.NoError(t, err)
	webhookdb.AutoMigrate(db)
	deliveries := NewGormDeliveryRepo(db, testLogger())
	webhooks := NewGormWebookRepo(db, testLogger())
	requests := NewGormWebhookRequestRepo(db, testLogger())

	seed := func(webhookID, requestID string, userID int, createdAt time.Time) {
		require.NoError(t, db.FirstOrCreate(&models.Webhook{ID: webhookID, UserID: userID, CreatedAt: createdAt}, "id = ?", webhookID).Error)
		require.NoError(t, db.Create(&models.WebhookRequest{ID: requestID, WebhookID: webhookID}).Error)
		require.NoError(t, deliveries.Insert(&models.Delivery{RequestID: requestID, WebhookID: webhookID, Trigger: models.DeliveryTriggerAuto}))
	}
	now := time.Now().UTC()
	seed("wh-1", "req-1", 5, now)
	seed("wh-1", "req-2", 5, now)
	seed("wh-2", "req-3", 5, now)
	seed("pub", "req-4", 0, now.Add(-2*time.Hour))

	// The constraint is in force: a delivery can't reference a missing request.
	require.Error(t, deliveries.Insert(&models.Delivery{RequestID: "missing", WebhookID: "wh-1", Trigger: models.DeliveryTriggerAuto}))

	require.NoError(t, requests.DeleteByID("req-1"))
	require.NoError(t, requests.DeleteByWebhook("wh-1"))
	require.NoError(t, webhooks.Delete("wh-2", 5))
	_, err = webhooks.CleanPublic(time.Hour)
	require.NoError(t, err)

	assert.Empty(t, deliveryIDs(t, db))
}

func TestGormWebhookRepo_CleanPublic_DeletesDeliveries(t *testing.T) {
	db := newTestDB(t)
	deliveries := NewGormDeliveryRepo(db, testLogger())
	repo := NewGormWebookRepo(db, testLogger())
	old := time.Now().UTC().Add(-2 * time.Hour)
	require.NoError(t, db.Create(&models.Webhook{ID: "old-public", Title: "old", CreatedAt: old}).Error)
	require.NoError(t, db.Create(&models.Webhook{ID: "new-public", Title: "new"}).Error)
	seedRequest(t, db, "old-public", "req-old")
	seedRequest(t, db, "new-public", "req-new")
	require.NoError(t, deliveries.Insert(&models.Delivery{ID: "d-old", RequestID: "req-old", WebhookID: "old-public", Trigger: models.DeliveryTriggerAuto}))
	require.NoError(t, deliveries.Insert(&models.Delivery{ID: "d-new", RequestID: "req-new", WebhookID: "new-public", Trigger: models.DeliveryTriggerAuto}))

	ids, err := repo.CleanPublic(time.Hour)
	require.NoError(t, err)
	assert.Equal(t, []string{"old-public"}, ids)

	assert.Equal(t, []string{"d-new"}, deliveryIDs(t, db))
}

// Captured requests loaded for display come with their deliveries, newest
// first.

func deliveryIDsOf(wr models.WebhookRequest) []string {
	ids := []string{}
	for _, d := range wr.Deliveries {
		ids = append(ids, d.ID)
	}
	return ids
}

func deliveryIDsByRequest(list []models.WebhookRequest) map[string][]string {
	out := map[string][]string{}
	for _, wr := range list {
		out[wr.ID] = deliveryIDsOf(wr)
	}
	return out
}

func TestRequestsLoadWithTheirDeliveries(t *testing.T) {
	db := newTestDB(t)
	webhooks := NewGormWebookRepo(db, testLogger())
	requests := NewGormWebhookRequestRepo(db, testLogger())
	deliveries := NewGormDeliveryRepo(db, testLogger())
	require.NoError(t, webhooks.Insert(&models.Webhook{ID: "wh-1", Title: "a", UserID: 5}))
	require.NoError(t, webhooks.Insert(&models.Webhook{ID: "wh-2", Title: "b", UserID: 5}))
	seedRequest(t, db, "wh-1", "req-1")
	seedRequest(t, db, "wh-2", "req-2")
	seedRequest(t, db, "wh-2", "req-none")
	now := time.Now().UTC()
	for _, d := range []models.Delivery{
		{ID: "d-old", RequestID: "req-1", WebhookID: "wh-1", Trigger: models.DeliveryTriggerAuto, StartedAt: now.Add(-time.Minute)},
		{ID: "d-new", RequestID: "req-1", WebhookID: "wh-1", Trigger: models.DeliveryTriggerReplay, StartedAt: now},
		{ID: "d-other", RequestID: "req-2", WebhookID: "wh-2", Trigger: models.DeliveryTriggerAuto, StartedAt: now},
	} {
		require.NoError(t, deliveries.Insert(&d))
	}

	t.Run("GetWithRequests", func(t *testing.T) {
		wh, err := webhooks.GetWithRequests("wh-1")
		require.NoError(t, err)
		assert.Equal(t, map[string][]string{"req-1": {"d-new", "d-old"}}, deliveryIDsByRequest(wh.Requests))
	})
	t.Run("GetAllByUser", func(t *testing.T) {
		list, err := webhooks.GetAllByUser(5)
		require.NoError(t, err)
		got := map[string][]string{}
		for _, wh := range list {
			for id, ds := range deliveryIDsByRequest(wh.Requests) {
				got[id] = ds
			}
		}
		assert.Equal(t, map[string][]string{"req-1": {"d-new", "d-old"}, "req-2": {"d-other"}, "req-none": {}}, got)
	})
	t.Run("GetRequestsAfter", func(t *testing.T) {
		list, err := webhooks.GetRequestsAfter("wh-2", models.RequestCursor{})
		require.NoError(t, err)
		assert.Equal(t, map[string][]string{"req-2": {"d-other"}, "req-none": {}}, deliveryIDsByRequest(list))
	})
	t.Run("GetByID", func(t *testing.T) {
		wr, err := requests.GetByID("req-1")
		require.NoError(t, err)
		assert.Equal(t, []string{"d-new", "d-old"}, deliveryIDsOf(*wr))
	})
}

// Deliveries are loaded in chunks of request IDs, so a webhook with more
// requests than fit in one chunk still gets all of them.
func TestGormWebhookRepo_GetWithRequests_LoadsDeliveriesAcrossChunks(t *testing.T) {
	db := newTestDB(t)
	webhooks := NewGormWebookRepo(db, testLogger())
	require.NoError(t, webhooks.Insert(&models.Webhook{ID: "wh-1", Title: "a"}))

	n := deliveriesQueryChunk + 1
	reqs := make([]models.WebhookRequest, n)
	dels := make([]models.Delivery, n)
	for i := range reqs {
		reqs[i] = models.WebhookRequest{ID: fmt.Sprintf("req-%04d", i), WebhookID: "wh-1"}
		dels[i] = models.Delivery{ID: fmt.Sprintf("del-%04d", i), RequestID: reqs[i].ID, WebhookID: "wh-1", Trigger: models.DeliveryTriggerAuto}
	}
	require.NoError(t, db.CreateInBatches(reqs, 100).Error)
	require.NoError(t, db.CreateInBatches(dels, 100).Error)

	wh, err := webhooks.GetWithRequests("wh-1")
	require.NoError(t, err)
	require.Len(t, wh.Requests, n)
	for _, wr := range wh.Requests {
		require.Len(t, wr.Deliveries, 1, "request %s", wr.ID)
		assert.Equal(t, wr.ID, wr.Deliveries[0].RequestID)
	}
}

func TestGormWebhookRepo_DeliveriesLoadFailureFailsRequestLoads(t *testing.T) {
	db := newTestDB(t)
	webhooks := NewGormWebookRepo(db, testLogger())
	require.NoError(t, webhooks.Insert(&models.Webhook{ID: "wh-1", Title: "a", UserID: 5}))
	seedRequest(t, db, "wh-1", "req-1")
	require.NoError(t, db.Migrator().DropTable(&models.Delivery{}))

	_, err := webhooks.GetWithRequests("wh-1")
	assert.Error(t, err)
	_, err = webhooks.GetAllByUser(5)
	assert.Error(t, err)
	_, err = webhooks.GetRequestsAfter("wh-1", models.RequestCursor{})
	assert.Error(t, err)
}
