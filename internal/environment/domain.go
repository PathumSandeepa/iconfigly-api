package environment

import (
	"strings"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/valueobject"
	"github.com/google/uuid"
)

type Environment struct {
	ID                 uuid.UUID        `json:"id"`
	OrganizationID     uuid.UUID        `json:"organization_id"`
	ProjectID          uuid.UUID        `json:"project_id"`
	Slug               valueobject.Slug `json:"slug"`
	Name               string           `json:"name"`
	TierOrder          int              `json:"tier_order"`
	IsProduction       bool             `json:"is_production"`
	RequiredApprovals  int              `json:"required_approvals"`
	CreatedAt          time.Time        `json:"created_at"`
	UpdatedAt          time.Time        `json:"updated_at"`
}

func NewEnvironment(
	orgID uuid.UUID,
	projectID uuid.UUID,
	slug valueobject.Slug,
	name string,
	tierOrder int,
	isProduction bool,
	requiredApprovals int,
) (*Environment, error) {
	trimmedName := strings.TrimSpace(name)
	if len(trimmedName) < 2 || len(trimmedName) > 100 {
		return nil, apperror.Validation("invalid environment name", map[string]string{
			"name": "must be between 2 and 100 characters",
		})
	}

	if tierOrder < 1 {
		return nil, apperror.Validation("invalid tier order", map[string]string{
			"tier_order": "must be greater than or equal to 1",
		})
	}

	if requiredApprovals < 0 {
		return nil, apperror.Validation("invalid required approvals", map[string]string{
			"required_approvals": "must be greater than or equal to 0",
		})
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	return &Environment{
		ID:                uuid.Must(uuid.NewV7()),
		OrganizationID:    orgID,
		ProjectID:         projectID,
		Slug:              slug,
		Name:              trimmedName,
		TierOrder:         tierOrder,
		IsProduction:      isProduction,
		RequiredApprovals: requiredApprovals,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}

func DefaultEnvironments(orgID, projectID uuid.UUID) ([]*Environment, error) {
	devSlug, err := valueobject.NewSlug("dev")
	if err != nil {
		return nil, err
	}
	dev, err := NewEnvironment(orgID, projectID, devSlug, "Development", 1, false, 0)
	if err != nil {
		return nil, err
	}

	qaSlug, err := valueobject.NewSlug("qa")
	if err != nil {
		return nil, err
	}
	qa, err := NewEnvironment(orgID, projectID, qaSlug, "Quality Assurance", 2, false, 0)
	if err != nil {
		return nil, err
	}

	uatSlug, err := valueobject.NewSlug("uat")
	if err != nil {
		return nil, err
	}
	uat, err := NewEnvironment(orgID, projectID, uatSlug, "User Acceptance Testing", 3, false, 1)
	if err != nil {
		return nil, err
	}

	prodSlug, err := valueobject.NewSlug("prod")
	if err != nil {
		return nil, err
	}
	prod, err := NewEnvironment(orgID, projectID, prodSlug, "Production", 4, true, 2)
	if err != nil {
		return nil, err
	}

	return []*Environment{dev, qa, uat, prod}, nil
}

type UpdatePolicyCmd struct {
	IsProduction      *bool `json:"is_production"`
	RequiredApprovals *int  `json:"required_approvals"`
}
