CREATE TABLE service_resource_bindings (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    resource_id UUID NOT NULL REFERENCES resources(id) ON DELETE CASCADE,
    access_mode TEXT NOT NULL DEFAULT 'READ_WRITE' CHECK (access_mode IN ('READ', 'WRITE', 'READ_WRITE', 'ADMIN')),
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_srb_env_service_resource UNIQUE (environment_id, service_id, resource_id)
);

CREATE INDEX idx_srb_resource_env ON service_resource_bindings (resource_id, environment_id);
CREATE INDEX idx_srb_service_env ON service_resource_bindings (service_id, environment_id);

CREATE TABLE service_dependencies (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    caller_service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    callee_service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    protocol TEXT NOT NULL DEFAULT 'HTTP' CHECK (protocol IN ('HTTP', 'GRPC', 'KAFKA', 'EVENT')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_caller_callee_different CHECK (caller_service_id <> callee_service_id),
    CONSTRAINT uq_sdep_env_caller_callee UNIQUE (environment_id, caller_service_id, callee_service_id)
);

CREATE INDEX idx_sdep_callee_env ON service_dependencies (callee_service_id, environment_id);
