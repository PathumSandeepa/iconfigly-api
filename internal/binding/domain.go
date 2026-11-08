package binding

import (
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/google/uuid"
)

type AccessMode string

const (
	ModeRead      AccessMode = "READ"
	ModeWrite     AccessMode = "WRITE"
	ModeReadWrite AccessMode = "READ_WRITE"
	ModeAdmin     AccessMode = "ADMIN"
)

func IsValidAccessMode(m AccessMode) bool {
	switch m {
	case ModeRead, ModeWrite, ModeReadWrite, ModeAdmin:
		return true
	default:
		return false
	}
}

type DependencyProtocol string

const (
	ProtocolHTTP  DependencyProtocol = "HTTP"
	ProtocolGRPC  DependencyProtocol = "GRPC"
	ProtocolKafka DependencyProtocol = "KAFKA"
	ProtocolEvent DependencyProtocol = "EVENT"
)

func IsValidProtocol(p DependencyProtocol) bool {
	switch p {
	case ProtocolHTTP, ProtocolGRPC, ProtocolKafka, ProtocolEvent:
		return true
	default:
		return false
	}
}

type ServiceResourceBinding struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	ProjectID      uuid.UUID  `json:"project_id"`
	EnvironmentID  uuid.UUID  `json:"environment_id"`
	ServiceID      uuid.UUID  `json:"service_id"`
	ResourceID     uuid.UUID  `json:"resource_id"`
	AccessMode     AccessMode `json:"access_mode"`
	Notes          string     `json:"notes"`
	CreatedAt      time.Time  `json:"created_at"`
}

func NewServiceResourceBinding(
	orgID, projectID, envID, serviceID, resourceID uuid.UUID,
	mode AccessMode,
	notes string,
) (*ServiceResourceBinding, error) {
	if !IsValidAccessMode(mode) {
		return nil, apperror.Validation("invalid access mode", map[string]string{
			"access_mode": "must be READ, WRITE, READ_WRITE, or ADMIN",
		})
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	return &ServiceResourceBinding{
		ID:             uuid.Must(uuid.NewV7()),
		OrganizationID: orgID,
		ProjectID:      projectID,
		EnvironmentID:  envID,
		ServiceID:      serviceID,
		ResourceID:     resourceID,
		AccessMode:     mode,
		Notes:          notes,
		CreatedAt:      now,
	}, nil
}

type ServiceDependency struct {
	ID              uuid.UUID          `json:"id"`
	OrganizationID  uuid.UUID          `json:"organization_id"`
	ProjectID       uuid.UUID          `json:"project_id"`
	EnvironmentID   uuid.UUID          `json:"environment_id"`
	CallerServiceID uuid.UUID          `json:"caller_service_id"`
	CalleeServiceID uuid.UUID          `json:"callee_service_id"`
	Protocol        DependencyProtocol `json:"protocol"`
	CreatedAt       time.Time          `json:"created_at"`
}

func NewServiceDependency(
	orgID, projectID, envID, callerID, calleeID uuid.UUID,
	protocol DependencyProtocol,
) (*ServiceDependency, error) {
	if callerID == calleeID {
		return nil, apperror.Validation("invalid dependency", map[string]string{
			"callee_service_id": "caller service cannot depend on itself",
		})
	}

	if !IsValidProtocol(protocol) {
		return nil, apperror.Validation("invalid protocol", map[string]string{
			"protocol": "must be HTTP, GRPC, KAFKA, or EVENT",
		})
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	return &ServiceDependency{
		ID:              uuid.Must(uuid.NewV7()),
		OrganizationID:  orgID,
		ProjectID:       projectID,
		EnvironmentID:   envID,
		CallerServiceID: callerID,
		CalleeServiceID: calleeID,
		Protocol:        protocol,
		CreatedAt:       now,
	}, nil
}

type GraphNodeType string

const (
	NodeTypeService  GraphNodeType = "SERVICE"
	NodeTypeResource GraphNodeType = "RESOURCE"
)

type GraphNode struct {
	ID       uuid.UUID      `json:"id"`
	Type     GraphNodeType  `json:"type"`
	Name     string         `json:"name"`
	Slug     string         `json:"slug"`
	Metadata map[string]any `json:"metadata"`
}

type GraphEdgeType string

const (
	EdgeTypeBinding    GraphEdgeType = "BINDING"
	EdgeTypeDependency GraphEdgeType = "DEPENDENCY"
)

type GraphEdge struct {
	ID       uuid.UUID      `json:"id"`
	Type     GraphEdgeType  `json:"type"`
	SourceID uuid.UUID      `json:"source_id"`
	TargetID uuid.UUID      `json:"target_id"`
	Label    string         `json:"label"`
	Metadata map[string]any `json:"metadata"`
}

type TopologyGraph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

type BoundResourceView struct {
	BindingID          uuid.UUID      `json:"binding_id"`
	ResourceID         uuid.UUID      `json:"resource_id"`
	ResourceSlug       string         `json:"resource_slug"`
	ResourceName       string         `json:"resource_name"`
	ResourceType       string         `json:"resource_type"`
	AccessMode         string         `json:"access_mode"`
	ConnectionMetadata map[string]any `json:"connection_metadata"`
	Notes              string         `json:"notes"`
}

type ServiceDependencyView struct {
	DependencyID uuid.UUID `json:"dependency_id"`
	ServiceID    uuid.UUID `json:"service_id"`
	ServiceSlug  string    `json:"service_slug"`
	ServiceName  string    `json:"service_name"`
	Protocol     string    `json:"protocol"`
}

type BindServiceCmd struct {
	EnvironmentID uuid.UUID  `json:"environment_id"`
	ServiceID     uuid.UUID  `json:"service_id"`
	ResourceID    uuid.UUID  `json:"resource_id"`
	AccessMode    AccessMode `json:"access_mode"`
	Notes         string     `json:"notes"`
}

type AddDependencyCmd struct {
	EnvironmentID   uuid.UUID          `json:"environment_id"`
	CallerServiceID uuid.UUID          `json:"caller_service_id"`
	CalleeServiceID uuid.UUID          `json:"callee_service_id"`
	Protocol        DependencyProtocol `json:"protocol"`
}
