package caselifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/YASSERRMD/verdex/packages/persistence"
)

// PostgresRepository is a PostgreSQL-backed Repository, storing Case
// rows in the `cases` table and TransitionRecord rows in the
// `case_transitions` table (see
// packages/persistence/migrations/000006_create_cases.up.sql and
// 000007_enable_rls_cases.up.sql). It accepts a persistence.Executor
// per call, mirroring packages/tenancy.PostgresProvisioningRecordRepository,
// so callers can run it directly against a pool or compose it inside a
// transaction via persistence.WithTx or packages/tenancy.WithTenantScope.
//
// PostgresRepository itself performs the application-level
// requireMatchingTenant guard before every statement (defense in
// depth); callers that also want Row-Level Security enforced at the
// database layer should invoke it from inside
// packages/tenancy.WithTenantScope(ctx, pool, tenantID, ...), exactly
// as packages/tenancy.TenantScopedDeploymentRepository composes
// WithTenantScope with persistence.DeploymentRepository.
type PostgresRepository struct {
	exec persistence.Executor
}

// NewPostgresRepository builds a PostgresRepository bound to exec.
// exec is typically a *pgxpool.Pool for ordinary calls, or a
// transaction-scoped Executor handed in by persistence.WithTx /
// packages/tenancy.WithTenantScope.
func NewPostgresRepository(exec persistence.Executor) *PostgresRepository {
	return &PostgresRepository{exec: exec}
}

// Create implements Repository.
func (r *PostgresRepository) Create(ctx context.Context, tenantID uuid.UUID, c *Case) error {
	if c == nil {
		return wrapf("PostgresRepository.Create", ErrInvalidCase)
	}
	if c.TenantID == uuid.Nil {
		c.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, c.TenantID); err != nil {
		return err
	}
	if err := c.Validate(); err != nil {
		return err
	}
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}

	metadata, err := marshalMetadata(c.Metadata)
	if err != nil {
		return wrapf("PostgresRepository.Create", err)
	}

	const q = `
		INSERT INTO cases (id, tenant_id, jurisdiction_id, category_id, title, reference, state, metadata, metadata_version, created_by, created_at, updated_at, archived_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, tenant_id, jurisdiction_id, category_id, title, reference, state, metadata, metadata_version, created_by, created_at, updated_at, archived_at`

	row := r.exec.QueryRow(ctx, q,
		c.ID, c.TenantID, c.JurisdictionID, c.CategoryID, c.Title, c.Reference,
		string(c.State), metadata, c.MetadataVersion, c.CreatedBy, c.CreatedAt, c.UpdatedAt, c.ArchivedAt,
	)
	if err := scanCase(row, c); err != nil {
		return wrapf("PostgresRepository.Create", err)
	}
	return nil
}

// Get implements Repository.
func (r *PostgresRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*Case, error) {
	const q = `
		SELECT id, tenant_id, jurisdiction_id, category_id, title, reference, state, metadata, metadata_version, created_by, created_at, updated_at, archived_at
		FROM cases
		WHERE id = $1 AND tenant_id = $2`

	c := &Case{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanCase(row, c); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, wrapf("PostgresRepository.Get", err)
	}
	return c, nil
}

// List implements Repository.
func (r *PostgresRepository) List(ctx context.Context, tenantID uuid.UUID, filter CaseFilter) ([]*Case, error) {
	q := `
		SELECT id, tenant_id, jurisdiction_id, category_id, title, reference, state, metadata, metadata_version, created_by, created_at, updated_at, archived_at
		FROM cases
		WHERE tenant_id = $1`
	args := []any{tenantID}

	if filter.State != "" {
		args = append(args, string(filter.State))
		q += fmt.Sprintf(" AND state = $%d", len(args))
	}
	if filter.JurisdictionID != uuid.Nil {
		args = append(args, filter.JurisdictionID)
		q += fmt.Sprintf(" AND jurisdiction_id = $%d", len(args))
	}
	if filter.CategoryID != "" {
		args = append(args, filter.CategoryID)
		q += fmt.Sprintf(" AND category_id = $%d", len(args))
	}
	q += " ORDER BY created_at ASC"

	rows, err := r.exec.Query(ctx, q, args...)
	if err != nil {
		return nil, wrapf("PostgresRepository.List", err)
	}
	defer rows.Close()

	var out []*Case
	for rows.Next() {
		c := &Case{}
		if err := scanCase(rows, c); err != nil {
			return nil, wrapf("PostgresRepository.List", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresRepository.List", err)
	}
	return out, nil
}

// Update implements Repository.
func (r *PostgresRepository) Update(ctx context.Context, tenantID uuid.UUID, c *Case) error {
	if c == nil {
		return wrapf("PostgresRepository.Update", ErrInvalidCase)
	}
	if err := requireMatchingTenant(tenantID, c.TenantID); err != nil {
		return err
	}

	metadata, err := marshalMetadata(c.Metadata)
	if err != nil {
		return wrapf("PostgresRepository.Update", err)
	}

	const q = `
		UPDATE cases
		SET jurisdiction_id = $3, category_id = $4, title = $5, reference = $6, state = $7,
		    metadata = $8, metadata_version = $9, updated_at = $10, archived_at = $11
		WHERE id = $1 AND tenant_id = $2
		RETURNING id, tenant_id, jurisdiction_id, category_id, title, reference, state, metadata, metadata_version, created_by, created_at, updated_at, archived_at`

	row := r.exec.QueryRow(ctx, q,
		c.ID, tenantID, c.JurisdictionID, c.CategoryID, c.Title, c.Reference,
		string(c.State), metadata, c.MetadataVersion, c.UpdatedAt, c.ArchivedAt,
	)
	if err := scanCase(row, c); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return wrapf("PostgresRepository.Update", err)
	}
	return nil
}

// AppendTransition implements Repository.
func (r *PostgresRepository) AppendTransition(ctx context.Context, tenantID uuid.UUID, rec *TransitionRecord) error {
	if rec == nil {
		return wrapf("PostgresRepository.AppendTransition", ErrInvalidCase)
	}
	if err := requireMatchingTenant(tenantID, rec.TenantID); err != nil {
		return err
	}
	if rec.ID == uuid.Nil {
		rec.ID = uuid.New()
	}

	const q = `
		INSERT INTO case_transitions (id, case_id, tenant_id, from_state, to_state, actor, reason, occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, case_id, tenant_id, from_state, to_state, actor, reason, occurred_at`

	row := r.exec.QueryRow(ctx, q,
		rec.ID, rec.CaseID, rec.TenantID, string(rec.FromState), string(rec.ToState), rec.Actor, rec.Reason, rec.OccurredAt,
	)
	if err := scanTransition(row, rec); err != nil {
		return wrapf("PostgresRepository.AppendTransition", err)
	}
	return nil
}

// ListTransitions implements Repository.
func (r *PostgresRepository) ListTransitions(ctx context.Context, tenantID, caseID uuid.UUID) ([]*TransitionRecord, error) {
	const q = `
		SELECT id, case_id, tenant_id, from_state, to_state, actor, reason, occurred_at
		FROM case_transitions
		WHERE case_id = $1 AND tenant_id = $2
		ORDER BY occurred_at ASC`

	rows, err := r.exec.Query(ctx, q, caseID, tenantID)
	if err != nil {
		return nil, wrapf("PostgresRepository.ListTransitions", err)
	}
	defer rows.Close()

	var out []*TransitionRecord
	for rows.Next() {
		rec := &TransitionRecord{}
		if err := scanTransition(rows, rec); err != nil {
			return nil, wrapf("PostgresRepository.ListTransitions", err)
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresRepository.ListTransitions", err)
	}
	return out, nil
}

// rowScanner is satisfied by both pgx.Row (QueryRow) and pgx.Rows
// (Query, iterated with Next), mirroring
// packages/persistence/tenant.go's rowScanner convention.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanCase(row rowScanner, c *Case) error {
	var state string
	var metadata []byte
	if err := row.Scan(
		&c.ID, &c.TenantID, &c.JurisdictionID, &c.CategoryID, &c.Title, &c.Reference,
		&state, &metadata, &c.MetadataVersion, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt, &c.ArchivedAt,
	); err != nil {
		return err
	}
	c.State = State(state)
	m, err := unmarshalMetadata(metadata)
	if err != nil {
		return err
	}
	c.Metadata = m
	return nil
}

func scanTransition(row rowScanner, rec *TransitionRecord) error {
	var fromState, toState string
	if err := row.Scan(
		&rec.ID, &rec.CaseID, &rec.TenantID, &fromState, &toState, &rec.Actor, &rec.Reason, &rec.OccurredAt,
	); err != nil {
		return err
	}
	rec.FromState = State(fromState)
	rec.ToState = State(toState)
	return nil
}

func marshalMetadata(m map[string]string) ([]byte, error) {
	if m == nil {
		m = map[string]string{}
	}
	return json.Marshal(m)
}

func unmarshalMetadata(b []byte) (map[string]string, error) {
	if len(b) == 0 {
		return map[string]string{}, nil
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]string{}
	}
	return m, nil
}

var _ Repository = (*PostgresRepository)(nil)
