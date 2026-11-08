package resource

import (
	"strings"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/valueobject"
	"github.com/google/uuid"
)

type ResourceType string

const (
	TypePostgres    ResourceType = "POSTGRES"
	TypeMySQL       ResourceType = "MYSQL"
	TypeRedis       ResourceType = "REDIS"
	TypeKafka       ResourceType = "KAFKA"
	TypeS3          ResourceType = "S3"
	TypeExternalAPI ResourceType = "EXTERNAL_API"
	TypeCustom      ResourceType = "CUSTOM"
)

func IsValidResourceType(t ResourceType) bool {
	switch t {
	case TypePostgres, TypeMySQL, TypeRedis, TypeKafka, TypeS3, TypeExternalAPI, TypeCustom:
		return true
	default:
		return false
	}
}

func ValidateConnectionMetadata(meta map[string]any) error {
	for k := range meta {
		lower := strings.ToLower(k)
		if strings.Contains(lower, "password") ||
			strings.Contains(lower, "secret") ||
			strings.Contains(lower, "private_key") ||
			strings.Contains(lower, "apikey") ||
			strings.Contains(lower, "api_key") ||
			strings.Contains(lower, "token") {
			return apperror.Validation("forbidden sensitive key in connection metadata", map[string]string{
				k: "plaintext secrets, passwords or tokens must not be stored in catalog metadata",
			})
		}
	}
	return nil
}

type Resource struct {
	ID                 uuid.UUID        `json:"id"`
	OrganizationID     uuid.UUID        `json:"organization_id"`
	ProjectID          uuid.UUID        `json:"project_id"`
	EnvironmentID      uuid.UUID        `json:"environment_id"`
	OwnerTeamID        uuid.UUID        `json:"owner_team_id"`
	Slug               valueobject.Slug `json:"slug"`
	Name               string           `json:"name"`
	ResourceType       ResourceType     `json:"resource_type"`
	ConnectionMetadata map[string]any   `json:"connection_metadata"`
	Version            int              `json:"version"`
	CreatedAt          time.Time        `json:"created_at"`
	UpdatedAt          time.Time        `json:"updated_at"`
}

func NewResource(
	orgID uuid.UUID,
	projectID uuid.UUID,
	envID uuid.UUID,
	ownerTeamID uuid.UUID,
	slug valueobject.Slug,
	name string,
	resType ResourceType,
	connMeta map[string]any,
) (*Resource, error) {
	trimmedName := strings.TrimSpace(name)
	if len(trimmedName) < 2 || len(trimmedName) > 100 {
		return nil, apperror.Validation("invalid resource name", map[string]string{
			"name": "must be between 2 and 100 characters",
		})
	}

	if !IsValidResourceType(resType) {
		return nil, apperror.Validation("invalid resource type", map[string]string{
			"resource_type": "must be POSTGRES, MYSQL, REDIS, KAFKA, S3, EXTERNAL_API, or CUSTOM",
		})
	}

	if connMeta == nil {
		connMeta = make(map[string]any)
	}

	if err := ValidateConnectionMetadata(connMeta); err != nil {
		return nil, err
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	return &Resource{
		ID:                 uuid.Must(uuid.NewV7()),
		OrganizationID:     orgID,
		ProjectID:          projectID,
		EnvironmentID:      envID,
		OwnerTeamID:        ownerTeamID,
		Slug:               slug,
		Name:               trimmedName,
		ResourceType:       resType,
		ConnectionMetadata: connMeta,
		Version:            1,
		CreatedAt:          now,
		UpdatedAt:          now,
	}, nil
}

type CreateResourceCmd struct {
	EnvironmentID      uuid.UUID      `json:"environment_id"`
	OwnerTeamID        uuid.UUID      `json:"owner_team_id"`
	Slug               string         `json:"slug"`
	Name               string         `json:"name"`
	ResourceType       ResourceType   `json:"resource_type"`
	ConnectionMetadata map[string]any `json:"connection_metadata"`
}
