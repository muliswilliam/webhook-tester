package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"webhook-tester/internal/models"
)

// TestAutoMigrate verifies that AutoMigrate creates working tables for all
// three models, and that it is safe to call more than once (idempotent).
//
// Note: Connect() is intentionally NOT tested here - it requires a live
// Postgres connection and calls log.Fatal when required env vars are
// missing, which is untestable without a real database / process exit.
func TestAutoMigrate(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	AutoMigrate(gdb)

	require.True(t, gdb.Migrator().HasTable(&models.Webhook{}))
	require.True(t, gdb.Migrator().HasTable(&models.WebhookRequest{}))
	require.True(t, gdb.Migrator().HasTable(&models.User{}))

	// Webhook CRUD works against the migrated schema.
	wh := &models.Webhook{ID: "wh-1", Title: "hello"}
	require.NoError(t, gdb.Create(wh).Error)
	var gotWh models.Webhook
	require.NoError(t, gdb.First(&gotWh, "id = ?", "wh-1").Error)
	assert.Equal(t, "hello", gotWh.Title)

	// WebhookRequest CRUD works against the migrated schema.
	req := &models.WebhookRequest{ID: "req-1", WebhookID: "wh-1", Method: "GET"}
	require.NoError(t, gdb.Create(req).Error)
	var gotReq models.WebhookRequest
	require.NoError(t, gdb.First(&gotReq, "id = ?", "req-1").Error)
	assert.Equal(t, "wh-1", gotReq.WebhookID)

	// User CRUD works against the migrated schema.
	u := &models.User{Email: "jane@example.com"}
	require.NoError(t, gdb.Create(u).Error)
	var gotUser models.User
	require.NoError(t, gdb.First(&gotUser, "id = ?", u.ID).Error)
	assert.Equal(t, "jane@example.com", gotUser.Email)

	// Calling AutoMigrate a second time should be idempotent and not error
	// (it does not panic / log.Fatal on an already-migrated schema).
	AutoMigrate(gdb)
	require.True(t, gdb.Migrator().HasTable(&models.Webhook{}))
}
