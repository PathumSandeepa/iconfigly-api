package project

import (
	"strings"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/valueobject"
	"github.com/google/uuid"
)

type Project struct {
	ID             uuid.UUID        `json:"id"`
	OrganizationID uuid.UUID        `json:"organization_id"`
	Slug           valueobject.Slug `json:"slug"`
	Name           string           `json:"name"`
	Description    string           `json:"description"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

func NewProject(
	orgID uuid.UUID,
	slug valueobject.Slug,
	name string,
	description string,
) (*Project, error) {
	trimmedName := strings.TrimSpace(name)
	if len(trimmedName) < 2 || len(trimmedName) > 100 {
		return nil, apperror.Validation("invalid project name", map[string]string{
			"name": "must be between 2 and 100 characters",
		})
	}

	trimmedDesc := strings.TrimSpace(description)
	if len(trimmedDesc) > 500 {
		return nil, apperror.Validation("invalid project description", map[string]string{
			"description": "must not exceed 500 characters",
		})
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	return &Project{
		ID:             uuid.Must(uuid.NewV7()),
		OrganizationID: orgID,
		Slug:           slug,
		Name:           trimmedName,
		Description:    trimmedDesc,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

type CreateProjectCmd struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
