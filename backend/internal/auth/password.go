package auth

import (
	"strings"
	"unicode/utf16"

	"golang.org/x/crypto/bcrypt"
)

var placeholders = map[string]bool{"123456": true, "change-me": true, "change-me-in-production": true, "change-me-to-a-long-random-secret": true, "password": true, "changeme": true}

func IsPlaceholder(value string) bool { return placeholders[strings.ToLower(strings.TrimSpace(value))] }
func StrongPassword(value string) bool {
	v := strings.TrimSpace(value)
	return len(utf16.Encode([]rune(v))) >= 12 && !IsPlaceholder(v)
}
func InitialPassword(configured, environment string) (string, bool) {
	value := strings.TrimSpace(configured)
	if value == "" {
		return "123456", true
	}
	if environment != "production" || value == "123456" {
		return value, true
	}
	return value, StrongPassword(value)
}
func VerifyPassword(password, hash, initial string) bool {
	if password == "" {
		return false
	}
	if hash != "" {
		return bcrypt.CompareHashAndPassword([]byte(hash), bcryptBytes(password)) == nil
	}
	return initial != "" && password == initial
}

// bcryptjs limita a entrada aos primeiros 72 bytes; x/crypto rejeita entradas maiores ao gerar.
func bcryptBytes(password string) []byte {
	b := []byte(password)
	if len(b) > 72 {
		return b[:72]
	}
	return b
}
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword(bcryptBytes(password), 10)
	return string(hash), err
}
