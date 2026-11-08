package organization

import (
	"strings"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/valueobject"
	"github.com/google/uuid"
)

type Organization struct {
	ID        uuid.UUID        `json:"id"`
	Slug      valueobject.Slug `json:"slug"`
	Name      string           `json:"name"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
}

func NewOrganization(slug valueobject.Slug, name string) (*Organization, error) {
	trimmedName := strings.TrimSpace(name)
	if len(trimmedName) < 2 || len(trimmedName) > 100 {
		return nil, apperror.Validation("invalid organization name", map[string]string{
			"name": "must be between 2 and 100 characters",
		})
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	return &Organization{
		ID:        uuid.Must(uuid.NewV7()),
		Slug:      slug,
		Name:      trimmedName,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}
