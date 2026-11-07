CREATE TABLE role_bindings (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    subject_type TEXT NOT NULL CHECK (subject_type IN ('USER', 'TEAM')),
    subject_id UUID NOT NULL,
    role_name TEXT NOT NULL CHECK (role_name IN ('ORG_ADMIN', 'ENV_ADMIN', 'SERVICE_LEAD', 'DEVELOPER', 'VIEWER')),
    scope_type TEXT NOT NULL CHECK (scope_type IN ('ORG', 'PROJECT', 'ENVIRONMENT', 'SERVICE', 'RESOURCE')),
    scope_id UUID NOT NULL,
    granted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    expires_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_role_bindings_unique UNIQUE (organization_id, subject_type, subject_id, role_name, scope_type, scope_id)
);

CREATE INDEX idx_role_bindings_subject ON role_bindings (organization_id, subject_type, subject_id);
CREATE INDEX idx_role_bindings_scope ON role_bindings (organization_id, scope_type, scope_id);
