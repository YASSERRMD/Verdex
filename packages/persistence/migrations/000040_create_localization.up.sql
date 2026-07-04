-- Phase 090: packages/localization. A single small table: which
-- Locale each user prefers, per tenant. Unlike packages/compliance's
-- shared compliance_controls catalogue, this table is tenant-scoped
-- per-row (like packages/integration's tables), since a locale
-- preference is inherently a per-user, per-tenant setting -- there is
-- no shared reference-data table in this phase. The translation
-- Catalog itself (packages/localization/seed.go) is compiled-in seed
-- data, not a database table: it needs no migration.

CREATE TABLE IF NOT EXISTS localization_preferences (
    id          UUID NOT NULL PRIMARY KEY,
    tenant_id   UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    user_id     UUID NOT NULL,
    locale      TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT localization_preferences_locale_not_blank CHECK (length(trim(locale)) > 0),
    CONSTRAINT localization_preferences_tenant_user_unique UNIQUE (tenant_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_localization_preferences_tenant_id ON localization_preferences (tenant_id);
