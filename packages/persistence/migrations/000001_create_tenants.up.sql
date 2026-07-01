CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS tenants (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT tenants_slug_unique UNIQUE (slug),
    CONSTRAINT tenants_name_not_blank CHECK (btrim(name) <> ''),
    CONSTRAINT tenants_slug_not_blank CHECK (btrim(slug) <> '')
);

CREATE INDEX IF NOT EXISTS idx_tenants_created_at ON tenants (created_at);
