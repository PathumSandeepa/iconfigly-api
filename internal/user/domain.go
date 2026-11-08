package user

import (
	"strings"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/valueobject"
	"github.com/google/uuid"
)

type UserStatus string

const (
	StatusActive   UserStatus = "ACTIVE"
	StatusDisabled UserStatus = "DISABLED"
)

type User struct {
	ID             uuid.UUID          `json:"id"`
	OrganizationID uuid.UUID          `json:"organization_id"`
	Email          valueobject.Email  `json:"email"`
	FullName       string             `json:"full_name"`
	PasswordHash   string             `json:"-"`
	IsSuperAdmin   bool               `json:"is_super_admin"`
	Status         UserStatus         `json:"status"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

func NewUser(
	orgID uuid.UUID,
	email valueobject.Email,
	fullName string,
	passwordHash string,
	isSuperAdmin bool,
) (*User, error) {
	trimmedName := strings.TrimSpace(fullName)
	if len(trimmedName) < 2 || len(trimmedName) > 100 {
		return nil, apperror.Validation("invalid full name", map[string]string{
			"full_name": "must be between 2 and 100 characters",
		})
	}

	if passwordHash == "" {
		return nil, apperror.Validation("invalid password", map[string]string{
			"password": "password hash cannot be empty",
		})
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	return &User{
		ID:             uuid.Must(uuid.NewV7()),
		OrganizationID: orgID,
		Email:          email,
		FullName:       trimmedName,
		PasswordHash:   passwordHash,
		IsSuperAdmin:   isSuperAdmin,
		Status:         StatusActive,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}
