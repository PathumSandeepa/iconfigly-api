CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    actor_type TEXT NOT NULL DEFAULT 'USER' CHECK (actor_type IN ('USER', 'SYSTEM', 'TOKEN')),
    action TEXT NOT NULL,
    target_type TEXT NOT NULL CHECK (target_type IN ('ORGANIZATION', 'USER', 'TEAM', 'PROJECT', 'ENVIRONMENT', 'SERVICE', 'RESOURCE', 'BINDING', 'ROLE_BINDING')),
    target_id UUID NOT NULL,
    environment_id UUID REFERENCES environments(id) ON DELETE SET NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    ip_address TEXT NOT NULL DEFAULT '',
    prev_hash TEXT NOT NULL DEFAULT '',
    entry_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_org_created ON audit_logs (organization_id, created_at DESC);

CREATE OR REPLACE FUNCTION forbid_audit_log_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_logs entries are immutable and cannot be updated or deleted';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_audit_logs_immutable
BEFORE UPDATE OR DELETE ON audit_logs
FOR EACH ROW
EXECUTE FUNCTION forbid_audit_log_mutation();
