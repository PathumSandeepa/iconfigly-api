package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const GenesisHash = "GENESIS"

const (
	ActorTypeUser   = "USER"
	ActorTypeSystem = "SYSTEM"
	ActorTypeToken  = "TOKEN"
)

const (
	TargetTypeOrganization = "ORGANIZATION"
	TargetTypeUser         = "USER"
	TargetTypeTeam         = "TEAM"
	TargetTypeProject      = "PROJECT"
	TargetTypeEnvironment  = "ENVIRONMENT"
	TargetTypeService      = "SERVICE"
	TargetTypeResource     = "RESOURCE"
	TargetTypeBinding      = "BINDING"
	TargetTypeRoleBinding  = "ROLE_BINDING"
)

type AuditEntry struct {
	ID             uuid.UUID      `json:"id"`
	OrganizationID uuid.UUID      `json:"organization_id"`
	ActorUserID    *uuid.UUID     `json:"actor_user_id,omitempty"`
	ActorType      string         `json:"actor_type"`
	Action         string         `json:"action"`
	TargetType     string         `json:"target_type"`
	TargetID       uuid.UUID      `json:"target_id"`
	EnvironmentID  *uuid.UUID     `json:"environment_id,omitempty"`
	Metadata       map[string]any `json:"metadata"`
	IPAddress      string         `json:"ip_address"`
	PrevHash       string         `json:"prev_hash"`
	EntryHash      string         `json:"entry_hash"`
	CreatedAt      time.Time      `json:"created_at"`
}

func (e *AuditEntry) ComputeHash() (string, error) {
	actorIDStr := ""
	if e.ActorUserID != nil {
		actorIDStr = e.ActorUserID.String()
	}

	meta := e.Metadata
	if meta == nil {
		meta = map[string]any{}
	}

	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return "", fmt.Errorf("canonical json for metadata: %w", err)
	}

	payload := fmt.Sprintf(
		"%s|%s|%s|%s|%s|%s|%s|%d",
		e.PrevHash,
		e.OrganizationID.String(),
		actorIDStr,
		e.Action,
		e.TargetType,
		e.TargetID.String(),
		string(metaBytes),
		e.CreatedAt.UTC().UnixMicro(),
	)

	digest := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(digest[:]), nil
}
