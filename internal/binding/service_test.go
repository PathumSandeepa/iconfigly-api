package binding

import (
	"context"
	"testing"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/access"
	"github.com/PathumSandeepa/iconfigly-api/internal/database"
	"github.com/PathumSandeepa/iconfigly-api/internal/resource"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/valueobject"
	"github.com/google/uuid"
)

type memoryBindingRepo struct {
	bindings []*ServiceResourceBinding
	deps     []*ServiceDependency
}

func newMemoryBindingRepo() *memoryBindingRepo {
	return &memoryBindingRepo{}
}

func (m *memoryBindingRepo) CreateBinding(ctx context.Context, b *ServiceResourceBinding) error {
	for _, ex := range m.bindings {
		if ex.EnvironmentID == b.EnvironmentID && ex.ServiceID == b.ServiceID && ex.ResourceID == b.ResourceID {
			return apperror.Conflict("service is already bound to this resource in environment")
		}
	}
	m.bindings = append(m.bindings, b)
	return nil
}

func (m *memoryBindingRepo) DeleteBinding(ctx context.Context, orgID, projectID, bindingID uuid.UUID) (*ServiceResourceBinding, error) {
	for i, b := range m.bindings {
		if b.ID == bindingID && b.OrganizationID == orgID && b.ProjectID == projectID {
			m.bindings = append(m.bindings[:i], m.bindings[i+1:]...)
			return b, nil
		}
	}
	return nil, apperror.NotFound("service resource binding")
}

func (m *memoryBindingRepo) ListBindingsByEnvironment(ctx context.Context, orgID, projectID, envID uuid.UUID) ([]*ServiceResourceBinding, error) {
	var list []*ServiceResourceBinding
	for _, b := range m.bindings {
		if b.OrganizationID == orgID && b.ProjectID == projectID && b.EnvironmentID == envID {
			list = append(list, b)
		}
	}
	return list, nil
}

func (m *memoryBindingRepo) ListBoundResourcesByService(ctx context.Context, serviceID, envID uuid.UUID) ([]*BoundResourceView, error) {
	return []*BoundResourceView{}, nil
}

func (m *memoryBindingRepo) CreateDependency(ctx context.Context, d *ServiceDependency) error {
	for _, ex := range m.deps {
		if ex.EnvironmentID == d.EnvironmentID && ex.CallerServiceID == d.CallerServiceID && ex.CalleeServiceID == d.CalleeServiceID {
			return apperror.Conflict("service dependency already exists in this environment")
		}
	}
	m.deps = append(m.deps, d)
	return nil
}

func (m *memoryBindingRepo) ListDependenciesByEnvironment(ctx context.Context, orgID, projectID, envID uuid.UUID) ([]*ServiceDependency, error) {
	var list []*ServiceDependency
	for _, d := range m.deps {
		if d.OrganizationID == orgID && d.ProjectID == projectID && d.EnvironmentID == envID {
			list = append(list, d)
		}
	}
	return list, nil
}

func (m *memoryBindingRepo) ListDependenciesForService(ctx context.Context, serviceID, envID uuid.UUID) ([]*ServiceDependencyView, []*ServiceDependencyView, error) {
	return []*ServiceDependencyView{}, []*ServiceDependencyView{}, nil
}

type memoryServiceChecker struct {
	services map[uuid.UUID]*ServiceSummary
}

func newMemoryServiceChecker() *memoryServiceChecker {
	return &memoryServiceChecker{services: make(map[uuid.UUID]*ServiceSummary)}
}

func (m *memoryServiceChecker) GetByID(ctx context.Context, orgID, projectID, serviceID uuid.UUID) (*ServiceSummary, error) {
	s, ok := m.services[serviceID]
	if !ok {
		return nil, apperror.NotFound("service")
	}
	return s, nil
}

func (m *memoryServiceChecker) ListByProject(ctx context.Context, orgID, projectID uuid.UUID) ([]*ServiceSummary, error) {
	var list []*ServiceSummary
	for _, s := range m.services {
		list = append(list, s)
	}
	return list, nil
}

type memoryResourceChecker struct {
	resources map[uuid.UUID]*resource.Resource
}

func newMemoryResourceChecker() *memoryResourceChecker {
	return &memoryResourceChecker{resources: make(map[uuid.UUID]*resource.Resource)}
}

func (m *memoryResourceChecker) GetByID(ctx context.Context, orgID, projectID, resourceID uuid.UUID) (*resource.Resource, error) {
	r, ok := m.resources[resourceID]
	if !ok || r.OrganizationID != orgID || r.ProjectID != projectID {
		return nil, apperror.NotFound("resource")
	}
	return r, nil
}

func (m *memoryResourceChecker) List(ctx context.Context, orgID, projectID uuid.UUID, envID *uuid.UUID, resType *resource.ResourceType) ([]*resource.Resource, error) {
	var list []*resource.Resource
	for _, r := range m.resources {
		if r.OrganizationID != orgID || r.ProjectID != projectID {
			continue
		}
		if envID != nil && r.EnvironmentID != *envID {
			continue
		}
		list = append(list, r)
	}
	return list, nil
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

func TestBindingServiceCrossEnvironmentValidation(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.Must(uuid.NewV7())
	projID := uuid.Must(uuid.NewV7())
	uatEnvID := uuid.Must(uuid.NewV7())
	prodEnvID := uuid.Must(uuid.NewV7())

	bindingRepo := newMemoryBindingRepo()
	svcChecker := newMemoryServiceChecker()
	resChecker := newMemoryResourceChecker()
	authorizer := access.NewAuthorizer(&mockAccessRepo{})
	txManager := database.NewTxManager(nil)

	svc := NewService(bindingRepo, svcChecker, resChecker, authorizer, nil, txManager)

	adminActor := access.Actor{
		UserID:         uuid.Must(uuid.NewV7()),
		OrganizationID: orgID,
		IsSuperAdmin:   true,
	}

	paymentService := &ServiceSummary{
		ID:   uuid.Must(uuid.NewV7()),
		Slug: "payment-service",
		Name: "Payment Service",
	}
	svcChecker.services[paymentService.ID] = paymentService

	dbSlug, _ := valueobject.NewSlug("payments-db")
	paymentsDBProd, _ := resource.NewResource(
		orgID,
		projID,
		prodEnvID,
		uuid.Must(uuid.NewV7()),
		dbSlug,
		"Payments DB Prod",
		resource.TypePostgres,
		map[string]any{"host": "prod.db"},
	)
	resChecker.resources[paymentsDBProd.ID] = paymentsDBProd

	_, err := svc.BindServiceToResource(ctx, adminActor, projID, BindServiceCmd{
		EnvironmentID: uatEnvID,
		ServiceID:     paymentService.ID,
		ResourceID:    paymentsDBProd.ID,
		AccessMode:    ModeReadWrite,
	})
	if err == nil {
		t.Fatalf("expected cross-environment binding error, got nil")
	}

	paymentsDBUAT, _ := resource.NewResource(
		orgID,
		projID,
		uatEnvID,
		uuid.Must(uuid.NewV7()),
		dbSlug,
		"Payments DB UAT",
		resource.TypePostgres,
		map[string]any{"host": "uat.db"},
	)
	resChecker.resources[paymentsDBUAT.ID] = paymentsDBUAT

	binding, err := svc.BindServiceToResource(ctx, adminActor, projID, BindServiceCmd{
		EnvironmentID: uatEnvID,
		ServiceID:     paymentService.ID,
		ResourceID:    paymentsDBUAT.ID,
		AccessMode:    ModeReadWrite,
	})
	if err != nil {
		t.Fatalf("unexpected error creating binding: %v", err)
	}
	if binding.AccessMode != ModeReadWrite {
		t.Fatalf("expected access mode READ_WRITE, got %s", binding.AccessMode)
	}

	err = svc.RemoveBinding(ctx, adminActor, projID, binding.ID)
	if err != nil {
		t.Fatalf("remove binding: %v", err)
	}
}

func TestBindingServiceDependenciesAndGraph(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.Must(uuid.NewV7())
	projID := uuid.Must(uuid.NewV7())
	uatEnvID := uuid.Must(uuid.NewV7())

	bindingRepo := newMemoryBindingRepo()
	svcChecker := newMemoryServiceChecker()
	resChecker := newMemoryResourceChecker()
	authorizer := access.NewAuthorizer(&mockAccessRepo{})
	txManager := database.NewTxManager(nil)

	svc := NewService(bindingRepo, svcChecker, resChecker, authorizer, nil, txManager)

	adminActor := access.Actor{
		UserID:         uuid.Must(uuid.NewV7()),
		OrganizationID: orgID,
		IsSuperAdmin:   true,
	}

	orderService := &ServiceSummary{
		ID:   uuid.Must(uuid.NewV7()),
		Slug: "order-service",
		Name: "Order Service",
	}
	svcChecker.services[orderService.ID] = orderService

	paymentService := &ServiceSummary{
		ID:   uuid.Must(uuid.NewV7()),
		Slug: "payment-service",
		Name: "Payment Service",
	}
	svcChecker.services[paymentService.ID] = paymentService

	_, err := svc.AddServiceDependency(ctx, adminActor, projID, AddDependencyCmd{
		EnvironmentID:   uatEnvID,
		CallerServiceID: orderService.ID,
		CalleeServiceID: orderService.ID,
		Protocol:        ProtocolHTTP,
	})
	if err == nil {
		t.Fatalf("expected error on self dependency, got nil")
	}

	dep, err := svc.AddServiceDependency(ctx, adminActor, projID, AddDependencyCmd{
		EnvironmentID:   uatEnvID,
		CallerServiceID: orderService.ID,
		CalleeServiceID: paymentService.ID,
		Protocol:        ProtocolGRPC,
	})
	if err != nil {
		t.Fatalf("add dependency: %v", err)
	}
	if dep.Protocol != ProtocolGRPC {
		t.Fatalf("expected protocol GRPC, got %s", dep.Protocol)
	}

	graph, err := svc.GetProjectTopologyGraph(ctx, adminActor, projID, uatEnvID)
	if err != nil {
		t.Fatalf("get graph: %v", err)
	}
	if len(graph.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(graph.Nodes))
	}
	if len(graph.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(graph.Edges))
	}
}
