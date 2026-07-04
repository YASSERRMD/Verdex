-- Phase 087: packages/integration. Every table here is per-tenant
-- data (unlike packages/compliance's shared compliance_controls
-- catalogue) since a connector configuration, its credentials
-- reference, its field mapping, and every import/delivery/
-- reconciliation run are all tenant-specific -- there is no shared
-- reference-data table in this phase.

CREATE TABLE IF NOT EXISTS integration_connector_configs (
    id                UUID NOT NULL PRIMARY KEY,
    tenant_id         UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    connector_type    TEXT NOT NULL,
    display_name      TEXT NOT NULL,
    endpoint          TEXT NOT NULL DEFAULT '',
    credentials_id    UUID,
    field_mapping_id  UUID,
    enabled           BOOLEAN NOT NULL DEFAULT true,
    created_by        UUID NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT integration_connector_configs_type_not_blank CHECK (length(trim(connector_type)) > 0),
    CONSTRAINT integration_connector_configs_name_not_blank CHECK (length(trim(display_name)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_integration_connector_configs_tenant_id ON integration_connector_configs (tenant_id);

-- ConnectorCredentials never carries raw secret material -- only a
-- reference/handle (secret_ref) into packages/keymanagement or
-- packages/encryption. See packages/integration/credentials.go's doc
-- comment for the full rationale.
CREATE TABLE IF NOT EXISTS integration_connector_credentials (
    id                UUID NOT NULL PRIMARY KEY,
    tenant_id         UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    kind              TEXT NOT NULL,
    secret_ref        TEXT NOT NULL DEFAULT '',
    client_id         TEXT NOT NULL DEFAULT '',
    token_url         TEXT NOT NULL DEFAULT '',
    scopes            JSONB NOT NULL DEFAULT '[]'::jsonb,
    last_verified_at  TIMESTAMPTZ,
    created_by        UUID NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT integration_credentials_kind_not_blank CHECK (length(trim(kind)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_integration_credentials_tenant_id ON integration_connector_credentials (tenant_id);

CREATE TABLE IF NOT EXISTS integration_field_mappings (
    id              UUID NOT NULL PRIMARY KEY,
    tenant_id       UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    connector_type  TEXT NOT NULL,
    name            TEXT NOT NULL,
    rules           JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_by      UUID NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT integration_field_mappings_type_not_blank CHECK (length(trim(connector_type)) > 0),
    CONSTRAINT integration_field_mappings_name_not_blank CHECK (length(trim(name)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_integration_field_mappings_tenant_id ON integration_field_mappings (tenant_id);

CREATE TABLE IF NOT EXISTS integration_import_runs (
    id                     UUID NOT NULL PRIMARY KEY,
    tenant_id              UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    connector_config_id    UUID NOT NULL REFERENCES integration_connector_configs (id) ON DELETE CASCADE,
    since                  TIMESTAMPTZ,
    status                 TEXT NOT NULL,
    imported_count         INTEGER NOT NULL DEFAULT 0,
    mapped_count           INTEGER NOT NULL DEFAULT 0,
    failed_external_ids    JSONB NOT NULL DEFAULT '[]'::jsonb,
    imported_external_ids  JSONB NOT NULL DEFAULT '[]'::jsonb,
    error_message          TEXT NOT NULL DEFAULT '',
    started_at             TIMESTAMPTZ NOT NULL,
    finished_at            TIMESTAMPTZ NOT NULL,
    triggered_by           UUID NOT NULL,
    CONSTRAINT integration_import_runs_status_not_blank CHECK (length(trim(status)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_integration_import_runs_tenant_id ON integration_import_runs (tenant_id);
CREATE INDEX IF NOT EXISTS idx_integration_import_runs_tenant_connector ON integration_import_runs (tenant_id, connector_config_id);

CREATE TABLE IF NOT EXISTS integration_delivery_runs (
    id                     UUID NOT NULL PRIMARY KEY,
    tenant_id              UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    connector_config_id    UUID NOT NULL REFERENCES integration_connector_configs (id) ON DELETE CASCADE,
    case_external_id       TEXT NOT NULL DEFAULT '',
    report_kind            TEXT NOT NULL DEFAULT '',
    status                 TEXT NOT NULL,
    external_receipt_id    TEXT NOT NULL DEFAULT '',
    detail                 TEXT NOT NULL DEFAULT '',
    attempt_count          INTEGER NOT NULL DEFAULT 0,
    started_at             TIMESTAMPTZ NOT NULL,
    finished_at            TIMESTAMPTZ NOT NULL,
    triggered_by           UUID NOT NULL,
    CONSTRAINT integration_delivery_runs_status_not_blank CHECK (length(trim(status)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_integration_delivery_runs_tenant_id ON integration_delivery_runs (tenant_id);
CREATE INDEX IF NOT EXISTS idx_integration_delivery_runs_tenant_connector ON integration_delivery_runs (tenant_id, connector_config_id);

CREATE TABLE IF NOT EXISTS integration_reconciliation_results (
    id                       UUID NOT NULL PRIMARY KEY,
    tenant_id                UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    connector_config_id      UUID NOT NULL REFERENCES integration_connector_configs (id) ON DELETE CASCADE,
    kind                     TEXT NOT NULL,
    expected_count           INTEGER NOT NULL DEFAULT 0,
    observed_count           INTEGER NOT NULL DEFAULT 0,
    missing_external_ids     JSONB NOT NULL DEFAULT '[]'::jsonb,
    unexpected_external_ids  JSONB NOT NULL DEFAULT '[]'::jsonb,
    ran_at                   TIMESTAMPTZ NOT NULL,
    ran_by                   UUID NOT NULL,
    CONSTRAINT integration_reconciliation_kind_not_blank CHECK (length(trim(kind)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_integration_reconciliation_tenant_id ON integration_reconciliation_results (tenant_id);
CREATE INDEX IF NOT EXISTS idx_integration_reconciliation_tenant_connector ON integration_reconciliation_results (tenant_id, connector_config_id);
