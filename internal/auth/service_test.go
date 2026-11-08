package auth

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/access"
	"github.com/PathumSandeepa/iconfigly-api/internal/database"
	"github.com/PathumSandeepa/iconfigly-api/internal/organization"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/valueobject"
	"github.com/PathumSandeepa/iconfigly-api/internal/user"
	"github.com/google/uuid"
)

type memoryOrgRepo struct {
	orgs map[uuid.UUID]*organization.Organization
}

func newMemoryOrgRepo() *memoryOrgRepo {
	return &memoryOrgRepo{orgs: make(map[uuid.UUID]*organization.Organization)}
}

func (m *memoryOrgRepo) Count(ctx context.Context) (int64, error) {
	return int64(len(m.orgs)), nil
}

func (m *memoryOrgRepo) Create(ctx context.Context, org *organization.Organization) error {
	m.orgs[org.ID] = org
	return nil
}

type memoryUserRepo struct {
	users map[uuid.UUID]*user.User
}

func newMemoryUserRepo() *memoryUserRepo {
	return &memoryUserRepo{users: make(map[uuid.UUID]*user.User)}
}

func (m *memoryUserRepo) Create(ctx context.Context, u *user.User) error {
	m.users[u.ID] = u
	return nil
}

func (m *memoryUserRepo) GetByID(ctx context.Context, orgID, userID uuid.UUID) (*user.User, error) {
	u, ok := m.users[userID]
	if !ok || u.OrganizationID != orgID {
		return nil, apperror.NotFound("user")
	}
	return u, nil
}

func (m *memoryUserRepo) GetByEmail(ctx context.Context, email valueobject.Email) (*user.User, error) {
	for _, u := range m.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, apperror.NotFound("user")
}

type memoryAccessRepo struct {
	bindings []*access.RoleBinding
}

func (m *memoryAccessRepo) Create(ctx context.Context, binding *access.RoleBinding) error {
	m.bindings = append(m.bindings, binding)
	return nil
}

type memorySessionRepo struct {
	sessions map[uuid.UUID]*Session
}

func newMemorySessionRepo() *memorySessionRepo {
	return &memorySessionRepo{sessions: make(map[uuid.UUID]*Session)}
}

func (m *memorySessionRepo) Create(ctx context.Context, s *Session) error {
	m.sessions[s.ID] = s
	return nil
}

func (m *memorySessionRepo) GetByTokenHash(ctx context.Context, tokenHash []byte) (*Session, error) {
	for _, s := range m.sessions {
		if bytes.Equal(s.TokenHash, tokenHash) {
			return s, nil
		}
	}
	return nil, apperror.NotFound("session")
}

func (m *memorySessionRepo) Revoke(ctx context.Context, tokenHash []byte) error {
	for _, s := range m.sessions {
		if bytes.Equal(s.TokenHash, tokenHash) {
			now := time.Now().UTC()
			s.RevokedAt = &now
			return nil
		}
	}
	return nil
}

func (m *memorySessionRepo) RevokeAllForUser(ctx context.Context, orgID, userID uuid.UUID) error {
	now := time.Now().UTC()
	for _, s := range m.sessions {
		if s.OrganizationID == orgID && s.UserID == userID {
			s.RevokedAt = &now
		}
	}
	return nil
}

type memoryTokenRepo struct {
	tokens map[uuid.UUID]*PersonalAccessToken
}

func newMemoryTokenRepo() *memoryTokenRepo {
	return &memoryTokenRepo{tokens: make(map[uuid.UUID]*PersonalAccessToken)}
}

func (m *memoryTokenRepo) Create(ctx context.Context, t *PersonalAccessToken) error {
	m.tokens[t.ID] = t
	return nil
}

func (m *memoryTokenRepo) GetByTokenHash(ctx context.Context, tokenHash []byte) (*PersonalAccessToken, error) {
	for _, t := range m.tokens {
		if bytes.Equal(t.TokenHash, tokenHash) {
			return t, nil
		}
	}
	return nil, apperror.NotFound("personal access token")
}

func (m *memoryTokenRepo) ListByUser(ctx context.Context, orgID, userID uuid.UUID) ([]*PersonalAccessToken, error) {
	var list []*PersonalAccessToken
	for _, t := range m.tokens {
		if t.OrganizationID == orgID && t.UserID == userID {
			list = append(list, t)
		}
	}
	return list, nil
}

func (m *memoryTokenRepo) Revoke(ctx context.Context, orgID, userID, tokenID uuid.UUID) error {
	t, ok := m.tokens[tokenID]
	if !ok || t.OrganizationID != orgID || t.UserID != userID {
		return apperror.NotFound("personal access token")
	}
	now := time.Now().UTC()
	t.RevokedAt = &now
	return nil
}

func (m *memoryTokenRepo) UpdateLastUsed(ctx context.Context, tokenID uuid.UUID, lastUsed time.Time) error {
	if t, ok := m.tokens[tokenID]; ok {
		t.LastUsedAt = &lastUsed
	}
	return nil
}

type mockMembershipResolver struct{}

func (m *mockMembershipResolver) ResolveUserTeams(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, []uuid.UUID, error) {
	return []uuid.UUID{}, []uuid.UUID{}, nil
}

func setupTestAuthService() (*Service, *memoryOrgRepo, *memoryUserRepo, *memoryAccessRepo, *memorySessionRepo, *memoryTokenRepo) {
	orgRepo := newMemoryOrgRepo()
	userRepo := newMemoryUserRepo()
	accessRepo := &memoryAccessRepo{}
	sessionRepo := newMemorySessionRepo()
	tokenRepo := newMemoryTokenRepo()
	membership := &mockMembershipResolver{}
	txManager := database.NewTxManager(nil)

	svc := NewService(
		orgRepo,
		userRepo,
		accessRepo,
		sessionRepo,
		tokenRepo,
		membership,
		nil,
		txManager,
	)

	return svc, orgRepo, userRepo, accessRepo, sessionRepo, tokenRepo
}

func TestAuthServiceSetupAndInitialize(t *testing.T) {
	ctx := context.Background()
	svc, _, _, accessRepo, _, _ := setupTestAuthService()

	init, err := svc.GetSetupStatus(ctx)
	if err != nil {
		t.Fatalf("setup status: %v", err)
	}
	if init {
		t.Fatalf("expected uninitialized platform")
	}

	res, err := svc.InitializePlatform(ctx, SetupCmd{
		OrgSlug:       "acme-corp",
		OrgName:       "Acme Corporation",
		AdminEmail:    "admin@acme.com",
		AdminFullName: "Alice Admin",
		AdminPassword: "supersecretpassword",
	}, "127.0.0.1", "Go-Test")
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}

	if res.User.Email.String() != "admin@acme.com" {
		t.Fatalf("expected admin@acme.com, got %s", res.User.Email.String())
	}
	if !res.User.IsSuperAdmin {
		t.Fatalf("expected user to be super admin")
	}
	if res.SessionToken == "" || res.CSRFToken == "" {
		t.Fatalf("expected session token and csrf token")
	}
	if len(accessRepo.bindings) != 1 {
		t.Fatalf("expected 1 role binding, got %d", len(accessRepo.bindings))
	}
	if accessRepo.bindings[0].RoleName != access.RoleOrgAdmin {
		t.Fatalf("expected ORG_ADMIN role, got %s", accessRepo.bindings[0].RoleName)
	}

	init, err = svc.GetSetupStatus(ctx)
	if err != nil {
		t.Fatalf("setup status: %v", err)
	}
	if !init {
		t.Fatalf("expected initialized platform")
	}

	_, err = svc.InitializePlatform(ctx, SetupCmd{
		OrgSlug:       "second-corp",
		OrgName:       "Second Corporation",
		AdminEmail:    "other@second.com",
		AdminFullName: "Other Admin",
		AdminPassword: "supersecretpassword",
	}, "127.0.0.1", "Go-Test")
	if err == nil {
		t.Fatalf("expected conflict on second initialization, got nil")
	}
}

func TestAuthServiceLoginAndLogout(t *testing.T) {
	ctx := context.Background()
	svc, _, _, _, _, _ := setupTestAuthService()

	initRes, err := svc.InitializePlatform(ctx, SetupCmd{
		OrgSlug:       "acme-corp",
		OrgName:       "Acme Corporation",
		AdminEmail:    "admin@acme.com",
		AdminFullName: "Alice Admin",
		AdminPassword: "supersecretpassword",
	}, "127.0.0.1", "Go-Test")
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	loginRes, err := svc.Login(ctx, LoginCmd{
		Email:    "admin@acme.com",
		Password: "supersecretpassword",
	}, "127.0.0.1", "Go-Test")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if loginRes.User.ID != initRes.User.ID {
		t.Fatalf("expected user id %s, got %s", initRes.User.ID, loginRes.User.ID)
	}

	_, err = svc.Login(ctx, LoginCmd{
		Email:    "admin@acme.com",
		Password: "wrongpassword",
	}, "127.0.0.1", "Go-Test")
	if err == nil {
		t.Fatalf("expected error on wrong password, got nil")
	}

	_, err = svc.Login(ctx, LoginCmd{
		Email:    "nonexistent@acme.com",
		Password: "supersecretpassword",
	}, "127.0.0.1", "Go-Test")
	if err == nil {
		t.Fatalf("expected error on nonexistent user, got nil")
	}

	actor, err := svc.AuthenticateSession(ctx, loginRes.SessionToken, "127.0.0.1")
	if err != nil {
		t.Fatalf("authenticate session: %v", err)
	}
	if actor.UserID != initRes.User.ID {
		t.Fatalf("expected actor user id %s, got %s", initRes.User.ID, actor.UserID)
	}
	if actor.CSRFToken != loginRes.CSRFToken {
		t.Fatalf("expected csrf token %s, got %s", loginRes.CSRFToken, actor.CSRFToken)
	}

	err = svc.Logout(ctx, loginRes.SessionToken)
	if err != nil {
		t.Fatalf("logout: %v", err)
	}

	_, err = svc.AuthenticateSession(ctx, loginRes.SessionToken, "127.0.0.1")
	if err == nil {
		t.Fatalf("expected error after logout, got nil")
	}
}

func TestAuthServicePATLifecycle(t *testing.T) {
	ctx := context.Background()
	svc, _, _, _, _, _ := setupTestAuthService()

	initRes, err := svc.InitializePlatform(ctx, SetupCmd{
		OrgSlug:       "acme-corp",
		OrgName:       "Acme Corporation",
		AdminEmail:    "admin@acme.com",
		AdminFullName: "Alice Admin",
		AdminPassword: "supersecretpassword",
	}, "127.0.0.1", "Go-Test")
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	actor := access.Actor{
		UserID:         initRes.User.ID,
		OrganizationID: initRes.User.OrganizationID,
		Email:          initRes.User.Email.String(),
		IsSuperAdmin:   true,
		AuthMethod:     "SESSION",
	}

	pat, rawToken, err := svc.CreatePAT(ctx, actor, CreatePATCmd{
		Name:          "CI Pipeline Token",
		Scopes:        []string{"catalog.read", "catalog.write"},
		ExpiresInDays: 30,
	})
	if err != nil {
		t.Fatalf("create pat: %v", err)
	}
	if pat.TokenPrefix == "" || len(rawToken) < 20 {
		t.Fatalf("invalid raw token or prefix")
	}

	patActor, err := svc.AuthenticatePAT(ctx, rawToken, "127.0.0.1")
	if err != nil {
		t.Fatalf("authenticate pat: %v", err)
	}
	if patActor.UserID != actor.UserID {
		t.Fatalf("expected actor user id %s, got %s", actor.UserID, patActor.UserID)
	}
	if patActor.AuthMethod != "TOKEN" {
		t.Fatalf("expected auth method TOKEN, got %s", patActor.AuthMethod)
	}

	tokens, err := svc.ListPATs(ctx, actor)
	if err != nil {
		t.Fatalf("list pats: %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("expected 1 pat, got %d", len(tokens))
	}

	err = svc.RevokePAT(ctx, actor, pat.ID)
	if err != nil {
		t.Fatalf("revoke pat: %v", err)
	}

	_, err = svc.AuthenticatePAT(ctx, rawToken, "127.0.0.1")
	if err == nil {
		t.Fatalf("expected authentication failure after pat revocation, got nil")
	}
}

func TestAuthCSRFMiddleware(t *testing.T) {
	csrfMw := CSRFMiddleware()

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	actor := access.Actor{
		UserID:         uuid.Must(uuid.NewV7()),
		OrganizationID: uuid.Must(uuid.NewV7()),
		AuthMethod:     "SESSION",
		CSRFToken:      "valid-csrf-token-123456",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", nil)
	req = req.WithContext(access.WithActor(req.Context(), actor))
	rec := httptest.NewRecorder()
	csrfMw(nextHandler).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden without CSRF token, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/projects", nil)
	req = req.WithContext(access.WithActor(req.Context(), actor))
	req.Header.Set("X-CSRF-Token", "wrong-token")
	rec = httptest.NewRecorder()
	csrfMw(nextHandler).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden with wrong CSRF token, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/projects", nil)
	req = req.WithContext(access.WithActor(req.Context(), actor))
	req.Header.Set("X-CSRF-Token", "valid-csrf-token-123456")
	rec = httptest.NewRecorder()
	csrfMw(nextHandler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK with matching CSRF token, got %d", rec.Code)
	}

	patActor := access.Actor{
		UserID:         uuid.Must(uuid.NewV7()),
		OrganizationID: uuid.Must(uuid.NewV7()),
		AuthMethod:     "TOKEN",
	}
	req = httptest.NewRequest(http.MethodPost, "/api/v1/projects", nil)
	req = req.WithContext(access.WithActor(req.Context(), patActor))
	rec = httptest.NewRecorder()
	csrfMw(nextHandler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for PAT token without CSRF token, got %d", rec.Code)
	}
}
