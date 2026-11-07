package valueobject

import (
	"net/mail"
	"strings"

	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
)

const emailMaxLength = 254

type Email string

func NewEmail(raw string) (Email, error) {
	value := strings.ToLower(strings.TrimSpace(raw))

	if value == "" {
		return "", invalidEmail("is required")
	}

	if len(value) > emailMaxLength {
		return "", invalidEmail("must be at most 254 characters")
	}

	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Address != value {
		return "", invalidEmail("must be a valid email address")
	}

	return Email(value), nil
}

func (e Email) String() string {
	return string(e)
}

func invalidEmail(reason string) error {
	return apperror.Validation("invalid email", map[string]string{"email": reason})
}
