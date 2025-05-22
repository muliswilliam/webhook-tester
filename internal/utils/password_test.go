package utils_test

import (
	"testing"

	"webhook-tester/internal/utils"

	"github.com/stretchr/testify/assert"
)

func TestPasswordValidator(t *testing.T) {
	validator := utils.NewPasswordValidator()
	assert.NotNil(t, validator)
}

func TestPasswordHasher(t *testing.T) {
	hasher := utils.NewPasswordHasher()
	assert.NotNil(t, hasher)
}

func TestPasswordHasher_HashPassword(t *testing.T) {
	hasher := utils.NewPasswordHasher()
	hash, err := hasher.HashPassword("password")
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)
}

func TestPasswordHasher_CheckPasswordHash(t *testing.T) {
	hasher := utils.NewPasswordHasher()
	hash, err := hasher.HashPassword("password")
	assert.NoError(t, err)
	assert.True(t, hasher.CheckPasswordHash("password", hash))
	assert.False(t, hasher.CheckPasswordHash("wrong-password", hash))
}

func TestPasswordValidator_Validate(t *testing.T) {
	validator := utils.NewPasswordValidator()
	err := validator.Validate("password", utils.PasswordRules{
		MinLength:        8,
		RequireUppercase: true,
		RequireLowercase: true,
		RequireNumber:    true,
	})
	assert.Error(t, err)
}

func TestPasswordValidator_Validate_Success(t *testing.T) {
	validator := utils.NewPasswordValidator()
	err := validator.Validate("Password123", utils.PasswordRules{
		MinLength:        8,
		RequireUppercase: true,
		RequireLowercase: true,
		RequireNumber:    true,
	})
	assert.NoError(t, err)
}

func TestPasswordValidator_Validate_MinLength(t *testing.T) {
	validator := utils.NewPasswordValidator()
	err := validator.Validate("pass", utils.PasswordRules{
		MinLength:        8,
		RequireUppercase: true,
		RequireLowercase: true,
		RequireNumber:    true,
	})
	assert.Error(t, err)
	assert.Equal(t, "password must be at least 8 characters", err.Error())
}

func TestPasswordValidator_Validate_RequireUppercase(t *testing.T) {
	validator := utils.NewPasswordValidator()
	err := validator.Validate("password", utils.PasswordRules{
		MinLength:        8,
		RequireUppercase: true,
		RequireLowercase: true,
		RequireNumber:    true,
	})
	assert.Error(t, err)
	assert.Equal(t, "password must contain at least one uppercase letter", err.Error())
}

func TestPasswordValidator_Validate_RequireLowercase(t *testing.T) {
	validator := utils.NewPasswordValidator()
	err := validator.Validate("PASSWORD", utils.PasswordRules{
		MinLength:        8,
		RequireUppercase: true,
		RequireLowercase: true,
		RequireNumber:    true,
	})
	assert.Error(t, err)
	assert.Equal(t, "password must contain at least one lowercase letter", err.Error())
}

func TestPasswordValidator_Validate_RequireNumber(t *testing.T) {
	validator := utils.NewPasswordValidator()
	err := validator.Validate("password", utils.PasswordRules{
		MinLength:        8,
		RequireUppercase: false,
		RequireLowercase: false,
		RequireNumber:    true,
	})
	assert.Error(t, err)
	assert.Equal(t, "password must contain at least one number", err.Error())
}
