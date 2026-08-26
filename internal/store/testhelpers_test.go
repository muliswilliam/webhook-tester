package store

import (
	"io"
	"log"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	webhookdb "webhook-tester/internal/db"
)

// newTestDB spins up a fresh in-memory sqlite database with the production
// schema applied via the real AutoMigrate function.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	webhookdb.AutoMigrate(db)

	return db
}

// testLogger returns a logger that discards all output, keeping test output
// clean while still exercising the logging code paths in the repos.
func testLogger() *log.Logger {
	return log.New(io.Discard, "", 0)
}
