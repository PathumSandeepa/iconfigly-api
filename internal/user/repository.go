package user

import (
	"context"

	"github.com/PathumSandeepa/iconfigly-api/internal/shared/valueobject"
	"github.com/google/uuid"
)

type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, orgID, userID uuid.UUID) (*User, error)
	GetByEmail(ctx context.Context, email valueobject.Email) (*User, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]*User, error)
	UpdateStatus(ctx context.Context, orgID, userID uuid.UUID, status UserStatus) error
	CountActiveSuperAdmins(ctx context.Context, orgID uuid.UUID) (int64, error)
}
