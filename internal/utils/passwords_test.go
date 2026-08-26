package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("s3cr3tPass")
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, "s3cr3tPass", hash)

	assert.True(t, CheckPasswordHash("s3cr3tPass", hash))
	assert.False(t, CheckPasswordHash("wrongPass", hash))
}

func TestValidatePassword(t *testing.T) {
	defaultRules := PasswordRules{
		MinLength:        8,
		RequireUppercase: true,
		RequireLowercase: true,
		RequireNumber:    true,
	}

	tests := []struct {
		name    string
		pw      string
		rules   PasswordRules
		wantErr string
	}{
		{
			name:    "too short",
			pw:      "Ab1",
			rules:   defaultRules,
			wantErr: "at least 8 characters",
		},
		{
			name:    "missing uppercase",
			pw:      "lowercase1",
			rules:   defaultRules,
			wantErr: "uppercase letter",
		},
		{
			name:    "missing lowercase",
			pw:      "UPPERCASE1",
			rules:   defaultRules,
			wantErr: "lowercase letter",
		},
		{
			name:    "missing number",
			pw:      "NoNumberHere",
			rules:   defaultRules,
			wantErr: "one number",
		},
		{
			name:    "valid password",
			pw:      "ValidPass1",
			rules:   defaultRules,
			wantErr: "",
		},
		{
			name:    "no rules enforced besides length",
			pw:      "short",
			rules:   PasswordRules{MinLength: 3},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.pw, tt.rules)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}
