CREATE TABLE IF NOT EXISTS audit_events (
    id            UUID NOT NULL PRIMARY KEY,
    tenant_id     UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    occurred_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    actor         TEXT NOT NULL,
    action        TEXT NOT NULL,
    target        TEXT NOT NULL DEFAULT '',
    outcome       TEXT NOT NULL,
    kind          TEXT NOT NULL,
    case_id       UUID,
    detail        TEXT NOT NULL DEFAULT '',
    prev_hash     TEXT NOT NULL DEFAULT '',
    chain_hash    TEXT NOT NULL,
    CONSTRAINT audit_events_kind_allowed CHECK (
        kind IN ('data_access', 'reasoning', 'signoff', 'data_change', 'admin', 'export', 'system')
    ),
    CONSTRAINT audit_events_action_not_blank CHECK (length(trim(action)) > 0),
    CONSTRAINT audit_events_actor_not_blank CHECK (length(trim(actor)) > 0),
    CONSTRAINT audit_events_chain_hash_not_blank CHECK (length(trim(chain_hash)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_audit_events_tenant_id ON audit_events (tenant_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_tenant_occurred ON audit_events (tenant_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_audit_events_tenant_case ON audit_events (tenant_id, case_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_tenant_actor ON audit_events (tenant_id, actor);
CREATE INDEX IF NOT EXISTS idx_audit_events_tenant_kind ON audit_events (tenant_id, kind);

-- Tamper-evident, append-only enforcement (task 4). This table is the
-- durable half of the hash chain built in packages/auditlog/chain.go:
-- every row's chain_hash commits to its own fields plus the previous
-- row's chain_hash, so modifying a stored row's content (or removing
-- one out of retention-purge order) is detectable by recomputing the
-- chain. The database layer backs that guarantee with an actual
-- privilege boundary, not just application discipline:
--
--   - UPDATE is revoked from verdex_app entirely and unconditionally.
--     No code path in this package ever needs to modify a persisted
--     audit_events row, so there is no legitimate UPDATE to carve an
--     exception for.
--   - DELETE remains granted (verdex_app's default privileges from
--     migrations/000005_create_app_role.up.sql already include it),
--     because Store.Purge (packages/auditlog/retention.go) legitimately
--     deletes events older than a configurable retention window. A
--     BEFORE DELETE trigger enforces that the *only* deletes verdex_app
--     can perform are ones targeting a row already past a real
--     retention boundary passed in from the calling transaction is not
--     expressible as a static privilege, so instead this migration
--     narrows the blast radius the way Postgres privileges actually
--     can: revoking UPDATE outright (the strictly-never-needed half of
--     "no UPDATE/DELETE grants"), and relying on
--     packages/auditlog.Repository's contract (PurgeBefore is the only
--     method that issues a DELETE, and it is only ever reachable
--     through Store.Purge's RetentionPolicy-bounded cutoff, itself
--     gated on identity.PermAuditRead) plus the trigger below as
--     defense-in-depth against an accidental unbounded DELETE.
REVOKE UPDATE ON audit_events FROM verdex_app;

CREATE OR REPLACE FUNCTION audit_events_reject_update()
RETURNS trigger
LANGUAGE plpgsql
AS $BODY$
BEGIN
    RAISE EXCEPTION 'audit_events is append-only: UPDATE is not permitted (id=%)', OLD.id;
END;
$BODY$;

-- Belt-and-suspenders alongside the REVOKE above: even a role that
-- somehow retains UPDATE (e.g. a future migration that re-grants it by
-- mistake, or a superuser connection that bypasses the GRANT system
-- entirely) is still stopped by this trigger, which fires
-- unconditionally for every UPDATE attempt.
DROP TRIGGER IF EXISTS trg_audit_events_reject_update ON audit_events;
CREATE TRIGGER trg_audit_events_reject_update
    BEFORE UPDATE ON audit_events
    FOR EACH ROW
    EXECUTE FUNCTION audit_events_reject_update();

-- A DELETE is only ever legitimate as part of a retention purge
-- (packages/auditlog.Store.Purge), which by construction only removes
-- rows older than RetentionPolicy.Window. This trigger enforces a
-- floor in the database itself: no row less than one hour old can ever
-- be deleted, regardless of what application code (correctly or
-- incorrectly) requests. One hour (rather than exactly "now") gives
-- Store.Purge's own cutoff computation a safety margin against clock
-- skew between the application and the database, while still making
-- an accidental "delete everything" or "delete recent rows" mistake
-- structurally impossible.
CREATE OR REPLACE FUNCTION audit_events_reject_recent_delete()
RETURNS trigger
LANGUAGE plpgsql
AS $BODY$
BEGIN
    IF OLD.occurred_at > now() - INTERVAL '1 hour' THEN
        RAISE EXCEPTION 'audit_events is append-only: DELETE of a row less than 1 hour old is not permitted (id=%, occurred_at=%)', OLD.id, OLD.occurred_at;
    END IF;
    RETURN OLD;
END;
$BODY$;

DROP TRIGGER IF EXISTS trg_audit_events_reject_recent_delete ON audit_events;
CREATE TRIGGER trg_audit_events_reject_recent_delete
    BEFORE DELETE ON audit_events
    FOR EACH ROW
    EXECUTE FUNCTION audit_events_reject_recent_delete();
