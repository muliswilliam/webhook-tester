package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSecureToken(t *testing.T) {
	token, err := GenerateSecureToken(16)
	require.NoError(t, err)
	assert.Len(t, token, 32)

	other, err := GenerateSecureToken(16)
	require.NoError(t, err)
	assert.NotEqual(t, token, other)
}

func TestGenerateSecureTokenZeroLength(t *testing.T) {
	token, err := GenerateSecureToken(0)
	require.NoError(t, err)
	assert.Equal(t, "", token)
}
