package access

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/audit"
	"github.com/PathumSandeepa/iconfigly-api/internal/database"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type memoryAccessRepo struct {
	bindings []*RoleBinding
}

func newMemoryAccessRepo() *memoryAccessRepo {
	return &memoryAccessRepo{bindings: make([]*RoleBinding, 0)}
}

func (m *memoryAccessRepo) Create(ctx context.Context, binding *RoleBinding) error {
	m.bindings = append(m.bindings, binding)
	return nil
}

func (m *memoryAccessRepo) GetByID(ctx context.Context, orgID uuid.UUID, id uuid.UUID) (*RoleBinding, error) {
	for _, b := range m.bindings {
		if b.OrganizationID == orgID && b.ID == id {
			return b, nil
		}
	}
	return nil, apperror.NotFound("role binding")
}

func (m *memoryAccessRepo) Delete(ctx context.Context, orgID uuid.UUID, id uuid.UUID) error {
	for i, b := range m.bindings {
		if b.OrganizationID == orgID && b.ID == id {
			m.bindings = append(m.bindings[:i], m.bindings[i+1:]...)
			return nil
		}
	}
	return apperror.NotFound("role binding")
}

func (m *memoryAccessRepo) ListByProject(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID) ([]*RoleBinding, error) {
	var res []*RoleBinding
	for _, b := range m.bindings {
		if b.OrganizationID == orgID && b.ScopeID == projectID {
			res = append(res, b)
		}
	}
	return res, nil
}

func (m *memoryAccessRepo) ListByScope(ctx context.Context, orgID uuid.UUID, scopeType ScopeType, scopeID uuid.UUID) ([]*RoleBinding, error) {
	var res []*RoleBinding
	for _, b := range m.bindings {
		if b.OrganizationID == orgID && b.ScopeType == scopeType && b.ScopeID == scopeID {
			res = append(res, b)
		}
	}
	return res, nil
}

func (m *memoryAccessRepo) FindActiveBindingsForActor(
	ctx context.Context,
	orgID uuid.UUID,
	userID uuid.UUID,
	teamIDs []uuid.UUID,
	scopes []ScopeRef,
	now time.Time,
) ([]*RoleBinding, error) {
	scopeMap := make(map[string]bool)
	for _, sc := range scopes {
		scopeMap[string(sc.Type)+":"+sc.ID.String()] = true
	}

	teamMap := make(map[uuid.UUID]bool)
	for _, tid := range teamIDs {
		teamMap[tid] = true
	}

	var res []*RoleBinding
	for _, b := range m.bindings {
		if b.OrganizationID != orgID {
			continue
		}
		if b.IsExpired(now) {
			continue
		}

		key := string(b.ScopeType) + ":" + b.ScopeID.String()
		if !scopeMap[key] {
			continue
		}

		if b.SubjectType == SubjectUser && b.SubjectID == userID {
			res = append(res, b)
		} else if b.SubjectType == SubjectTeam && teamMap[b.SubjectID] {
			res = append(res, b)
		}
	}

	return res, nil
}

func (m *memoryAccessRepo) FindActiveBindingsForScopes(
	ctx context.Context,
	orgID uuid.UUID,
	scopes []ScopeRef,
	now time.Time,
) ([]*RoleBinding, error) {
	scopeMap := make(map[string]bool)
	for _, sc := range scopes {
		scopeMap[string(sc.Type)+":"+sc.ID.String()] = true
	}

	var res []*RoleBinding
	for _, b := range m.bindings {
		if b.OrganizationID != orgID {
			continue
		}
		if b.IsExpired(now) {
			continue
		}

		key := string(b.ScopeType) + ":" + b.ScopeID.String()
		if scopeMap[key] {
			res = append(res, b)
		}
	}

	return res, nil
}

func TestAuthorizerTableDrivenMatrix(t *testing.T) {
	orgID := uuid.Must(uuid.NewV7())
	otherOrgID := uuid.Must(uuid.NewV7())
	projectID := uuid.Must(uuid.NewV7())
	uatEnvID := uuid.Must(uuid.NewV7())
	prodEnvID := uuid.Must(uuid.NewV7())
	paymentServiceID := uuid.Must(uuid.NewV7())
	authServiceID := uuid.Must(uuid.NewV7())
	paymentsDBID := uuid.Must(uuid.NewV7())

	orgScope := NewOrgScope(orgID)
	projectScope := NewProjectScope(orgID, projectID)
	uatScope := NewEnvironmentScope(orgID, projectID, uatEnvID)
	prodScope := NewEnvironmentScope(orgID, projectID, prodEnvID)
	paymentServiceScope := NewServiceScope(orgID, projectID, uatEnvID, paymentServiceID)
	authServiceScope := NewServiceScope(orgID, projectID, uatEnvID, authServiceID)
	paymentsDBScope := NewResourceScope(orgID, projectID, uatEnvID, paymentsDBID)

	userID := uuid.Must(uuid.NewV7())
	teamID := uuid.Must(uuid.NewV7())
	otherTeamID := uuid.Must(uuid.NewV7())

	now := time.Now().UTC()
	past := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)

	tests := []struct {
		name        string
		actor       Actor
		bindings    []*RoleBinding
		verb        PermissionVerb
		targetScope Scope
		wantAllowed bool
	}{
		{
			name:        "super admin bypass org.manage on Org scope",
			actor:       Actor{UserID: userID, OrganizationID: orgID, IsSuperAdmin: true},
			verb:        VerbOrgManage,
			targetScope: orgScope,
			wantAllowed: true,
		},
		{
			name:        "super admin bypass catalog.write on Resource scope",
			actor:       Actor{UserID: userID, OrganizationID: orgID, IsSuperAdmin: true},
			verb:        VerbCatalogWrite,
			targetScope: paymentsDBScope,
			wantAllowed: true,
		},
		{
			name:        "super admin bypass access.manage on Service scope",
			actor:       Actor{UserID: userID, OrganizationID: orgID, IsSuperAdmin: true},
			verb:        VerbAccessManage,
			targetScope: paymentServiceScope,
			wantAllowed: true,
		},
		{
			name:        "ungranted user denied catalog.read by default",
			actor:       Actor{UserID: userID, OrganizationID: orgID},
			verb:        VerbCatalogRead,
			targetScope: paymentServiceScope,
			wantAllowed: false,
		},
		{
			name:        "ungranted user denied org.manage by default",
			actor:       Actor{UserID: userID, OrganizationID: orgID},
			verb:        VerbOrgManage,
			targetScope: orgScope,
			wantAllowed: false,
		},
		{
			name:  "cross tenant isolation denied even if granted in other org",
			actor: Actor{UserID: userID, OrganizationID: otherOrgID},
			bindings: []*RoleBinding{
				{
					OrganizationID: otherOrgID,
					SubjectType:    SubjectUser,
					SubjectID:      userID,
					RoleName:       RoleOrgAdmin,
					ScopeType:      ScopeOrg,
					ScopeID:        otherOrgID,
				},
			},
			verb:        VerbCatalogRead,
			targetScope: projectScope,
			wantAllowed: false,
		},
		{
			name:  "ORG_ADMIN at Org scope allowed org.manage",
			actor: Actor{UserID: userID, OrganizationID: orgID},
			bindings: []*RoleBinding{
				{OrganizationID: orgID, SubjectType: SubjectUser, SubjectID: userID, RoleName: RoleOrgAdmin, ScopeType: ScopeOrg, ScopeID: orgID},
			},
			verb:        VerbOrgManage,
			targetScope: orgScope,
			wantAllowed: true,
		},
		{
			name:  "ORG_ADMIN at Org scope inherited down to Project scope catalog.read",
			actor: Actor{UserID: userID, OrganizationID: orgID},
			bindings: []*RoleBinding{
				{OrganizationID: orgID, SubjectType: SubjectUser, SubjectID: userID, RoleName: RoleOrgAdmin, ScopeType: ScopeOrg, ScopeID: orgID},
			},
			verb:        VerbCatalogRead,
			targetScope: projectScope,
			wantAllowed: true,
		},
		{
			name:  "ORG_ADMIN at Org scope inherited down to Environment scope catalog.write",
			actor: Actor{UserID: userID, OrganizationID: orgID},
			bindings: []*RoleBinding{
				{OrganizationID: orgID, SubjectType: SubjectUser, SubjectID: userID, RoleName: RoleOrgAdmin, ScopeType: ScopeOrg, ScopeID: orgID},
			},
			verb:        VerbCatalogWrite,
			targetScope: uatScope,
			wantAllowed: true,
		},
		{
			name:  "ORG_ADMIN at Org scope inherited down to Service scope catalog.write",
			actor: Actor{UserID: userID, OrganizationID: orgID},
			bindings: []*RoleBinding{
				{OrganizationID: orgID, SubjectType: SubjectUser, SubjectID: userID, RoleName: RoleOrgAdmin, ScopeType: ScopeOrg, ScopeID: orgID},
			},
			verb:        VerbCatalogWrite,
			targetScope: paymentServiceScope,
			wantAllowed: true,
		},
		{
			name:  "ORG_ADMIN at Org scope inherited down to Resource scope catalog.write",
			actor: Actor{UserID: userID, OrganizationID: orgID},
			bindings: []*RoleBinding{
				{OrganizationID: orgID, SubjectType: SubjectUser, SubjectID: userID, RoleName: RoleOrgAdmin, ScopeType: ScopeOrg, ScopeID: orgID},
			},
			verb:        VerbCatalogWrite,
			targetScope: paymentsDBScope,
			wantAllowed: true,
		},
		{
			name:  "ORG_ADMIN at Org scope allowed access.manage",
			actor: Actor{UserID: userID, OrganizationID: orgID},
			bindings: []*RoleBinding{
				{OrganizationID: orgID, SubjectType: SubjectUser, SubjectID: userID, RoleName: RoleOrgAdmin, ScopeType: ScopeOrg, ScopeID: orgID},
			},
			verb:        VerbAccessManage,
			targetScope: paymentServiceScope,
			wantAllowed: true,
		},
		{
			name:  "ORG_ADMIN at Org scope allowed audit.read",
			actor: Actor{UserID: userID, OrganizationID: orgID},
			bindings: []*RoleBinding{
				{OrganizationID: orgID, SubjectType: SubjectUser, SubjectID: userID, RoleName: RoleOrgAdmin, ScopeType: ScopeOrg, ScopeID: orgID},
			},
			verb:        VerbAuditRead,
			targetScope: orgScope,
			wantAllowed: true,
		},
		{
			name:  "ENV_ADMIN at UAT scope allowed catalog.read in UAT",
			actor: Actor{UserID: userID, OrganizationID: orgID},
			bindings: []*RoleBinding{
				{OrganizationID: orgID, SubjectType: SubjectUser, SubjectID: userID, RoleName: RoleEnvAdmin, ScopeType: ScopeEnvironment, ScopeID: uatEnvID},
			},
			verb:        VerbCatalogRead,
			targetScope: uatScope,
			wantAllowed: true,
		},
		{
			name:  "ENV_ADMIN at UAT scope allowed catalog.write in UAT",
			actor: Actor{UserID: userID, OrganizationID: orgID},
			bindings: []*RoleBinding{
				{OrganizationID: orgID, SubjectType: SubjectUser, SubjectID: userID, RoleName: RoleEnvAdmin, ScopeType: ScopeEnvironment, ScopeID: uatEnvID},
			},
			verb:        VerbCatalogWrite,
			targetScope: uatScope,
			wantAllowed: true,
		},
		{
			name:  "ENV_ADMIN at UAT scope denied catalog.read in PROD",
			actor: Actor{UserID: userID, OrganizationID: orgID},
			bindings: []*RoleBinding{
				{OrganizationID: orgID, SubjectType: SubjectUser, SubjectID: userID, RoleName: RoleEnvAdmin, ScopeType: ScopeEnvironment, ScopeID: uatEnvID},
			},
			verb:        VerbCatalogRead,
			targetScope: prodScope,
			wantAllowed: false,
		},
		{
			name:  "ENV_ADMIN at UAT scope denied catalog.write in PROD",
			actor: Actor{UserID: userID, OrganizationID: orgID},
			bindings: []*RoleBinding{
				{OrganizationID: orgID, SubjectType: SubjectUser, SubjectID: userID, RoleName: RoleEnvAdmin, ScopeType: ScopeEnvironment, ScopeID: uatEnvID},
			},
			verb:        VerbCatalogWrite,
			targetScope: prodScope,
			wantAllowed: false,
		},
		{
			name:  "ENV_ADMIN at UAT scope allowed access.manage at Environment scope",
			actor: Actor{UserID: userID, OrganizationID: orgID},
			bindings: []*RoleBinding{
				{OrganizationID: orgID, SubjectType: SubjectUser, SubjectID: userID, RoleName: RoleEnvAdmin, ScopeType: ScopeEnvironment, ScopeID: uatEnvID},
			},
			verb:        VerbAccessManage,
			targetScope: uatScope,
			wantAllowed: true,
		},
		{
			name:  "ENV_ADMIN at UAT scope allowed access.manage at Resource scope",
			actor: Actor{UserID: userID, OrganizationID: orgID},
			bindings: []*RoleBinding{
				{OrganizationID: orgID, SubjectType: SubjectUser, SubjectID: userID, RoleName: RoleEnvAdmin, ScopeType: ScopeEnvironment, ScopeID: uatEnvID},
			},
			verb:        VerbAccessManage,
			targetScope: paymentsDBScope,
			wantAllowed: true,
		},
		{
			name:  "ENV_ADMIN at UAT scope denied access.manage at Service scope",
			actor: Actor{UserID: userID, OrganizationID: orgID},
			bindings: []*RoleBinding{
				{OrganizationID: orgID, SubjectType: SubjectUser, SubjectID: userID, RoleName: RoleEnvAdmin, ScopeType: ScopeEnvironment, ScopeID: uatEnvID},
			},
			verb:        VerbAccessManage,
			targetScope: paymentServiceScope,
			wantAllowed: false,
		},
		{
			name:  "ENV_ADMIN at UAT scope denied org.manage",
			actor: Actor{UserID: userID, OrganizationID: orgID},
			bindings: []*RoleBinding{
				{OrganizationID: orgID, SubjectType: SubjectUser, SubjectID: userID, RoleName: RoleEnvAdmin, ScopeType: ScopeEnvironment, ScopeID: uatEnvID},
			},
			verb:        VerbOrgManage,
			targetScope: orgScope,
			wantAllowed: false,
		},
		{
			name:  "SERVICE_LEAD at Service scope allowed catalog.read on payment-service",
			actor: Actor{UserID: userID, OrganizationID: orgID},
			bindings: []*RoleBinding{
				{OrganizationID: orgID, SubjectType: SubjectUser, SubjectID: userID, RoleName: RoleServiceLead, ScopeType: ScopeService, ScopeID: paymentServiceID},
			},
			verb:        VerbCatalogRead,
			targetScope: paymentServiceScope,
			wantAllowed: true,
		},
		{
			name:  "SERVICE_LEAD at Service scope allowed catalog.write on payment-service",
			actor: Actor{UserID: userID, OrganizationID: orgID},
			bindings: []*RoleBinding{
				{OrganizationID: orgID, SubjectType: SubjectUser, SubjectID: userID, RoleName: RoleServiceLead, ScopeType: ScopeService, ScopeID: paymentServiceID},
			},
			verb:        VerbCatalogWrite,
			targetScope: paymentServiceScope,
			wantAllowed: true,
		},
		{
			name:  "SERVICE_LEAD at Service scope allowed access.manage on payment-service",
			actor: Actor{UserID: userID, OrganizationID: orgID},
			bindings: []*RoleBinding{
				{OrganizationID: orgID, SubjectType: SubjectUser, SubjectID: userID, RoleName: RoleServiceLead, ScopeType: ScopeService, ScopeID: paymentServiceID},
			},
			verb:        VerbAccessManage,
			targetScope: paymentServiceScope,
			wantAllowed: true,
		},
		{
			name:  "SERVICE_LEAD on payment-service denied access.manage on Environment scope",
			actor: Actor{UserID: userID, OrganizationID: orgID},
			bindings: []*RoleBinding{
				{OrganizationID: orgID, SubjectType: SubjectUser, SubjectID: userID, RoleName: RoleServiceLead, ScopeType: ScopeService, ScopeID: paymentServiceID},
			},
			verb:        VerbAccessManage,
			targetScope: uatScope,
			wantAllowed: false,
		},
		{
			name:  "SERVICE_LEAD on payment-service denied catalog.write on auth-service",
			actor: Actor{UserID: userID, OrganizationID: orgID},
			bindings: []*RoleBinding{
				{OrganizationID: orgID, SubjectType: SubjectUser, SubjectID: userID, RoleName: RoleServiceLead, ScopeType: ScopeService, ScopeID: paymentServiceID},
			},
			verb:        VerbCatalogWrite,
			targetScope: authServiceScope,
			wantAllowed: false,
		},
		{
			name:  "DEVELOPER at Project scope allowed catalog.read on Service",
			actor: Actor{UserID: userID, OrganizationID: orgID},
			bindings: []*RoleBinding{
				{OrganizationID: orgID, SubjectType: SubjectUser, SubjectID: userID, RoleName: RoleDeveloper, ScopeType: ScopeProject, ScopeID: projectID},
			},
			verb:        VerbCatalogRead,
			targetScope: paymentServiceScope,
			wantAllowed: true,
		},
		{
			name:  "DEVELOPER at Project scope denied catalog.write on Service",
			actor: Actor{UserID: userID, OrganizationID: orgID},
			bindings: []*RoleBinding{
				{OrganizationID: orgID, SubjectType: SubjectUser, SubjectID: userID, RoleName: RoleDeveloper, ScopeType: ScopeProject, ScopeID: projectID},
			},
			verb:        VerbCatalogWrite,
			targetScope: paymentServiceScope,
			wantAllowed: false,
		},
		{
			name:  "DEVELOPER at Project scope denied access.manage",
			actor: Actor{UserID: userID, OrganizationID: orgID},
			bindings: []*RoleBinding{
				{OrganizationID: orgID, SubjectType: SubjectUser, SubjectID: userID, RoleName: RoleDeveloper, ScopeType: ScopeProject, ScopeID: projectID},
			},
			verb:        VerbAccessManage,
			targetScope: projectScope,
			wantAllowed: false,
		},
		{
			name:  "DEVELOPER at Project scope denied audit.read",
			actor: Actor{UserID: userID, OrganizationID: orgID},
			bindings: []*RoleBinding{
				{OrganizationID: orgID, SubjectType: SubjectUser, SubjectID: userID, RoleName: RoleDeveloper, ScopeType: ScopeProject, ScopeID: projectID},
			},
			verb:        VerbAuditRead,
			targetScope: projectScope,
			wantAllowed: false,
		},
		{
			name:  "VIEWER at Org scope allowed catalog.read on Resource",
			actor: Actor{UserID: userID, OrganizationID: orgID},
			bindings: []*RoleBinding{
				{OrganizationID: orgID, SubjectType: SubjectUser, SubjectID: userID, RoleName: RoleViewer, ScopeType: ScopeOrg, ScopeID: orgID},
			},
			verb:        VerbCatalogRead,
			targetScope: paymentsDBScope,
			wantAllowed: true,
		},
		{
			name:  "VIEWER at Org scope allowed audit.read on Org",
			actor: Actor{UserID: userID, OrganizationID: orgID},
			bindings: []*RoleBinding{
				{OrganizationID: orgID, SubjectType: SubjectUser, SubjectID: userID, RoleName: RoleViewer, ScopeType: ScopeOrg, ScopeID: orgID},
			},
			verb:        VerbAuditRead,
			targetScope: orgScope,
			wantAllowed: true,
		},
		{
			name:  "VIEWER at Org scope denied catalog.write",
			actor: Actor{UserID: userID, OrganizationID: orgID},
			bindings: []*RoleBinding{
				{OrganizationID: orgID, SubjectType: SubjectUser, SubjectID: userID, RoleName: RoleViewer, ScopeType: ScopeOrg, ScopeID: orgID},
			},
			verb:        VerbCatalogWrite,
			targetScope: paymentServiceScope,
			wantAllowed: false,
		},
		{
			name:  "VIEWER at Org scope denied access.manage",
			actor: Actor{UserID: userID, OrganizationID: orgID},
			bindings: []*RoleBinding{
				{OrganizationID: orgID, SubjectType: SubjectUser, SubjectID: userID, RoleName: RoleViewer, ScopeType: ScopeOrg, ScopeID: orgID},
			},
			verb:        VerbAccessManage,
			targetScope: orgScope,
			wantAllowed: false,
		},
		{
			name:  "team member inherits team DEVELOPER grant for catalog.read",
			actor: Actor{UserID: userID, OrganizationID: orgID, TeamIDs: []uuid.UUID{teamID}},
			bindings: []*RoleBinding{
				{OrganizationID: orgID, SubjectType: SubjectTeam, SubjectID: teamID, RoleName: RoleDeveloper, ScopeType: ScopeProject, ScopeID: projectID},
			},
			verb:        VerbCatalogRead,
			targetScope: paymentServiceScope,
			wantAllowed: true,
		},
		{
			name:  "non team member denied team grant",
			actor: Actor{UserID: userID, OrganizationID: orgID, TeamIDs: []uuid.UUID{otherTeamID}},
			bindings: []*RoleBinding{
				{OrganizationID: orgID, SubjectType: SubjectTeam, SubjectID: teamID, RoleName: RoleDeveloper, ScopeType: ScopeProject, ScopeID: projectID},
			},
			verb:        VerbCatalogRead,
			targetScope: paymentServiceScope,
			wantAllowed: false,
		},
		{
			name:  "expired role binding automatically denied",
			actor: Actor{UserID: userID, OrganizationID: orgID},
			bindings: []*RoleBinding{
				{
					OrganizationID: orgID,
					SubjectType:    SubjectUser,
					SubjectID:      userID,
					RoleName:       RoleOrgAdmin,
					ScopeType:      ScopeOrg,
					ScopeID:        orgID,
					ExpiresAt:      &past,
				},
			},
			verb:        VerbCatalogRead,
			targetScope: projectScope,
			wantAllowed: false,
		},
		{
			name:  "active time-boxed grant allowed before expiry",
			actor: Actor{UserID: userID, OrganizationID: orgID},
			bindings: []*RoleBinding{
				{
					OrganizationID: orgID,
					SubjectType:    SubjectUser,
					SubjectID:      userID,
					RoleName:       RoleOrgAdmin,
					ScopeType:      ScopeOrg,
					ScopeID:        orgID,
					ExpiresAt:      &future,
				},
			},
			verb:        VerbCatalogRead,
			targetScope: projectScope,
			wantAllowed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMemoryAccessRepo()
			repo.bindings = tt.bindings
			authorizer := NewAuthorizer(repo)

			got, err := authorizer.Can(context.Background(), tt.actor, tt.verb, tt.targetScope)
			if err != nil {
				t.Fatalf("authorizer.Can returned unexpected error: %v", err)
			}
			if got != tt.wantAllowed {
				t.Fatalf("expected allowed=%v, got=%v", tt.wantAllowed, got)
			}
		})
	}
}

func TestRequireReturnsForbidden(t *testing.T) {
	orgID := uuid.Must(uuid.NewV7())
	userID := uuid.Must(uuid.NewV7())
	projectID := uuid.Must(uuid.NewV7())

	repo := newMemoryAccessRepo()
	authorizer := NewAuthorizer(repo)

	actor := Actor{UserID: userID, OrganizationID: orgID}
	scope := NewProjectScope(orgID, projectID)

	err := authorizer.Require(context.Background(), actor, VerbCatalogWrite, scope)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	appErr, ok := apperror.From(err)
	if !ok {
		t.Fatalf("expected apperror, got %v", err)
	}
	if appErr.Code != apperror.CodeForbidden {
		t.Fatalf("expected FORBIDDEN code, got %s", appErr.Code)
	}
}

func setupAccessIntegrationDB(t *testing.T) (*pgxpool.Pool, *database.TxManager, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	pool, err := database.NewPostgres(dbURL)
	if err != nil {
		t.Fatalf("connect to database: %v", err)
	}
	t.Cleanup(pool.Close)

	orgID := uuid.Must(uuid.NewV7())
	slug := "acc-" + uuid.NewString()[:8] + "-" + orgID.String()[24:]
	_, err = pool.Exec(context.Background(),
		"INSERT INTO organizations (id, slug, name) VALUES ($1, $2, $3)",
		orgID, slug, "Access Test Org",
	)
	if err != nil {
		t.Fatalf("insert test org: %v", err)
	}

	projectID := uuid.Must(uuid.NewV7())
	_, err = pool.Exec(context.Background(),
		"INSERT INTO projects (id, organization_id, slug, name) VALUES ($1, $2, $3, $4)",
		projectID, orgID, "proj-"+projectID.String()[:8], "Test Project",
	)
	if err != nil {
		t.Fatalf("insert test project: %v", err)
	}

	envID := uuid.Must(uuid.NewV7())
	_, err = pool.Exec(context.Background(),
		"INSERT INTO environments (id, organization_id, project_id, slug, name, tier_order) VALUES ($1, $2, $3, $4, $5, 1)",
		envID, orgID, projectID, "uat", "UAT",
	)
	if err != nil {
		t.Fatalf("insert test env: %v", err)
	}

	adminUserID := uuid.Must(uuid.NewV7())
	_, err = pool.Exec(context.Background(),
		"INSERT INTO users (id, organization_id, email, full_name, password_hash, is_super_admin) VALUES ($1, $2, $3, $4, $5, true)",
		adminUserID, orgID, "admin@"+slug+".com", "Admin User", "dummyhash",
	)
	if err != nil {
		t.Fatalf("insert test admin user: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", adminUserID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM environments WHERE id = $1", envID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM projects WHERE id = $1", projectID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM organizations WHERE id = $1", orgID)
	})

	txManager := database.NewTxManager(pool)
	return pool, txManager, orgID, projectID, envID, adminUserID
}

func TestAccessIntegrationPostgresRepoAndService(t *testing.T) {
	_, txManager, orgID, projectID, envID, adminUserID := setupAccessIntegrationDB(t)

	auditRepo := audit.NewPostgresRepository(txManager)
	auditSvc := audit.NewService(auditRepo, txManager)

	accessRepo := NewPostgresRepository(txManager)
	authorizer := NewAuthorizer(accessRepo)
	svc := NewService(accessRepo, authorizer, auditSvc, txManager)
	ctx := context.Background()

	adminActor := Actor{
		UserID:         adminUserID,
		OrganizationID: orgID,
		IsSuperAdmin:   true,
	}

	targetUser := uuid.Must(uuid.NewV7())
	targetTeam := uuid.Must(uuid.NewV7())

	projectScope := NewProjectScope(orgID, projectID)
	envScope := NewEnvironmentScope(orgID, projectID, envID)

	binding1, err := svc.GrantRole(ctx, adminActor, projectScope, GrantRoleCmd{
		SubjectType: SubjectUser,
		SubjectID:   targetUser,
		RoleName:    RoleDeveloper,
		ScopeType:   ScopeProject,
		ScopeID:     projectID,
	})
	if err != nil {
		t.Fatalf("grant role 1: %v", err)
	}

	binding2, err := svc.GrantRole(ctx, adminActor, envScope, GrantRoleCmd{
		SubjectType: SubjectTeam,
		SubjectID:   targetTeam,
		RoleName:    RoleEnvAdmin,
		ScopeType:   ScopeEnvironment,
		ScopeID:     envID,
	})
	if err != nil {
		t.Fatalf("grant role 2: %v", err)
	}

	devActor := Actor{
		UserID:         targetUser,
		OrganizationID: orgID,
	}
	allowed, err := authorizer.Can(ctx, devActor, VerbCatalogRead, envScope)
	if err != nil {
		t.Fatalf("authorizer.Can: %v", err)
	}
	if !allowed {
		t.Fatal("expected devActor to inherit catalog.read from Project scope to Environment scope")
	}

	writeAllowed, err := authorizer.Can(ctx, devActor, VerbCatalogWrite, envScope)
	if err != nil {
		t.Fatalf("authorizer.Can write: %v", err)
	}
	if writeAllowed {
		t.Fatal("DEVELOPER must not have catalog.write")
	}

	teamMemberActor := Actor{
		UserID:         uuid.Must(uuid.NewV7()),
		OrganizationID: orgID,
		TeamIDs:        []uuid.UUID{targetTeam},
	}
	teamAllowed, err := authorizer.Can(ctx, teamMemberActor, VerbCatalogWrite, envScope)
	if err != nil {
		t.Fatalf("authorizer.Can team member: %v", err)
	}
	if !teamAllowed {
		t.Fatal("expected team member to inherit ENV_ADMIN catalog.write in UAT environment")
	}

	inspection, err := svc.InspectTargetAccess(ctx, adminActor, envScope)
	if err != nil {
		t.Fatalf("inspect target access: %v", err)
	}

	if len(inspection.UserGrants) != 1 {
		t.Fatalf("expected 1 user grant, got %d", len(inspection.UserGrants))
	}
	if inspection.UserGrants[0].Source != "Inherited from PROJECT" {
		t.Fatalf("expected source 'Inherited from PROJECT', got %s", inspection.UserGrants[0].Source)
	}

	if len(inspection.TeamGrants) != 1 {
		t.Fatalf("expected 1 team grant, got %d", len(inspection.TeamGrants))
	}
	if inspection.TeamGrants[0].Source != "Direct" {
		t.Fatalf("expected source 'Direct', got %s", inspection.TeamGrants[0].Source)
	}

	err = svc.RevokeRole(ctx, adminActor, envScope, binding2.ID)
	if err != nil {
		t.Fatalf("revoke role: %v", err)
	}

	teamAllowedAfterRevoke, err := authorizer.Can(ctx, teamMemberActor, VerbCatalogWrite, envScope)
	if err != nil {
		t.Fatalf("authorizer.Can after revoke: %v", err)
	}
	if teamAllowedAfterRevoke {
		t.Fatal("team member must not have access after binding revoked")
	}

	verifyResult, err := auditSvc.VerifyHashChain(ctx, orgID)
	if err != nil {
		t.Fatalf("verify audit chain: %v", err)
	}
	if !verifyResult.Valid || verifyResult.TotalCount != 3 {
		t.Fatalf("expected 3 valid audit log entries (2 grants + 1 revoke), got %+v", verifyResult)
	}

	_ = binding1
}
