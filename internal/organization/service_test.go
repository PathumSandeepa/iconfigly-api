package organization

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

type memoryOrgRepo struct {
	orgs map[uuid.UUID]*Organization
}

func newMemoryOrgRepo() *memoryOrgRepo {
	return &memoryOrgRepo{orgs: make(map[uuid.UUID]*Organization)}
}

func (m *memoryOrgRepo) Count(ctx context.Context) (int64, error) {
	return int64(len(m.orgs)), nil
}

func (m *memoryOrgRepo) Create(ctx context.Context, org *Organization) error {
	m.orgs[org.ID] = org
	return nil
}

func (m *memoryOrgRepo) GetByID(ctx context.Context, id uuid.UUID) (*Organization, error) {
	org, ok := m.orgs[id]
	if !ok {
		return nil, apperror.NotFound("organization")
	}
	return org, nil
}

func (m *memoryOrgRepo) GetBySlug(ctx context.Context, slug valueobject.Slug) (*Organization, error) {
	for _, o := range m.orgs {
		if o.Slug == slug {
			return o, nil
		}
	}
	return nil, apperror.NotFound("organization")
}

func (m *memoryOrgRepo) Update(ctx context.Context, org *Organization) error {
	if _, ok := m.orgs[org.ID]; !ok {
		return apperror.NotFound("organization")
	}
	m.orgs[org.ID] = org
	return nil
}

func TestOrganizationServiceGetAndUpdate(t *testing.T) {
	repo := newMemoryOrgRepo()
	txManager := database.NewTxManager(nil)
	authorizer := access.NewAuthorizer(&mockAuthorizerRepo{})
	svc := NewService(repo, authorizer, nil, txManager)
	ctx := context.Background()

	slug, err := valueobject.NewSlug("acme-fintech")
	if err != nil {
		t.Fatalf("slug: %v", err)
	}

	org, err := NewOrganization(slug, "Acme Fintech")
	if err != nil {
		t.Fatalf("new org: %v", err)
	}
	_ = repo.Create(ctx, org)

	got, err := svc.Get(ctx, org.ID)
	if err != nil {
		t.Fatalf("get org: %v", err)
	}
	if got.Name != "Acme Fintech" {
		t.Fatalf("expected name Acme Fintech, got %s", got.Name)
	}

	adminActor := access.Actor{
		UserID:         uuid.Must(uuid.NewV7()),
		OrganizationID: org.ID,
		IsSuperAdmin:   true,
	}

	updated, err := svc.Update(ctx, adminActor, org.ID, "Acme Corp")
	if err != nil {
		t.Fatalf("update org: %v", err)
	}
	if updated.Name != "Acme Corp" {
		t.Fatalf("expected Acme Corp, got %s", updated.Name)
	}
}

type mockAuthorizerRepo struct{}

func (m *mockAuthorizerRepo) Create(ctx context.Context, binding *access.RoleBinding) error {
	return nil
}

func (m *mockAuthorizerRepo) GetByID(ctx context.Context, orgID uuid.UUID, id uuid.UUID) (*access.RoleBinding, error) {
	return nil, nil
}

func (m *mockAuthorizerRepo) Delete(ctx context.Context, orgID uuid.UUID, id uuid.UUID) error {
	return nil
}

func (m *mockAuthorizerRepo) ListByProject(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID) ([]*access.RoleBinding, error) {
	return nil, nil
}

func (m *mockAuthorizerRepo) ListByScope(ctx context.Context, orgID uuid.UUID, scopeType access.ScopeType, scopeID uuid.UUID) ([]*access.RoleBinding, error) {
	return nil, nil
}

func (m *mockAuthorizerRepo) FindActiveBindingsForActor(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, teamIDs []uuid.UUID, scopes []access.ScopeRef, now time.Time) ([]*access.RoleBinding, error) {
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

func (m *mockAuthorizerRepo) FindActiveBindingsForScopes(ctx context.Context, orgID uuid.UUID, scopes []access.ScopeRef, now time.Time) ([]*access.RoleBinding, error) {
	return nil, nil
}
