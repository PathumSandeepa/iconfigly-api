package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/access"
	"github.com/PathumSandeepa/iconfigly-api/internal/audit"
	"github.com/PathumSandeepa/iconfigly-api/internal/binding"
	"github.com/PathumSandeepa/iconfigly-api/internal/database"
	"github.com/PathumSandeepa/iconfigly-api/internal/environment"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/valueobject"
	"github.com/PathumSandeepa/iconfigly-api/internal/team"
	"github.com/google/uuid"
)

type memoryServiceRepo struct {
	services map[uuid.UUID]*Service
}

func newMemoryServiceRepo() *memoryServiceRepo {
	return &memoryServiceRepo{services: make(map[uuid.UUID]*Service)}
}

func (m *memoryServiceRepo) Create(ctx context.Context, s *Service) error {
	for _, ex := range m.services {
		if ex.ProjectID == s.ProjectID && ex.Slug == s.Slug {
			return apperror.Conflict("service with this slug already exists in project")
		}
	}
	m.services[s.ID] = s
	return nil
}

func (m *memoryServiceRepo) GetByID(ctx context.Context, orgID, projectID, serviceID uuid.UUID) (*Service, error) {
	s, ok := m.services[serviceID]
	if !ok || s.OrganizationID != orgID || s.ProjectID != projectID {
		return nil, apperror.NotFound("service")
	}
	return s, nil
}

func (m *memoryServiceRepo) GetBySlug(ctx context.Context, orgID, projectID uuid.UUID, slug valueobject.Slug) (*Service, error) {
	for _, s := range m.services {
		if s.OrganizationID == orgID && s.ProjectID == projectID && s.Slug == slug {
			return s, nil
		}
	}
	return nil, apperror.NotFound("service")
}

func (m *memoryServiceRepo) ListByProject(
	ctx context.Context,
	orgID, projectID uuid.UUID,
	ownerTeamID *uuid.UUID,
	search string,
) ([]*Service, error) {
	var list []*Service
	for _, s := range m.services {
		if s.OrganizationID != orgID || s.ProjectID != projectID {
			continue
		}
		if ownerTeamID != nil && s.OwnerTeamID != *ownerTeamID {
			continue
		}
		if search != "" {
			if !strings.Contains(s.Name, search) && !strings.Contains(s.Slug.String(), search) {
				continue
			}
		}
		list = append(list, s)
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

type memoryEnvRepo struct {
	envs map[uuid.UUID]*environment.Environment
}

func newMemoryEnvRepo() *memoryEnvRepo {
	return &memoryEnvRepo{envs: make(map[uuid.UUID]*environment.Environment)}
}

func (m *memoryEnvRepo) GetByID(ctx context.Context, orgID, projectID, envID uuid.UUID) (*environment.Environment, error) {
	e, ok := m.envs[envID]
	if !ok || e.OrganizationID != orgID || e.ProjectID != projectID {
		return nil, apperror.NotFound("environment")
	}
	return e, nil
}

func (m *memoryEnvRepo) GetBySlug(ctx context.Context, orgID, projectID uuid.UUID, slug valueobject.Slug) (*environment.Environment, error) {
	for _, e := range m.envs {
		if e.OrganizationID == orgID && e.ProjectID == projectID && e.Slug == slug {
			return e, nil
		}
	}
	return nil, apperror.NotFound("environment")
}

type memoryBindingRepo struct {
	boundResources []*binding.BoundResourceView
	upstream       []*binding.ServiceDependencyView
	downstream     []*binding.ServiceDependencyView
}

func (m *memoryBindingRepo) ListBoundResourcesByService(ctx context.Context, serviceID, envID uuid.UUID) ([]*binding.BoundResourceView, error) {
	return m.boundResources, nil
}

func (m *memoryBindingRepo) ListDependenciesForService(ctx context.Context, serviceID, envID uuid.UUID) ([]*binding.ServiceDependencyView, []*binding.ServiceDependencyView, error) {
	return m.upstream, m.downstream, nil
}

type memoryAuditRepo struct {
	entries []*audit.AuditEntry
}

func (m *memoryAuditRepo) ListPaginated(ctx context.Context, filter audit.QueryFilter) ([]*audit.AuditEntry, error) {
	return m.entries, nil
}

type mockAccessRepo struct{}

func (m *mockAccessRepo) Create(ctx context.Context, b *access.RoleBinding) error {
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

func TestServiceServiceCreateAndList(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.Must(uuid.NewV7())
	projID := uuid.Must(uuid.NewV7())

	svcRepo := newMemoryServiceRepo()
	teamRepo := newMemoryTeamRepo()
	envRepo := newMemoryEnvRepo()
	bindingRepo := &memoryBindingRepo{}
	auditRepo := &memoryAuditRepo{}
	authorizer := access.NewAuthorizer(&mockAccessRepo{})
	txManager := database.NewTxManager(nil)

	teamSlug, _ := valueobject.NewSlug("payments-team")
	ownerTeam, _ := team.NewTeam(orgID, teamSlug, "Payments Team", "Owner")
	teamRepo.teams[ownerTeam.ID] = ownerTeam

	svc := NewService(svcRepo, teamRepo, envRepo, bindingRepo, auditRepo, authorizer, nil, txManager)

	adminActor := access.Actor{
		UserID:         uuid.Must(uuid.NewV7()),
		OrganizationID: orgID,
		IsSuperAdmin:   true,
	}

	created, err := svc.CreateService(ctx, adminActor, projID, CreateServiceCmd{
		OwnerTeamID: ownerTeam.ID,
		Slug:        "payment-service",
		Name:        "Payment Service",
		Description: "Handles checkout transactions",
		RepoURL:     "https://github.com/org/payment-service",
		RunbookURL:  "https://wiki.org/payment-service",
	})
	if err != nil {
		t.Fatalf("unexpected error creating service: %v", err)
	}

	if created.Name != "Payment Service" {
		t.Fatalf("expected name Payment Service, got %s", created.Name)
	}

	_, err = svc.CreateService(ctx, adminActor, projID, CreateServiceCmd{
		OwnerTeamID: ownerTeam.ID,
		Slug:        "payment-service",
		Name:        "Duplicate Service",
	})
	if err == nil {
		t.Fatalf("expected conflict error on duplicate slug, got nil")
	}

	list, err := svc.ListServices(ctx, adminActor, projID, nil, "")
	if err != nil {
		t.Fatalf("list services: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 service, got %d", len(list))
	}
}

func TestServiceServiceGetEnvironmentContext(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.Must(uuid.NewV7())
	projID := uuid.Must(uuid.NewV7())

	svcRepo := newMemoryServiceRepo()
	teamRepo := newMemoryTeamRepo()
	envRepo := newMemoryEnvRepo()
	bindingRepo := &memoryBindingRepo{}
	auditRepo := &memoryAuditRepo{}
	authorizer := access.NewAuthorizer(&mockAccessRepo{})
	txManager := database.NewTxManager(nil)

	teamSlug, _ := valueobject.NewSlug("payments-team")
	ownerTeam, _ := team.NewTeam(orgID, teamSlug, "Payments Team", "Owner")
	teamRepo.teams[ownerTeam.ID] = ownerTeam

	uatSlug, _ := valueobject.NewSlug("uat")
	uatEnv, _ := environment.NewEnvironment(orgID, projID, uatSlug, "User Acceptance Testing", 3, false, 1)
	envRepo.envs[uatEnv.ID] = uatEnv

	serviceSlug, _ := valueobject.NewSlug("payment-service")
	paymentSvc, _ := NewServiceEntity(orgID, projID, ownerTeam.ID, serviceSlug, "Payment Service", "Desc", "", "")
	svcRepo.services[paymentSvc.ID] = paymentSvc

	bindingRepo.boundResources = []*binding.BoundResourceView{
		{
			BindingID:    uuid.Must(uuid.NewV7()),
			ResourceID:   uuid.Must(uuid.NewV7()),
			ResourceSlug: "payments-db",
			ResourceName: "Payments PostgreSQL DB",
			ResourceType: "POSTGRES",
			AccessMode:   "READ_WRITE",
		},
	}

	bindingRepo.downstream = []*binding.ServiceDependencyView{
		{
			DependencyID: uuid.Must(uuid.NewV7()),
			ServiceID:    uuid.Must(uuid.NewV7()),
			ServiceSlug:  "fraud-service",
			ServiceName:  "Fraud Detection Service",
			Protocol:     "GRPC",
		},
	}

	svc := NewService(svcRepo, teamRepo, envRepo, bindingRepo, auditRepo, authorizer, nil, txManager)

	adminActor := access.Actor{
		UserID:         uuid.Must(uuid.NewV7()),
		OrganizationID: orgID,
		IsSuperAdmin:   true,
	}

	contextView, err := svc.GetServiceEnvironmentContext(ctx, adminActor, projID, "payment-service", "uat")
	if err != nil {
		t.Fatalf("unexpected error getting service context: %v", err)
	}

	if contextView.Service.Name != "Payment Service" {
		t.Fatalf("expected service name Payment Service, got %s", contextView.Service.Name)
	}
	if contextView.OwnerTeam.Name != "Payments Team" {
		t.Fatalf("expected owner team Payments Team, got %s", contextView.OwnerTeam.Name)
	}
	if contextView.Environment.Slug != "uat" {
		t.Fatalf("expected environment uat, got %s", contextView.Environment.Slug)
	}
	if len(contextView.BoundResources) != 1 {
		t.Fatalf("expected 1 bound resource, got %d", len(contextView.BoundResources))
	}
	if len(contextView.DownstreamDependencies) != 1 {
		t.Fatalf("expected 1 downstream dependency, got %d", len(contextView.DownstreamDependencies))
	}
	if contextView.CallerEffectiveRole != "ORG_ADMIN" {
		t.Fatalf("expected effective role ORG_ADMIN, got %s", contextView.CallerEffectiveRole)
	}
}
