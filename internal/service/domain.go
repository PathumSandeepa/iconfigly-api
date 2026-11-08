package service

import (
	"strings"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/valueobject"
	"github.com/google/uuid"
)

type Service struct {
	ID             uuid.UUID        `json:"id"`
	OrganizationID uuid.UUID        `json:"organization_id"`
	ProjectID      uuid.UUID        `json:"project_id"`
	OwnerTeamID    uuid.UUID        `json:"owner_team_id"`
	Slug           valueobject.Slug `json:"slug"`
	Name           string           `json:"name"`
	Description    string           `json:"description"`
	RepoURL        string           `json:"repo_url"`
	RunbookURL     string           `json:"runbook_url"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

func NewServiceEntity(
	orgID uuid.UUID,
	projectID uuid.UUID,
	ownerTeamID uuid.UUID,
	slug valueobject.Slug,
	name string,
	description string,
	repoURL string,
	runbookURL string,
) (*Service, error) {
	trimmedName := strings.TrimSpace(name)
	if len(trimmedName) < 2 || len(trimmedName) > 100 {
		return nil, apperror.Validation("invalid service name", map[string]string{
			"name": "must be between 2 and 100 characters",
		})
	}

	trimmedDesc := strings.TrimSpace(description)
	if len(trimmedDesc) > 500 {
		return nil, apperror.Validation("invalid service description", map[string]string{
			"description": "must not exceed 500 characters",
		})
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	return &Service{
		ID:             uuid.Must(uuid.NewV7()),
		OrganizationID: orgID,
		ProjectID:      projectID,
		OwnerTeamID:    ownerTeamID,
		Slug:           slug,
		Name:           trimmedName,
		Description:    trimmedDesc,
		RepoURL:        strings.TrimSpace(repoURL),
		RunbookURL:     strings.TrimSpace(runbookURL),
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

type CreateServiceCmd struct {
	OwnerTeamID uuid.UUID `json:"owner_team_id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	RepoURL     string    `json:"repo_url"`
	RunbookURL  string    `json:"runbook_url"`
}

type OwnerTeamSummary struct {
	ID   uuid.UUID `json:"id"`
	Slug string    `json:"slug"`
	Name string    `json:"name"`
}

type EnvironmentSummary struct {
	ID           uuid.UUID `json:"id"`
	Slug         string    `json:"slug"`
	Name         string    `json:"name"`
	TierOrder    int       `json:"tier_order"`
	IsProduction bool      `json:"is_production"`
}

type BoundResourceItem struct {
	ID                 uuid.UUID      `json:"id"`
	BindingID          uuid.UUID      `json:"binding_id"`
	Slug               string         `json:"slug"`
	Name               string         `json:"name"`
	ResourceType       string         `json:"resource_type"`
	AccessMode         string         `json:"access_mode"`
	ConnectionMetadata map[string]any `json:"connection_metadata"`
	Notes              string         `json:"notes"`
}

type DependencyItem struct {
	ServiceID   uuid.UUID `json:"service_id"`
	ServiceSlug string    `json:"service_slug"`
	ServiceName string    `json:"service_name"`
	Protocol    string    `json:"protocol"`
}

type LatestAuditSummary struct {
	Action      string     `json:"action"`
	ActorUserID *uuid.UUID `json:"actor_user_id,omitempty"`
	Timestamp   time.Time  `json:"timestamp"`
}

type ServiceContextView struct {
	Service                *Service            `json:"service"`
	OwnerTeam              OwnerTeamSummary    `json:"owner_team"`
	Environment            EnvironmentSummary  `json:"environment"`
	BoundResources         []BoundResourceItem `json:"bound_resources"`
	UpstreamDependencies   []DependencyItem    `json:"upstream_dependencies"`
	DownstreamDependencies []DependencyItem   `json:"downstream_dependencies"`
	CallerEffectiveRole    string              `json:"caller_effective_role"`
	LatestAuditEntry       *LatestAuditSummary `json:"latest_audit_entry,omitempty"`
}
