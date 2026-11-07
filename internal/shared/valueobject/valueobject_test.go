package valueobject

import (
	"strings"
	"testing"

	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
)

func TestNewSlug(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    Slug
		wantErr bool
	}{
		{"simple", "payments", "payments", false},
		{"with hyphen", "payment-service", "payment-service", false},
		{"with digits", "service-2", "service-2", false},
		{"minimum length", "ab", "ab", false},
		{"maximum length", strings.Repeat("a", 63), Slug(strings.Repeat("a", 63)), false},
		{"trims whitespace", "  payments  ", "payments", false},
		{"empty", "", "", true},
		{"only whitespace", "   ", "", true},
		{"too short", "a", "", true},
		{"too long", strings.Repeat("a", 64), "", true},
		{"uppercase", "Payments", "", true},
		{"spaces inside", "my service", "", true},
		{"special characters", "My Service!", "", true},
		{"underscore", "my_service", "", true},
		{"leading hyphen", "-payments", "", true},
		{"trailing hyphen", "payments-", "", true},
		{"double hyphen", "pay--ments", "", true},
		{"unicode", "paymënts", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewSlug(tt.raw)

			if tt.wantErr {
				assertValidationError(t, err, "slug")
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
			if got.String() != string(tt.want) {
				t.Fatalf("expected String() %q, got %q", tt.want, got.String())
			}
		})
	}
}

func TestNewEmail(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    Email
		wantErr bool
	}{
		{"simple", "user@example.com", "user@example.com", false},
		{"lowercases", "User@Example.COM", "user@example.com", false},
		{"trims whitespace", "  user@example.com  ", "user@example.com", false},
		{"plus addressing", "user+tag@example.com", "user+tag@example.com", false},
		{"subdomain", "user@mail.example.co.uk", "user@mail.example.co.uk", false},
		{"empty", "", "", true},
		{"only whitespace", "   ", "", true},
		{"missing at sign", "user.example.com", "", true},
		{"missing domain", "user@", "", true},
		{"missing local part", "@example.com", "", true},
		{"display name form", "John <john@example.com>", "", true},
		{"contains space", "us er@example.com", "", true},
		{"too long", strings.Repeat("a", 250) + "@example.com", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewEmail(tt.raw)

			if tt.wantErr {
				assertValidationError(t, err, "email")
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
			if got.String() != string(tt.want) {
				t.Fatalf("expected String() %q, got %q", tt.want, got.String())
			}
		})
	}
}

func assertValidationError(t *testing.T, err error, field string) {
	t.Helper()

	appErr, ok := apperror.From(err)
	if !ok {
		t.Fatalf("expected an apperror, got %v", err)
	}
	if appErr.Code != apperror.CodeValidationFailed {
		t.Fatalf("expected %s, got %s", apperror.CodeValidationFailed, appErr.Code)
	}
	if appErr.Fields[field] == "" {
		t.Fatalf("expected a field error for %q, got %v", field, appErr.Fields)
	}
}
