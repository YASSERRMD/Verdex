CREATE TABLE IF NOT EXISTS saved_searches (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    owner_id    UUID NOT NULL,
    name        TEXT NOT NULL,
    query       JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT saved_searches_name_not_blank CHECK (btrim(name) <> '')
);

CREATE INDEX IF NOT EXISTS idx_saved_searches_tenant_id ON saved_searches (tenant_id);
CREATE INDEX IF NOT EXISTS idx_saved_searches_owner_id ON saved_searches (owner_id);
CREATE INDEX IF NOT EXISTS idx_saved_searches_tenant_owner ON saved_searches (tenant_id, owner_id);
