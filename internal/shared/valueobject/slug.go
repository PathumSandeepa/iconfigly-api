package valueobject

import (
	"regexp"
	"strings"

	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
)

const (
	slugMinLength = 2
	slugMaxLength = 63
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Slug string

func NewSlug(raw string) (Slug, error) {
	value := strings.TrimSpace(raw)

	if len(value) < slugMinLength || len(value) > slugMaxLength {
		return "", invalidSlug("must be between 2 and 63 characters")
	}

	if !slugPattern.MatchString(value) {
		return "", invalidSlug("must contain only lowercase letters, digits and single hyphens between them")
	}

	return Slug(value), nil
}

func (s Slug) String() string {
	return string(s)
}

func invalidSlug(reason string) error {
	return apperror.Validation("invalid slug", map[string]string{"slug": reason})
}
