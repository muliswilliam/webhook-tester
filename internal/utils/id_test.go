package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateID(t *testing.T) {
	id := GenerateID()
	assert.NotEmpty(t, id)
	assert.Len(t, id, 21)

	other := GenerateID()
	assert.NotEqual(t, id, other)
}
