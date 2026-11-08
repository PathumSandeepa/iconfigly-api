package access

import (
	"time"

	"github.com/google/uuid"
)

type RoleName string

const (
	RoleOrgAdmin    RoleName = "ORG_ADMIN"
	RoleEnvAdmin    RoleName = "ENV_ADMIN"
	RoleServiceLead RoleName = "SERVICE_LEAD"
	RoleDeveloper   RoleName = "DEVELOPER"
	RoleViewer      RoleName = "VIEWER"
)

type PermissionVerb string

const (
	VerbOrgManage    PermissionVerb = "org.manage"
	VerbCatalogRead  PermissionVerb = "catalog.read"
	VerbCatalogWrite PermissionVerb = "catalog.write"
	VerbAccessRead   PermissionVerb = "access.read"
	VerbAccessManage PermissionVerb = "access.manage"
	VerbAuditRead    PermissionVerb = "audit.read"
)

type ScopeType string

const (
	ScopeOrg         ScopeType = "ORG"
	ScopeProject     ScopeType = "PROJECT"
	ScopeEnvironment ScopeType = "ENVIRONMENT"
	ScopeService     ScopeType = "SERVICE"
	ScopeResource    ScopeType = "RESOURCE"
)

type SubjectType string

const (
	SubjectUser SubjectType = "USER"
	SubjectTeam SubjectType = "TEAM"
)

type Actor struct {
	UserID         uuid.UUID   `json:"user_id"`
	OrganizationID uuid.UUID   `json:"organization_id"`
	Email          string      `json:"email"`
	IsSuperAdmin   bool        `json:"is_super_admin"`
	TeamIDs        []uuid.UUID `json:"team_ids"`
	LeadTeamIDs    []uuid.UUID `json:"lead_team_ids"`
	AuthMethod     string      `json:"auth_method"`
	CSRFToken      string      `json:"csrf_token"`
	ClientIP       string      `json:"client_ip"`
}

type ScopeRef struct {
	Type ScopeType `json:"type"`
	ID   uuid.UUID `json:"id"`
}

type Scope struct {
	Type           ScopeType  `json:"type"`
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	ProjectID      *uuid.UUID `json:"project_id,omitempty"`
	EnvironmentID  *uuid.UUID `json:"environment_id,omitempty"`
}

func NewOrgScope(orgID uuid.UUID) Scope {
	return Scope{
		Type:           ScopeOrg,
		ID:             orgID,
		OrganizationID: orgID,
	}
}

func NewProjectScope(orgID, projectID uuid.UUID) Scope {
	return Scope{
		Type:           ScopeProject,
		ID:             projectID,
		OrganizationID: orgID,
		ProjectID:      &projectID,
	}
}

func NewEnvironmentScope(orgID, projectID, envID uuid.UUID) Scope {
	return Scope{
		Type:           ScopeEnvironment,
		ID:             envID,
		OrganizationID: orgID,
		ProjectID:      &projectID,
		EnvironmentID:  &envID,
	}
}

func NewServiceScope(orgID, projectID, envID, serviceID uuid.UUID) Scope {
	return Scope{
		Type:           ScopeService,
		ID:             serviceID,
		OrganizationID: orgID,
		ProjectID:      &projectID,
		EnvironmentID:  &envID,
	}
}

func NewResourceScope(orgID, projectID, envID, resourceID uuid.UUID) Scope {
	return Scope{
		Type:           ScopeResource,
		ID:             resourceID,
		OrganizationID: orgID,
		ProjectID:      &projectID,
		EnvironmentID:  &envID,
	}
}

func (s Scope) Ancestors() []ScopeRef {
	var list []ScopeRef
	list = append(list, ScopeRef{Type: s.Type, ID: s.ID})

	if s.Type == ScopeService || s.Type == ScopeResource {
		if s.EnvironmentID != nil {
			list = append(list, ScopeRef{Type: ScopeEnvironment, ID: *s.EnvironmentID})
		}
		if s.ProjectID != nil {
			list = append(list, ScopeRef{Type: ScopeProject, ID: *s.ProjectID})
		}
		list = append(list, ScopeRef{Type: ScopeOrg, ID: s.OrganizationID})
		return list
	}

	if s.Type == ScopeEnvironment {
		if s.ProjectID != nil {
			list = append(list, ScopeRef{Type: ScopeProject, ID: *s.ProjectID})
		}
		list = append(list, ScopeRef{Type: ScopeOrg, ID: s.OrganizationID})
		return list
	}

	if s.Type == ScopeProject {
		list = append(list, ScopeRef{Type: ScopeOrg, ID: s.OrganizationID})
		return list
	}

	return list
}

type RoleBinding struct {
	ID             uuid.UUID   `json:"id"`
	OrganizationID uuid.UUID   `json:"organization_id"`
	SubjectType    SubjectType `json:"subject_type"`
	SubjectID      uuid.UUID   `json:"subject_id"`
	RoleName       RoleName    `json:"role_name"`
	ScopeType      ScopeType   `json:"scope_type"`
	ScopeID        uuid.UUID   `json:"scope_id"`
	GrantedBy      *uuid.UUID  `json:"granted_by,omitempty"`
	ExpiresAt      *time.Time  `json:"expires_at,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
}

func (b *RoleBinding) IsExpired(now time.Time) bool {
	if b.ExpiresAt == nil {
		return false
	}
	return !b.ExpiresAt.After(now)
}

type EffectiveGrant struct {
	BindingID   uuid.UUID   `json:"binding_id"`
	SubjectType SubjectType `json:"subject_type"`
	SubjectID   uuid.UUID   `json:"subject_id"`
	RoleName    RoleName    `json:"role_name"`
	ScopeType   ScopeType   `json:"scope_type"`
	ScopeID     uuid.UUID   `json:"scope_id"`
	Source      string      `json:"source"`
	GrantedBy   *uuid.UUID  `json:"granted_by,omitempty"`
	ExpiresAt   *time.Time  `json:"expires_at,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
}

type TargetAccessInspection struct {
	ScopeType  ScopeType        `json:"scope_type"`
	ScopeID    uuid.UUID        `json:"scope_id"`
	UserGrants []EffectiveGrant `json:"user_grants"`
	TeamGrants []EffectiveGrant `json:"team_grants"`
}

func HasPermission(role RoleName, verb PermissionVerb, targetScopeType ScopeType) bool {
	switch role {
	case RoleOrgAdmin:
		return true

	case RoleEnvAdmin:
		switch verb {
		case VerbCatalogRead, VerbCatalogWrite, VerbAccessRead, VerbAuditRead:
			return true
		case VerbAccessManage:
			return targetScopeType == ScopeEnvironment || targetScopeType == ScopeResource
		default:
			return false
		}

	case RoleServiceLead:
		switch verb {
		case VerbCatalogRead, VerbCatalogWrite, VerbAccessRead, VerbAuditRead:
			return true
		case VerbAccessManage:
			return targetScopeType == ScopeService
		default:
			return false
		}

	case RoleDeveloper:
		switch verb {
		case VerbCatalogRead, VerbAccessRead:
			return true
		default:
			return false
		}

	case RoleViewer:
		switch verb {
		case VerbCatalogRead, VerbAccessRead, VerbAuditRead:
			return true
		default:
			return false
		}

	default:
		return false
	}
}

func IsValidRole(role RoleName) bool {
	switch role {
	case RoleOrgAdmin, RoleEnvAdmin, RoleServiceLead, RoleDeveloper, RoleViewer:
		return true
	default:
		return false
	}
}

func IsValidScopeType(st ScopeType) bool {
	switch st {
	case ScopeOrg, ScopeProject, ScopeEnvironment, ScopeService, ScopeResource:
		return true
	default:
		return false
	}
}

func IsValidSubjectType(st SubjectType) bool {
	switch st {
	case SubjectUser, SubjectTeam:
		return true
	default:
		return false
	}
}
