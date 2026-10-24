package auth_test

import (
	"strings"
	"testing"

	"github.com/sklyar-vlad/HackAeza/backend/internal/auth"
)

func TestPasswordLengthValidation(t *testing.T) {
	for _, password := range []string{
		strings.Repeat("a", auth.MinPasswordBytes),
		strings.Repeat("a", auth.MaxPasswordBytes),
		strings.Repeat("я", auth.MinPasswordBytes/2),
		strings.Repeat("я", auth.MaxPasswordBytes/2),
	} {
		if err := auth.ValidatePassword(password); err != nil {
			t.Fatalf("valid byte length rejected: %v", err)
		}
	}
	for _, test := range []struct {
		password string
		message  string
	}{
		{"", "0 символов (0 байт)"},
		{strings.Repeat("a", 11), "11 символов (11 байт)"},
		{strings.Repeat("a", 73), "73 символов (73 байт)"},
		{strings.Repeat("я", 37), "37 символов (74 байт)"},
	} {
		err := auth.ValidatePassword(test.password)
		if err == nil || !strings.Contains(err.Error(), test.message) {
			t.Fatalf("missing clear length diagnostic: %v", err)
		}
		if test.password != "" && strings.Contains(err.Error(), test.password) {
			t.Fatal("error contains the password")
		}
	}
}
