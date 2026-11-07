package apperror

import (
	"errors"
	"fmt"
	"testing"
)

func TestConstructorsSetExpectedCode(t *testing.T) {
	tests := []struct {
		name string
		err  *Error
		want Code
	}{
		{"not found", NotFound("project"), CodeNotFound},
		{"conflict", Conflict("slug already exists"), CodeConflict},
		{"validation", Validation("invalid input", nil), CodeValidationFailed},
		{"unauthorized", Unauthorized("login required"), CodeUnauthorized},
		{"forbidden", Forbidden("missing permission"), CodeForbidden},
		{"internal", Internal(errors.New("boom")), CodeInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.want {
				t.Fatalf("expected code %s, got %s", tt.want, tt.err.Code)
			}
			if tt.err.Title == "" {
				t.Fatal("expected a non-empty title")
			}
		})
	}
}

func TestNotFoundDetailMentionsResource(t *testing.T) {
	err := NotFound("project")

	if err.Detail != "project was not found" {
		t.Fatalf("unexpected detail: %s", err.Detail)
	}
}

func TestValidationCarriesFieldErrors(t *testing.T) {
	err := Validation("invalid input", map[string]string{"slug": "too short"})

	if err.Fields["slug"] != "too short" {
		t.Fatalf("expected field error to be preserved, got %v", err.Fields)
	}
}

func TestInternalHidesCauseFromDetail(t *testing.T) {
	cause := errors.New("connection refused")

	err := Internal(cause)

	if err.Detail == cause.Error() {
		t.Fatal("internal cause must not leak into the public detail")
	}
	if !errors.Is(err, cause) {
		t.Fatal("expected the cause to remain reachable through Unwrap")
	}
}

func TestErrorStringIncludesCodeAndDetail(t *testing.T) {
	err := Conflict("slug already exists")

	if got := err.Error(); got != "CONFLICT: slug already exists" {
		t.Fatalf("unexpected message: %s", got)
	}
}

func TestErrorStringIncludesCause(t *testing.T) {
	err := Internal(errors.New("boom"))

	if got := err.Error(); got != "INTERNAL: an unexpected error occurred: boom" {
		t.Fatalf("unexpected message: %s", got)
	}
}

func TestFromFindsWrappedError(t *testing.T) {
	wrapped := fmt.Errorf("create project: %w", NotFound("organization"))

	appErr, ok := From(wrapped)

	if !ok {
		t.Fatal("expected to find an apperror in the chain")
	}
	if appErr.Code != CodeNotFound {
		t.Fatalf("expected %s, got %s", CodeNotFound, appErr.Code)
	}
}

func TestFromIgnoresForeignErrors(t *testing.T) {
	if _, ok := From(errors.New("plain error")); ok {
		t.Fatal("expected plain errors not to match")
	}
	if _, ok := From(nil); ok {
		t.Fatal("expected nil not to match")
	}
}

func TestHasCode(t *testing.T) {
	wrapped := fmt.Errorf("outer: %w", Forbidden("denied"))

	if !HasCode(wrapped, CodeForbidden) {
		t.Fatal("expected wrapped forbidden error to match its code")
	}
	if HasCode(wrapped, CodeNotFound) {
		t.Fatal("expected a different code not to match")
	}
	if HasCode(errors.New("plain"), CodeInternal) {
		t.Fatal("expected plain errors not to match any code")
	}
}
