package utils

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateAPIKey(t *testing.T) {
	t.Run("length too short", func(t *testing.T) {
		key, err := GenerateAPIKey("user_", 8)
		require.Error(t, err)
		assert.Empty(t, key)
		assert.Contains(t, err.Error(), "at least 16 bytes")
	})

	t.Run("prefix with space", func(t *testing.T) {
		key, err := GenerateAPIKey("user key_", 16)
		require.Error(t, err)
		assert.Empty(t, key)
		assert.Contains(t, err.Error(), "must not contain spaces")
	})

	t.Run("success", func(t *testing.T) {
		key, err := GenerateAPIKey("user_", 16)
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(key, "user_"))
		suffix := strings.TrimPrefix(key, "user_")
		assert.Len(t, suffix, 32)
	})
}
