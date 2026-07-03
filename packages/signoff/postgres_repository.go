package signoff

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/YASSERRMD/verdex/packages/guardrail"
	"github.com/YASSERRMD/verdex/packages/persistence"
)

// PostgresRepository is a PostgreSQL-backed Repository, storing
// SignoffRecord rows in the `signoff_records` table and AuditEntry
// rows in the `signoff_audit_entries` table (see
// packages/persistence/migrations/000008_create_signoff.up.sql and
// 000009_enable_rls_signoff.up.sql), mirroring
// packages/caselifecycle.PostgresRepository exactly. It accepts a
// persistence.Executor per call, so callers can run it directly
// against a pool or compose it inside a transaction via
// persistence.WithTx or packages/tenancy.WithTenantScope.
type PostgresRepository struct {
	exec persistence.Executor
}

// NewPostgresRepository builds a PostgresRepository bound to exec.
func NewPostgresRepository(exec persistence.Executor) *PostgresRepository {
	return &PostgresRepository{exec: exec}
}

// Get implements Repository.
func (r *PostgresRepository) Get(ctx context.Context, tenantID, caseID uuid.UUID) (*SignoffRecord, error) {
	const q = `
		SELECT id, case_id, tenant_id, status, reviewer_id, notes, case_version, source, decided_at, created_at
		FROM signoff_records
		WHERE case_id = $1 AND tenant_id = $2`

	rec := &SignoffRecord{}
	row := r.exec.QueryRow(ctx, q, caseID, tenantID)
	if err := scanRecord(row, rec); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, wrapf("PostgresRepository.Get", err)
	}
	return rec, nil
}

// Upsert implements Repository.
func (r *PostgresRepository) Upsert(ctx context.Context, tenantID uuid.UUID, rec *SignoffRecord) error {
	if rec == nil {
		return wrapf("PostgresRepository.Upsert", ErrNilRepository)
	}
	if rec.TenantID == uuid.Nil {
		rec.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, rec.TenantID); err != nil {
		return err
	}
	if rec.ID == uuid.Nil {
		rec.ID = uuid.New()
	}

	var reviewerID any
	if rec.ReviewerID != uuid.Nil {
		reviewerID = rec.ReviewerID
	}

	const q = `
		INSERT INTO signoff_records (id, case_id, tenant_id, status, reviewer_id, notes, case_version, source, decided_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (case_id) DO UPDATE SET
			status = EXCLUDED.status,
			reviewer_id = EXCLUDED.reviewer_id,
			notes = EXCLUDED.notes,
			case_version = EXCLUDED.case_version,
			source = EXCLUDED.source,
			decided_at = EXCLUDED.decided_at
		RETURNING id, case_id, tenant_id, status, reviewer_id, notes, case_version, source, decided_at, created_at`

	row := r.exec.QueryRow(ctx, q,
		rec.ID, rec.CaseID, rec.TenantID, string(statusString(rec.Status)), reviewerID, rec.Notes,
		rec.CaseVersion, string(rec.Source), rec.DecidedAt, rec.CreatedAt,
	)
	if err := scanRecord(row, rec); err != nil {
		return wrapf("PostgresRepository.Upsert", err)
	}
	return nil
}

// AppendAudit implements Repository.
func (r *PostgresRepository) AppendAudit(ctx context.Context, tenantID uuid.UUID, e *AuditEntry) error {
	if e == nil {
		return wrapf("PostgresRepository.AppendAudit", ErrNilRepository)
	}
	if e.TenantID == uuid.Nil {
		e.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, e.TenantID); err != nil {
		return err
	}
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}

	var actor any
	if e.Actor != uuid.Nil {
		actor = e.Actor
	}

	const q = `
		INSERT INTO signoff_audit_entries (id, case_id, tenant_id, from_status, to_status, actor, source, notes, case_version, occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, case_id, tenant_id, from_status, to_status, actor, source, notes, case_version, occurred_at`

	row := r.exec.QueryRow(ctx, q,
		e.ID, e.CaseID, e.TenantID, string(statusString(e.FromStatus)), string(statusString(e.ToStatus)),
		actor, string(e.Source), e.Notes, e.CaseVersion, e.OccurredAt,
	)
	if err := scanAuditEntry(row, e); err != nil {
		return wrapf("PostgresRepository.AppendAudit", err)
	}
	return nil
}

// ListAudit implements Repository.
func (r *PostgresRepository) ListAudit(ctx context.Context, tenantID, caseID uuid.UUID) ([]*AuditEntry, error) {
	const q = `
		SELECT id, case_id, tenant_id, from_status, to_status, actor, source, notes, case_version, occurred_at
		FROM signoff_audit_entries
		WHERE case_id = $1 AND tenant_id = $2
		ORDER BY occurred_at ASC`

	rows, err := r.exec.Query(ctx, q, caseID, tenantID)
	if err != nil {
		return nil, wrapf("PostgresRepository.ListAudit", err)
	}
	defer rows.Close()

	var out []*AuditEntry
	for rows.Next() {
		e := &AuditEntry{}
		if err := scanAuditEntry(rows, e); err != nil {
			return nil, wrapf("PostgresRepository.ListAudit", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresRepository.ListAudit", err)
	}
	return out, nil
}

// rowScanner is satisfied by both pgx.Row (QueryRow) and pgx.Rows
// (Query, iterated with Next), mirroring
// packages/caselifecycle.PostgresRepository's rowScanner convention.
type rowScanner interface {
	Scan(dest ...any) error
}

func statusString(s guardrail.SignoffStatus) string {
	return s.String()
}

func parseStatus(s string) guardrail.SignoffStatus {
	switch s {
	case "approved":
		return guardrail.SignoffApproved
	case "rejected":
		return guardrail.SignoffRejected
	default:
		return guardrail.SignoffPending
	}
}

func scanRecord(row rowScanner, rec *SignoffRecord) error {
	var status, source string
	var reviewerID *uuid.UUID
	if err := row.Scan(
		&rec.ID, &rec.CaseID, &rec.TenantID, &status, &reviewerID, &rec.Notes,
		&rec.CaseVersion, &source, &rec.DecidedAt, &rec.CreatedAt,
	); err != nil {
		return err
	}
	rec.Status = parseStatus(status)
	rec.Source = DecisionSource(source)
	if reviewerID != nil {
		rec.ReviewerID = *reviewerID
	}
	return nil
}

func scanAuditEntry(row rowScanner, e *AuditEntry) error {
	var fromStatus, toStatus, source string
	var actor *uuid.UUID
	if err := row.Scan(
		&e.ID, &e.CaseID, &e.TenantID, &fromStatus, &toStatus, &actor, &source, &e.Notes,
		&e.CaseVersion, &e.OccurredAt,
	); err != nil {
		return err
	}
	e.FromStatus = parseStatus(fromStatus)
	e.ToStatus = parseStatus(toStatus)
	e.Source = DecisionSource(source)
	if actor != nil {
		e.Actor = *actor
	}
	return nil
}

var _ Repository = (*PostgresRepository)(nil)
