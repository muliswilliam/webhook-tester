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

func TestAbbreviate(t *testing.T) {
	for _, tc := range []struct {
		s    string
		max  int
		want string
	}{
		{"", 10, ""},
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"eleven char", 10, "eleven ..."},
		// Never cut inside a character.
		{"ééééé", 8, "éé..."},
		{"ééééé", 9, "ééé..."},
		{"ééééé", 10, "ééééé"},
	} {
		got := Abbreviate(tc.s, tc.max)
		assert.Equal(t, tc.want, got, "%q, %d", tc.s, tc.max)
		assert.LessOrEqual(t, len(got), tc.max)
	}
}
