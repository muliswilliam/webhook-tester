package main

import (
	"io"
	"log"
	"testing"
	"time"

	"github.com/robfig/cron"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	webhookdb "webhook-tester/internal/db"
	"webhook-tester/internal/models"
	"webhook-tester/internal/service"
	"webhook-tester/internal/store"
)

func TestScheduleCleanup(t *testing.T) {
	c := cron.New()

	require.NotPanics(t, func() {
		scheduleCleanup(nil, log.New(io.Discard, "", 0), c)
	})

	require.Len(t, c.Entries(), 1)
}

func TestScheduleCleanup_DeletesExpiredGuestWebhooks(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	webhookdb.AutoMigrate(db)
	logger := log.New(io.Discard, "", 0)
	svc := service.NewWebhookService(store.NewGormWebookRepo(db, logger), store.NewGormDeliveryRepo(db, logger), "", service.ForwardPolicy{})

	now := time.Now().UTC()
	require.NoError(t, db.Create(&models.Webhook{ID: "expired", CreatedAt: now.Add(-guestWorkspaceTTL - time.Minute)}).Error)
	require.NoError(t, db.Create(&models.Webhook{ID: "fresh", CreatedAt: now.Add(-time.Hour)}).Error)
	require.NoError(t, db.Create(&models.Webhook{ID: "owned", UserID: 1, CreatedAt: now.Add(-30 * 24 * time.Hour)}).Error)

	c := cron.New()
	scheduleCleanup(svc, logger, c)
	c.Entries()[0].Job.Run()

	var ids []string
	require.NoError(t, db.Model(&models.Webhook{}).Order("id").Pluck("id", &ids).Error)
	require.Equal(t, []string{"fresh", "owned"}, ids)
}
