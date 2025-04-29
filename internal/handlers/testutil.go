package handlers

import (
	"testing"
)

func NewTestHandler(t *testing.T) *Handler {
	//db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	//if err != nil {
	//	t.Fatalf("failed to connect test db: %v", err)
	//}
	//
	//err = db.AutoMigrate(&models.Webhook{}, &models.WebhookRequest{}, &models.User{})
	//if err != nil {
	//	t.Fatalf("migration failed: %v", err)
	//}
	//
	//store := gormstore.New(db, []byte("test-secret"))
	//go store.PeriodicCleanup(24*time.Hour, make(chan struct{}))

	return &Handler{
		//DB:           db,
		//SessionStore: store,
	}
}
