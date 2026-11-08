package resource

import (
	"context"
	"testing"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/access"
	"github.com/PathumSandeepa/iconfigly-api/internal/database"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/valueobject"
	"github.com/PathumSandeepa/iconfigly-api/internal/team"
	"github.com/google/uuid"
)

type memoryResourceRepo struct {
	resources map[uuid.UUID]*Resource
}

func newMemoryResourceRepo() *memoryResourceRepo {
	return &memoryResourceRepo{resources: make(map[uuid.UUID]*Resource)}
}

func (m *memoryResourceRepo) Create(ctx context.Context, r *Resource) error {
	for _, ex := range m.resources {
		if ex.EnvironmentID == r.EnvironmentID && ex.Slug == r.Slug {
			return apperror.Conflict("resource with this slug already exists in environment")
		}
	}
	m.resources[r.ID] = r
	return nil
}

func (m *memoryResourceRepo) GetByID(ctx context.Context, orgID, projectID, resourceID uuid.UUID) (*Resource, error) {
	r, ok := m.resources[resourceID]
	if !ok || r.OrganizationID != orgID || r.ProjectID != projectID {
		return nil, apperror.NotFound("resource")
	}
	return r, nil
}

func (m *memoryResourceRepo) GetBySlug(ctx context.Context, orgID, projectID, envID uuid.UUID, slug valueobject.Slug) (*Resource, error) {
	for _, r := range m.resources {
		if r.OrganizationID == orgID && r.ProjectID == projectID && r.EnvironmentID == envID && r.Slug == slug {
			return r, nil
		}
	}
	return nil, apperror.NotFound("resource")
}

func (m *memoryResourceRepo) List(
	ctx context.Context,
	orgID, projectID uuid.UUID,
	envID *uuid.UUID,
	resType *ResourceType,
) ([]*Resource, error) {
	var list []*Resource
	for _, r := range m.resources {
		if r.OrganizationID != orgID || r.ProjectID != projectID {
			continue
		}
		if envID != nil && r.EnvironmentID != *envID {
			continue
		}
		if resType != nil && r.ResourceType != *resType {
			continue
		}
		list = append(list, r)
	}
	return list, nil
}

type memoryTeamRepo struct {
	teams map[uuid.UUID]*team.Team
}

func newMemoryTeamRepo() *memoryTeamRepo {
	return &memoryTeamRepo{teams: make(map[uuid.UUID]*team.Team)}
}

func (m *memoryTeamRepo) GetByID(ctx context.Context, orgID, teamID uuid.UUID) (*team.Team, error) {
	t, ok := m.teams[teamID]
	if !ok || t.OrganizationID != orgID {
		return nil, apperror.NotFound("team")
	}
	return t, nil
}

type mockAccessRepo struct{}

func (m *mockAccessRepo) Create(ctx context.Context, binding *access.RoleBinding) error {
	return nil
}

func (m *mockAccessRepo) GetByID(ctx context.Context, orgID uuid.UUID, id uuid.UUID) (*access.RoleBinding, error) {
	return nil, nil
}

func (m *mockAccessRepo) Delete(ctx context.Context, orgID uuid.UUID, id uuid.UUID) error {
	return nil
}

func (m *mockAccessRepo) ListByProject(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID) ([]*access.RoleBinding, error) {
	return nil, nil
}

func (m *mockAccessRepo) ListByScope(ctx context.Context, orgID uuid.UUID, scopeType access.ScopeType, scopeID uuid.UUID) ([]*access.RoleBinding, error) {
	return nil, nil
}

func (m *mockAccessRepo) FindActiveBindingsForActor(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, teamIDs []uuid.UUID, scopes []access.ScopeRef, now time.Time) ([]*access.RoleBinding, error) {
	return []*access.RoleBinding{
		{
			OrganizationID: orgID,
			SubjectType:    access.SubjectUser,
			SubjectID:      userID,
			RoleName:       access.RoleOrgAdmin,
			ScopeType:      access.ScopeOrg,
			ScopeID:        orgID,
		},
	}, nil
}

func (m *mockAccessRepo) FindActiveBindingsForScopes(ctx context.Context, orgID uuid.UUID, scopes []access.ScopeRef, now time.Time) ([]*access.RoleBinding, error) {
	return nil, nil
}

func TestResourceServiceCreateAndList(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.Must(uuid.NewV7())
	projID := uuid.Must(uuid.NewV7())
	envID := uuid.Must(uuid.NewV7())

	teamRepo := newMemoryTeamRepo()
	teamSlug, _ := valueobject.NewSlug("payments-team")
	ownerTeam, _ := team.NewTeam(orgID, teamSlug, "Payments Team", "Owner")
	teamRepo.teams[ownerTeam.ID] = ownerTeam

	resRepo := newMemoryResourceRepo()
	authorizer := access.NewAuthorizer(&mockAccessRepo{})
	txManager := database.NewTxManager(nil)
	svc := NewService(resRepo, teamRepo, authorizer, nil, txManager)

	adminActor := access.Actor{
		UserID:         uuid.Must(uuid.NewV7()),
		OrganizationID: orgID,
		IsSuperAdmin:   true,
	}

	_, err := svc.CreateResource(ctx, adminActor, projID, CreateResourceCmd{
		EnvironmentID: envID,
		OwnerTeamID:   ownerTeam.ID,
		Slug:          "payments-db",
		Name:          "Payments PostgreSQL DB",
		ResourceType:  TypePostgres,
		ConnectionMetadata: map[string]any{
			"password": "plain-text-secret",
		},
	})
	if err == nil {
		t.Fatalf("expected validation error for plaintext password in connection metadata, got nil")
	}

	res, err := svc.CreateResource(ctx, adminActor, projID, CreateResourceCmd{
		EnvironmentID: envID,
		OwnerTeamID:   ownerTeam.ID,
		Slug:          "payments-db",
		Name:          "Payments PostgreSQL DB",
		ResourceType:  TypePostgres,
		ConnectionMetadata: map[string]any{
			"host": "postgres.internal",
			"port": 5432,
			"database": "payments_prod",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error creating resource: %v", err)
	}
	if res.Version != 1 {
		t.Fatalf("expected initial version 1, got %d", res.Version)
	}

	_, err = svc.CreateResource(ctx, adminActor, projID, CreateResourceCmd{
		EnvironmentID: envID,
		OwnerTeamID:   ownerTeam.ID,
		Slug:          "payments-db",
		Name:          "Duplicate Resource",
		ResourceType:  TypePostgres,
	})
	if err == nil {
		t.Fatalf("expected conflict error on duplicate slug, got nil")
	}

	nonexistentTeamID := uuid.Must(uuid.NewV7())
	_, err = svc.CreateResource(ctx, adminActor, projID, CreateResourceCmd{
		EnvironmentID: envID,
		OwnerTeamID:   nonexistentTeamID,
		Slug:          "redis-cache",
		Name:          "Redis Cache",
		ResourceType:  TypeRedis,
	})
	if err == nil {
		t.Fatalf("expected not found error for nonexistent owner team, got nil")
	}

	list, err := svc.ListResources(ctx, adminActor, projID, &envID, nil)
	if err != nil {
		t.Fatalf("list resources: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 resource in list, got %d", len(list))
	}
}
