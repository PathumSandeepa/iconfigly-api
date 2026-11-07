CREATE TABLE resources (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    owner_team_id UUID NOT NULL REFERENCES teams(id) ON DELETE RESTRICT,
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    resource_type TEXT NOT NULL CHECK (resource_type IN ('POSTGRES', 'MYSQL', 'REDIS', 'KAFKA', 'S3', 'EXTERNAL_API', 'CUSTOM')),
    connection_metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_resources_environment_slug UNIQUE (environment_id, slug)
);

CREATE INDEX idx_resources_env_id ON resources (environment_id);
