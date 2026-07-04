CREATE TABLE IF NOT EXISTS corpusupdater_jobs (
    id                   UUID NOT NULL PRIMARY KEY,
    tenant_id            UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    jurisdiction_code    TEXT NOT NULL,
    target_corpus        TEXT NOT NULL,
    source_description   TEXT NOT NULL DEFAULT '',
    status               TEXT NOT NULL,
    failure_reason       TEXT NOT NULL DEFAULT '',
    created_by           UUID NOT NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT corpusupdater_jobs_jurisdiction_not_blank CHECK (length(trim(jurisdiction_code)) > 0),
    CONSTRAINT corpusupdater_jobs_target_corpus_allowed CHECK (target_corpus IN ('statute', 'precedent')),
    CONSTRAINT corpusupdater_jobs_status_allowed CHECK (status IN ('pending', 'validating', 'applying', 'applied', 'failed', 'rolled_back'))
);

CREATE INDEX IF NOT EXISTS idx_corpusupdater_jobs_tenant_id ON corpusupdater_jobs (tenant_id);
CREATE INDEX IF NOT EXISTS idx_corpusupdater_jobs_tenant_jurisdiction ON corpusupdater_jobs (tenant_id, jurisdiction_code);
CREATE INDEX IF NOT EXISTS idx_corpusupdater_jobs_tenant_status ON corpusupdater_jobs (tenant_id, status);

CREATE TABLE IF NOT EXISTS corpusupdater_amendments (
    id                   UUID NOT NULL PRIMARY KEY,
    tenant_id            UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    job_id               UUID NOT NULL REFERENCES corpusupdater_jobs (id) ON DELETE CASCADE,
    target_corpus        TEXT NOT NULL,
    target_id            TEXT NOT NULL DEFAULT '',
    change_type          TEXT NOT NULL,
    new_text             TEXT NOT NULL DEFAULT '',
    citation             TEXT NOT NULL,
    effective_date       TIMESTAMPTZ NOT NULL,
    previous_text        TEXT NOT NULL DEFAULT '',
    previous_citation    TEXT NOT NULL DEFAULT '',
    applied              BOOLEAN NOT NULL DEFAULT false,
    rolled_back          BOOLEAN NOT NULL DEFAULT false,
    created_by           UUID NOT NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT corpusupdater_amendments_target_corpus_allowed CHECK (target_corpus IN ('statute', 'precedent')),
    CONSTRAINT corpusupdater_amendments_change_type_allowed CHECK (change_type IN ('add', 'amend', 'repeal')),
    CONSTRAINT corpusupdater_amendments_citation_not_blank CHECK (length(trim(citation)) > 0),
    -- Mirrors Amendment.Validate's requiresTarget guard at the database
    -- layer: amend/repeal changes must name the rule/precedent they
    -- change.
    CONSTRAINT corpusupdater_amendments_target_id_required CHECK (
        change_type NOT IN ('amend', 'repeal') OR length(trim(target_id)) > 0
    )
);

CREATE INDEX IF NOT EXISTS idx_corpusupdater_amendments_tenant_id ON corpusupdater_amendments (tenant_id);
CREATE INDEX IF NOT EXISTS idx_corpusupdater_amendments_tenant_job ON corpusupdater_amendments (tenant_id, job_id);
CREATE INDEX IF NOT EXISTS idx_corpusupdater_amendments_tenant_target ON corpusupdater_amendments (tenant_id, target_corpus, target_id);
CREATE INDEX IF NOT EXISTS idx_corpusupdater_amendments_tenant_effective ON corpusupdater_amendments (tenant_id, effective_date);
