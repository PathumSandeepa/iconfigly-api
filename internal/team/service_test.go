package team

import (
	"context"
	"testing"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/access"
	"github.com/PathumSandeepa/iconfigly-api/internal/database"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/valueobject"
	"github.com/PathumSandeepa/iconfigly-api/internal/user"
	"github.com/google/uuid"
)

type memoryTeamRepo struct {
	teams   map[uuid.UUID]*Team
	members map[uuid.UUID][]*TeamMember
}

func newMemoryTeamRepo() *memoryTeamRepo {
	return &memoryTeamRepo{
		teams:   make(map[uuid.UUID]*Team),
		members: make(map[uuid.UUID][]*TeamMember),
	}
}

func (m *memoryTeamRepo) Create(ctx context.Context, t *Team) error {
	for _, existing := range m.teams {
		if existing.OrganizationID == t.OrganizationID && existing.Slug == t.Slug {
			return apperror.Conflict("team with this slug already exists in organization")
		}
	}
	m.teams[t.ID] = t
	return nil
}

func (m *memoryTeamRepo) GetByID(ctx context.Context, orgID, teamID uuid.UUID) (*Team, error) {
	t, ok := m.teams[teamID]
	if !ok || t.OrganizationID != orgID {
		return nil, apperror.NotFound("team")
	}
	return t, nil
}

func (m *memoryTeamRepo) GetBySlug(ctx context.Context, orgID uuid.UUID, slug valueobject.Slug) (*Team, error) {
	for _, t := range m.teams {
		if t.OrganizationID == orgID && t.Slug == slug {
			return t, nil
		}
	}
	return nil, apperror.NotFound("team")
}

func (m *memoryTeamRepo) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]*TeamWithMemberCount, error) {
	var list []*TeamWithMemberCount
	for _, t := range m.teams {
		if t.OrganizationID == orgID {
			count := len(m.members[t.ID])
			list = append(list, &TeamWithMemberCount{
				ID:             t.ID,
				OrganizationID: t.OrganizationID,
				Slug:           t.Slug,
				Name:           t.Name,
				Description:    t.Description,
				MemberCount:    count,
				CreatedAt:      t.CreatedAt,
				UpdatedAt:      t.UpdatedAt,
			})
		}
	}
	return list, nil
}

func (m *memoryTeamRepo) AddMember(ctx context.Context, member *TeamMember) error {
	existingList := m.members[member.TeamID]
	for _, ex := range existingList {
		if ex.UserID == member.UserID {
			return apperror.Conflict("user is already a member of this team")
		}
	}
	m.members[member.TeamID] = append(existingList, member)
	return nil
}

func (m *memoryTeamRepo) RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error {
	existingList := m.members[teamID]
	for i, ex := range existingList {
		if ex.UserID == userID {
			m.members[teamID] = append(existingList[:i], existingList[i+1:]...)
			return nil
		}
	}
	return apperror.NotFound("team member")
}

func (m *memoryTeamRepo) ListMembers(ctx context.Context, teamID uuid.UUID) ([]*TeamMember, error) {
	return m.members[teamID], nil
}

func (m *memoryTeamRepo) ListUserTeamMemberships(ctx context.Context, userID uuid.UUID) ([]UserTeamMembership, error) {
	var list []UserTeamMembership
	for tID, mems := range m.members {
		for _, mem := range mems {
			if mem.UserID == userID {
				list = append(list, UserTeamMembership{
					TeamID:   tID,
					TeamRole: mem.Role,
				})
			}
		}
	}
	return list, nil
}

type memoryUserRepo struct {
	users map[uuid.UUID]*user.User
}

func newMemoryUserRepo() *memoryUserRepo {
	return &memoryUserRepo{users: make(map[uuid.UUID]*user.User)}
}

func (m *memoryUserRepo) GetByID(ctx context.Context, orgID, userID uuid.UUID) (*user.User, error) {
	u, ok := m.users[userID]
	if !ok || u.OrganizationID != orgID {
		return nil, apperror.NotFound("user")
	}
	return u, nil
}

type mockAccessRepo struct {
	allowedRoles map[uuid.UUID]access.RoleName
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
	role, ok := m.allowedRoles[userID]
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

func TestTeamServiceCreateAndList(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.Must(uuid.NewV7())

	teamRepo := newMemoryTeamRepo()
	userRepo := newMemoryUserRepo()
	accessRepo := &mockAccessRepo{allowedRoles: make(map[uuid.UUID]access.RoleName)}
	authorizer := access.NewAuthorizer(accessRepo)
	txManager := database.NewTxManager(nil)

	svc := NewService(teamRepo, userRepo, authorizer, nil, txManager)

	adminUserID := uuid.Must(uuid.NewV7())
	adminActor := access.Actor{
		UserID:         adminUserID,
		OrganizationID: orgID,
		IsSuperAdmin:   true,
	}

	team, err := svc.CreateTeam(ctx, adminActor, CreateTeamCmd{
		Slug:        "payments-team",
		Name:        "Payments Team",
		Description: "Handles payment gateways and checkout",
	})
	if err != nil {
		t.Fatalf("unexpected error creating team: %v", err)
	}

	if team.Name != "Payments Team" {
		t.Fatalf("expected Payments Team, got %s", team.Name)
	}
	if team.Slug.String() != "payments-team" {
		t.Fatalf("expected slug payments-team, got %s", team.Slug.String())
	}

	_, err = svc.CreateTeam(ctx, adminActor, CreateTeamCmd{
		Slug:        "payments-team",
		Name:        "Payments Team 2",
		Description: "Duplicate slug test",
	})
	if err == nil {
		t.Fatalf("expected conflict on duplicate team slug, got nil")
	}

	unauthActor := access.Actor{
		UserID:         uuid.Must(uuid.NewV7()),
		OrganizationID: orgID,
		IsSuperAdmin:   false,
	}
	_, err = svc.CreateTeam(ctx, unauthActor, CreateTeamCmd{
		Slug:        "fraud-team",
		Name:        "Fraud Team",
		Description: "Unauthorized creation test",
	})
	if err == nil {
		t.Fatalf("expected forbidden error for unprivileged actor, got nil")
	}

	teams, err := svc.ListTeams(ctx, adminActor)
	if err != nil {
		t.Fatalf("unexpected error listing teams: %v", err)
	}
	if len(teams) != 1 {
		t.Fatalf("expected 1 team, got %d", len(teams))
	}
	if teams[0].MemberCount != 0 {
		t.Fatalf("expected member count 0, got %d", teams[0].MemberCount)
	}
}

func TestTeamServiceAddAndRemoveMember(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.Must(uuid.NewV7())
	otherOrgID := uuid.Must(uuid.NewV7())

	teamRepo := newMemoryTeamRepo()
	userRepo := newMemoryUserRepo()
	accessRepo := &mockAccessRepo{allowedRoles: make(map[uuid.UUID]access.RoleName)}
	authorizer := access.NewAuthorizer(accessRepo)
	txManager := database.NewTxManager(nil)

	svc := NewService(teamRepo, userRepo, authorizer, nil, txManager)

	adminUserID := uuid.Must(uuid.NewV7())
	adminActor := access.Actor{
		UserID:         adminUserID,
		OrganizationID: orgID,
		IsSuperAdmin:   true,
	}

	team, err := svc.CreateTeam(ctx, adminActor, CreateTeamCmd{
		Slug:        "core-qa",
		Name:        "QA Team",
		Description: "Testing infrastructure and verification",
	})
	if err != nil {
		t.Fatalf("create team: %v", err)
	}

	leadEmail, _ := valueobject.NewEmail("lead@acme.com")
	leadUser, _ := user.NewUser(orgID, leadEmail, "Lead Tester", "passhash", false)
	userRepo.users[leadUser.ID] = leadUser

	devEmail, _ := valueobject.NewEmail("dev@acme.com")
	devUser, _ := user.NewUser(orgID, devEmail, "Dev Tester", "passhash", false)
	userRepo.users[devUser.ID] = devUser

	otherEmail, _ := valueobject.NewEmail("other@othercorp.com")
	otherUser, _ := user.NewUser(otherOrgID, otherEmail, "External User", "passhash", false)
	userRepo.users[otherUser.ID] = otherUser

	member, err := svc.AddTeamMember(ctx, adminActor, team.ID, leadUser.ID, RoleLead)
	if err != nil {
		t.Fatalf("unexpected error adding lead member: %v", err)
	}
	if member.Role != RoleLead {
		t.Fatalf("expected role LEAD, got %s", member.Role)
	}

	leadActor := access.Actor{
		UserID:         leadUser.ID,
		OrganizationID: orgID,
		IsSuperAdmin:   false,
		LeadTeamIDs:    []uuid.UUID{team.ID},
		TeamIDs:        []uuid.UUID{team.ID},
	}

	devMember, err := svc.AddTeamMember(ctx, leadActor, team.ID, devUser.ID, RoleMember)
	if err != nil {
		t.Fatalf("unexpected error adding member by team lead: %v", err)
	}
	if devMember.Role != RoleMember {
		t.Fatalf("expected role MEMBER, got %s", devMember.Role)
	}

	_, err = svc.AddTeamMember(ctx, leadActor, team.ID, devUser.ID, RoleMember)
	if err == nil {
		t.Fatalf("expected conflict on duplicate team member, got nil")
	}

	_, err = svc.AddTeamMember(ctx, leadActor, team.ID, otherUser.ID, RoleMember)
	if err == nil {
		t.Fatalf("expected not found or cross-org error when adding user from other org, got nil")
	}

	details, err := svc.GetTeamDetails(ctx, adminActor, team.ID)
	if err != nil {
		t.Fatalf("get team details: %v", err)
	}
	if len(details.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(details.Members))
	}

	err = svc.RemoveTeamMember(ctx, leadActor, team.ID, devUser.ID)
	if err != nil {
		t.Fatalf("unexpected error removing member: %v", err)
	}

	details, err = svc.GetTeamDetails(ctx, adminActor, team.ID)
	if err != nil {
		t.Fatalf("get team details after removal: %v", err)
	}
	if len(details.Members) != 1 {
		t.Fatalf("expected 1 member after removal, got %d", len(details.Members))
	}
}
