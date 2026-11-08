package user

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

type memoryUserRepo struct {
	users map[uuid.UUID]*User
}

func newMemoryUserRepo() *memoryUserRepo {
	return &memoryUserRepo{users: make(map[uuid.UUID]*User)}
}

func (m *memoryUserRepo) Create(ctx context.Context, u *User) error {
	for _, existing := range m.users {
		if existing.Email == u.Email {
			return apperror.Conflict("email already registered")
		}
	}
	m.users[u.ID] = u
	return nil
}

func (m *memoryUserRepo) GetByID(ctx context.Context, orgID, userID uuid.UUID) (*User, error) {
	u, ok := m.users[userID]
	if !ok || u.OrganizationID != orgID {
		return nil, apperror.NotFound("user")
	}
	return u, nil
}

func (m *memoryUserRepo) GetByEmail(ctx context.Context, email valueobject.Email) (*User, error) {
	for _, u := range m.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, apperror.NotFound("user")
}

func (m *memoryUserRepo) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]*User, error) {
	var list []*User
	for _, u := range m.users {
		if u.OrganizationID == orgID {
			list = append(list, u)
		}
	}
	return list, nil
}

func (m *memoryUserRepo) UpdateStatus(ctx context.Context, orgID, userID uuid.UUID, status UserStatus) error {
	u, ok := m.users[userID]
	if !ok || u.OrganizationID != orgID {
		return apperror.NotFound("user")
	}
	u.Status = status
	return nil
}

func (m *memoryUserRepo) CountActiveSuperAdmins(ctx context.Context, orgID uuid.UUID) (int64, error) {
	var count int64
	for _, u := range m.users {
		if u.OrganizationID == orgID && u.IsSuperAdmin && u.Status == StatusActive {
			count++
		}
	}
	return count, nil
}

type memorySessionRevoker struct {
	revoked []uuid.UUID
}

func (r *memorySessionRevoker) RevokeAllForUser(ctx context.Context, orgID, userID uuid.UUID) error {
	r.revoked = append(r.revoked, userID)
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

func TestUserServiceCreateAndList(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.Must(uuid.NewV7())
	repo := newMemoryUserRepo()
	txManager := database.NewTxManager(nil)
	authorizer := access.NewAuthorizer(&mockAccessRepo{})
	revoker := &memorySessionRevoker{}
	svc := NewService(repo, authorizer, nil, nil, revoker, txManager)

	adminActor := access.Actor{
		UserID:         uuid.Must(uuid.NewV7()),
		OrganizationID: orgID,
		IsSuperAdmin:   true,
	}

	u, err := svc.CreateUser(ctx, adminActor, CreateUserCmd{
		Email:        "bob@acme.com",
		FullName:     "Bob Smith",
		Password:     "securepassword123",
		IsSuperAdmin: false,
	})
	if err != nil {
		t.Fatalf("unexpected error creating user: %v", err)
	}

	if u.Email.String() != "bob@acme.com" {
		t.Fatalf("expected email bob@acme.com, got %s", u.Email.String())
	}
	if u.Status != StatusActive {
		t.Fatalf("expected status ACTIVE, got %s", u.Status)
	}

	_, err = svc.CreateUser(ctx, adminActor, CreateUserCmd{
		Email:        "bob@acme.com",
		FullName:     "Bob Duplicate",
		Password:     "securepassword123",
		IsSuperAdmin: false,
	})
	if err == nil {
		t.Fatalf("expected conflict error for duplicate email, got nil")
	}

	users, err := svc.ListUsers(ctx, adminActor)
	if err != nil {
		t.Fatalf("unexpected error listing users: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
}

func TestUserServiceChangeStatus(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.Must(uuid.NewV7())
	repo := newMemoryUserRepo()
	txManager := database.NewTxManager(nil)
	authorizer := access.NewAuthorizer(&mockAccessRepo{})
	revoker := &memorySessionRevoker{}
	svc := NewService(repo, authorizer, nil, nil, revoker, txManager)

	adminActor := access.Actor{
		UserID:         uuid.Must(uuid.NewV7()),
		OrganizationID: orgID,
		IsSuperAdmin:   true,
	}

	superAdmin, err := svc.CreateUser(ctx, adminActor, CreateUserCmd{
		Email:        "admin@acme.com",
		FullName:     "Alice Admin",
		Password:     "securepassword123",
		IsSuperAdmin: true,
	})
	if err != nil {
		t.Fatalf("create superadmin: %v", err)
	}

	devUser, err := svc.CreateUser(ctx, adminActor, CreateUserCmd{
		Email:        "dev@acme.com",
		FullName:     "Dave Dev",
		Password:     "securepassword123",
		IsSuperAdmin: false,
	})
	if err != nil {
		t.Fatalf("create dev: %v", err)
	}

	err = svc.ChangeStatus(ctx, adminActor, superAdmin.ID, StatusDisabled)
	if err == nil {
		t.Fatalf("expected error disabling the only super admin, got nil")
	}

	err = svc.ChangeStatus(ctx, adminActor, devUser.ID, StatusDisabled)
	if err != nil {
		t.Fatalf("unexpected error disabling normal user: %v", err)
	}

	updated, err := repo.GetByID(ctx, orgID, devUser.ID)
	if err != nil {
		t.Fatalf("get dev user: %v", err)
	}
	if updated.Status != StatusDisabled {
		t.Fatalf("expected status DISABLED, got %s", updated.Status)
	}
	if len(revoker.revoked) != 1 || revoker.revoked[0] != devUser.ID {
		t.Fatalf("expected session revocation for dev user, got %v", revoker.revoked)
	}
}
