package access

import (
	"context"
	"fmt"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
)

type Authorizer struct {
	repo AccessRepository
}

func NewAuthorizer(repo AccessRepository) *Authorizer {
	return &Authorizer{repo: repo}
}

func (a *Authorizer) Can(ctx context.Context, actor Actor, verb PermissionVerb, targetScope Scope) (bool, error) {
	if actor.IsSuperAdmin {
		return true, nil
	}

	if actor.OrganizationID != targetScope.OrganizationID {
		return false, nil
	}

	ancestors := targetScope.Ancestors()
	if len(ancestors) == 0 {
		return false, nil
	}

	bindings, err := a.repo.FindActiveBindingsForActor(ctx, actor.OrganizationID, actor.UserID, actor.TeamIDs, ancestors, time.Now().UTC())
	if err != nil {
		return false, err
	}

	for _, b := range bindings {
		if HasPermission(b.RoleName, verb, targetScope.Type) {
			return true, nil
		}
	}

	return false, nil
}

func (a *Authorizer) Require(ctx context.Context, actor Actor, verb PermissionVerb, targetScope Scope) error {
	allowed, err := a.Can(ctx, actor, verb, targetScope)
	if err != nil {
		return err
	}
	if !allowed {
		return apperror.Forbidden(fmt.Sprintf("actor lacks %s permission on %s scope %s", verb, targetScope.Type, targetScope.ID))
	}
	return nil
}

func (a *Authorizer) ResolveEffectiveRole(ctx context.Context, actor Actor, targetScope Scope) (RoleName, error) {
	if actor.IsSuperAdmin {
		return RoleOrgAdmin, nil
	}

	if actor.OrganizationID != targetScope.OrganizationID {
		return "", nil
	}

	ancestors := targetScope.Ancestors()
	bindings, err := a.repo.FindActiveBindingsForActor(ctx, actor.OrganizationID, actor.UserID, actor.TeamIDs, ancestors, time.Now().UTC())
	if err != nil {
		return "", err
	}

	roleOrder := []RoleName{RoleOrgAdmin, RoleEnvAdmin, RoleServiceLead, RoleDeveloper, RoleViewer}
	for _, role := range roleOrder {
		for _, b := range bindings {
			if b.RoleName == role {
				return role, nil
			}
		}
	}

	return "", nil
}
