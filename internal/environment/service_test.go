package environment

import (
	"context"
	"testing"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/access"
	"github.com/PathumSandeepa/iconfigly-api/internal/database"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/valueobject"
	"github.com/google/uuid"
)

type memoryEnvironmentRepo struct {
	envs map[uuid.UUID]*Environment
}

func newMemoryEnvironmentRepo() *memoryEnvironmentRepo {
	return &memoryEnvironmentRepo{envs: make(map[uuid.UUID]*Environment)}
}

func (m *memoryEnvironmentRepo) Create(ctx context.Context, env *Environment) error {
	for _, ex := range m.envs {
		if ex.ProjectID == env.ProjectID && ex.Slug == env.Slug {
			return apperror.Conflict("environment with this slug already exists in project")
		}
	}
	m.envs[env.ID] = env
	return nil
}

func (m *memoryEnvironmentRepo) CreateBatch(ctx context.Context, envs []*Environment) error {
	for _, env := range envs {
		if err := m.Create(ctx, env); err != nil {
			return err
		}
	}
	return nil
}

func (m *memoryEnvironmentRepo) GetByID(ctx context.Context, orgID, projectID, envID uuid.UUID) (*Environment, error) {
	e, ok := m.envs[envID]
	if !ok || e.OrganizationID != orgID || e.ProjectID != projectID {
		return nil, apperror.NotFound("environment")
	}
	return e, nil
}

func (m *memoryEnvironmentRepo) GetBySlug(ctx context.Context, orgID, projectID uuid.UUID, slug valueobject.Slug) (*Environment, error) {
	for _, e := range m.envs {
		if e.OrganizationID == orgID && e.ProjectID == projectID && e.Slug == slug {
			return e, nil
		}
	}
	return nil, apperror.NotFound("environment")
}

func (m *memoryEnvironmentRepo) ListByProject(ctx context.Context, orgID, projectID uuid.UUID) ([]*Environment, error) {
	var list []*Environment
	for _, e := range m.envs {
		if e.OrganizationID == orgID && e.ProjectID == projectID {
			list = append(list, e)
		}
	}
	return list, nil
}

func (m *memoryEnvironmentRepo) UpdatePolicy(ctx context.Context, orgID, projectID, envID uuid.UUID, isProduction bool, requiredApprovals int) error {
	e, ok := m.envs[envID]
	if !ok || e.OrganizationID != orgID || e.ProjectID != projectID {
		return apperror.NotFound("environment")
	}
	e.IsProduction = isProduction
	e.RequiredApprovals = requiredApprovals
	return nil
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

func TestDefaultEnvironmentsGeneration(t *testing.T) {
	orgID := uuid.Must(uuid.NewV7())
	projID := uuid.Must(uuid.NewV7())

	envs, err := DefaultEnvironments(orgID, projID)
	if err != nil {
		t.Fatalf("unexpected error creating default environments: %v", err)
	}

	if len(envs) != 4 {
		t.Fatalf("expected 4 default environments, got %d", len(envs))
	}

	expected := []struct {
		slug         string
		name         string
		tier         int
		isProduction bool
		approvals    int
	}{
		{"dev", "Development", 1, false, 0},
		{"qa", "Quality Assurance", 2, false, 0},
		{"uat", "User Acceptance Testing", 3, false, 1},
		{"prod", "Production", 4, true, 2},
	}

	for i, exp := range expected {
		e := envs[i]
		if e.Slug.String() != exp.slug {
			t.Errorf("env[%d] slug: expected %s, got %s", i, exp.slug, e.Slug.String())
		}
		if e.Name != exp.name {
			t.Errorf("env[%d] name: expected %s, got %s", i, exp.name, e.Name)
		}
		if e.TierOrder != exp.tier {
			t.Errorf("env[%d] tier: expected %d, got %d", i, exp.tier, e.TierOrder)
		}
		if e.IsProduction != exp.isProduction {
			t.Errorf("env[%d] isProduction: expected %v, got %v", i, exp.isProduction, e.IsProduction)
		}
		if e.RequiredApprovals != exp.approvals {
			t.Errorf("env[%d] approvals: expected %d, got %d", i, exp.approvals, e.RequiredApprovals)
		}
	}
}

func TestEnvironmentServiceListAndUpdatePolicy(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.Must(uuid.NewV7())
	projID := uuid.Must(uuid.NewV7())

	repo := newMemoryEnvironmentRepo()
	authorizer := access.NewAuthorizer(&mockAccessRepo{})
	txManager := database.NewTxManager(nil)
	svc := NewService(repo, authorizer, nil, txManager)

	envs, err := DefaultEnvironments(orgID, projID)
	if err != nil {
		t.Fatalf("default envs: %v", err)
	}
	_ = repo.CreateBatch(ctx, envs)

	adminActor := access.Actor{
		UserID:         uuid.Must(uuid.NewV7()),
		OrganizationID: orgID,
		IsSuperAdmin:   true,
	}

	list, err := svc.ListEnvironments(ctx, adminActor, projID)
	if err != nil {
		t.Fatalf("list environments: %v", err)
	}
	if len(list) != 4 {
		t.Fatalf("expected 4 environments, got %d", len(list))
	}

	prodEnv := envs[3]
	newApprovals := 3
	updated, err := svc.UpdateEnvironmentPolicy(ctx, adminActor, projID, prodEnv.ID, UpdatePolicyCmd{
		RequiredApprovals: &newApprovals,
	})
	if err != nil {
		t.Fatalf("update policy: %v", err)
	}
	if updated.RequiredApprovals != 3 {
		t.Fatalf("expected 3 required approvals, got %d", updated.RequiredApprovals)
	}

	invalidApprovals := -1
	_, err = svc.UpdateEnvironmentPolicy(ctx, adminActor, projID, prodEnv.ID, UpdatePolicyCmd{
		RequiredApprovals: &invalidApprovals,
	})
	if err == nil {
		t.Fatalf("expected validation error on negative approvals, got nil")
	}
}
