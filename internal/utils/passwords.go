package utils

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"regexp"
)

type PasswordRules struct {
	MinLength        int
	RequireUppercase bool
	RequireLowercase bool
	RequireNumber    bool
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// PasswordError explains why a password breaks the PasswordRules. Its text
// is a complete sentence, shown to the user as is.
type PasswordError string

func (e PasswordError) Error() string { return string(e) }

func ValidatePassword(pw string, rules PasswordRules) error {
	if len(pw) < rules.MinLength {
		return PasswordError(fmt.Sprintf("Password must be at least %d characters", rules.MinLength))
	}

	if rules.RequireUppercase && !regexp.MustCompile("[A-Z]").MatchString(pw) {
		return PasswordError("Password must contain at least one uppercase letter")
	}

	if rules.RequireLowercase && !regexp.MustCompile("[a-z]").MatchString(pw) {
		return PasswordError("Password must contain at least one lowercase letter")
	}

	if rules.RequireNumber && !regexp.MustCompile("[0-9]").MatchString(pw) {
		return PasswordError("Password must contain at least one number")
	}

	return nil
}
