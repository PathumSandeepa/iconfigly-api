package project

import (
	"context"
	"testing"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/access"
	"github.com/PathumSandeepa/iconfigly-api/internal/database"
	"github.com/PathumSandeepa/iconfigly-api/internal/environment"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/valueobject"
	"github.com/google/uuid"
)

type memoryProjectRepo struct {
	projects map[uuid.UUID]*Project
}

func newMemoryProjectRepo() *memoryProjectRepo {
	return &memoryProjectRepo{projects: make(map[uuid.UUID]*Project)}
}

func (m *memoryProjectRepo) Create(ctx context.Context, p *Project) error {
	for _, ex := range m.projects {
		if ex.OrganizationID == p.OrganizationID && ex.Slug == p.Slug {
			return apperror.Conflict("project with this slug already exists in organization")
		}
	}
	m.projects[p.ID] = p
	return nil
}

func (m *memoryProjectRepo) GetByID(ctx context.Context, orgID, projectID uuid.UUID) (*Project, error) {
	p, ok := m.projects[projectID]
	if !ok || p.OrganizationID != orgID {
		return nil, apperror.NotFound("project")
	}
	return p, nil
}

func (m *memoryProjectRepo) GetBySlug(ctx context.Context, orgID uuid.UUID, slug valueobject.Slug) (*Project, error) {
	for _, p := range m.projects {
		if p.OrganizationID == orgID && p.Slug == slug {
			return p, nil
		}
	}
	return nil, apperror.NotFound("project")
}

func (m *memoryProjectRepo) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]*Project, error) {
	var list []*Project
	for _, p := range m.projects {
		if p.OrganizationID == orgID {
			list = append(list, p)
		}
	}
	return list, nil
}

type memoryEnvRepo struct {
	envs map[uuid.UUID][]*environment.Environment
}

func newMemoryEnvRepo() *memoryEnvRepo {
	return &memoryEnvRepo{envs: make(map[uuid.UUID][]*environment.Environment)}
}

func (m *memoryEnvRepo) CreateBatch(ctx context.Context, envs []*environment.Environment) error {
	for _, env := range envs {
		m.envs[env.ProjectID] = append(m.envs[env.ProjectID], env)
	}
	return nil
}

type mockAccessRepo struct {
	allowedUsers map[uuid.UUID]access.RoleName
}

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
	role, ok := m.allowedUsers[userID]
	if !ok {
		return nil, nil
	}
	return []*access.RoleBinding{
		{
			OrganizationID: orgID,
			SubjectType:    access.SubjectUser,
			SubjectID:      userID,
			RoleName:       role,
			ScopeType:      access.ScopeOrg,
			ScopeID:        orgID,
		},
	}, nil
}

func (m *mockAccessRepo) FindActiveBindingsForScopes(ctx context.Context, orgID uuid.UUID, scopes []access.ScopeRef, now time.Time) ([]*access.RoleBinding, error) {
	return nil, nil
}

func TestProjectServiceCreateAndSeedEnvironments(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.Must(uuid.NewV7())

	projRepo := newMemoryProjectRepo()
	envRepo := newMemoryEnvRepo()
	accessRepo := &mockAccessRepo{allowedUsers: map[uuid.UUID]access.RoleName{}}
	authorizer := access.NewAuthorizer(accessRepo)
	txManager := database.NewTxManager(nil)

	svc := NewService(projRepo, envRepo, authorizer, nil, txManager)

	adminActor := access.Actor{
		UserID:         uuid.Must(uuid.NewV7()),
		OrganizationID: orgID,
		IsSuperAdmin:   true,
	}

	p, err := svc.CreateProject(ctx, adminActor, CreateProjectCmd{
		Slug:        "checkout-hub",
		Name:        "Checkout Hub",
		Description: "Global checkout and payment processing",
	})
	if err != nil {
		t.Fatalf("unexpected error creating project: %v", err)
	}

	if p.Name != "Checkout Hub" {
		t.Fatalf("expected project name Checkout Hub, got %s", p.Name)
	}

	seededEnvs := envRepo.envs[p.ID]
	if len(seededEnvs) != 4 {
		t.Fatalf("expected 4 seeded environments, got %d", len(seededEnvs))
	}

	expectedSlugs := []string{"dev", "qa", "uat", "prod"}
	for i, expSlug := range expectedSlugs {
		if seededEnvs[i].Slug.String() != expSlug {
			t.Errorf("expected env[%d] slug %s, got %s", i, expSlug, seededEnvs[i].Slug.String())
		}
	}

	_, err = svc.CreateProject(ctx, adminActor, CreateProjectCmd{
		Slug:        "checkout-hub",
		Name:        "Duplicate Hub",
		Description: "Testing duplicate slug",
	})
	if err == nil {
		t.Fatalf("expected conflict on duplicate slug, got nil")
	}

	unauthActor := access.Actor{
		UserID:         uuid.Must(uuid.NewV7()),
		OrganizationID: orgID,
		IsSuperAdmin:   false,
	}
	_, err = svc.CreateProject(ctx, unauthActor, CreateProjectCmd{
		Slug:        "other-project",
		Name:        "Other Project",
		Description: "Testing unauthorized",
	})
	if err == nil {
		t.Fatalf("expected forbidden error for unauthorized actor, got nil")
	}

	projects, err := svc.ListProjects(ctx, adminActor)
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}

	got, err := svc.GetProject(ctx, adminActor, p.ID)
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	if got.ID != p.ID {
		t.Fatalf("expected project id %s, got %s", p.ID, got.ID)
	}
}
