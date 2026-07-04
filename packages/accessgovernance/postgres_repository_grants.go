package accessgovernance

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/YASSERRMD/verdex/packages/persistence"
)

// PostgresGrantRepository is a PostgreSQL-backed GrantRepository,
// storing Grant (JIT elevation) rows in the `access_elevation_grants`
// table.
type PostgresGrantRepository struct {
	exec persistence.Executor
}

// NewPostgresGrantRepository builds a PostgresGrantRepository bound to
// exec.
func NewPostgresGrantRepository(exec persistence.Executor) *PostgresGrantRepository {
	return &PostgresGrantRepository{exec: exec}
}

const elevationGrantColumns = `id, tenant_id, grantee_user_id, action, case_id, justification, granted_at, expires_at, requested_by, revoked_at`

func scanGrant(row rowScanner, g *Grant) error {
	var action string
	var caseID uuid.NullUUID
	if err := row.Scan(
		&g.ID, &g.TenantID, &g.GranteeUserID, &action, &caseID, &g.Justification,
		&g.GrantedAt, &g.ExpiresAt, &g.RequestedBy, &g.RevokedAt,
	); err != nil {
		return err
	}
	g.Action = Action(action)
	if caseID.Valid {
		g.CaseID = caseID.UUID
	}
	return nil
}

// Create implements GrantRepository.
func (r *PostgresGrantRepository) Create(ctx context.Context, tenantID uuid.UUID, g *Grant) error {
	if g == nil {
		return ErrInvalidGrant
	}
	if g.TenantID == uuid.Nil {
		g.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, g.TenantID); err != nil {
		return err
	}
	var caseID uuid.NullUUID
	if g.CaseID != uuid.Nil {
		caseID = uuid.NullUUID{UUID: g.CaseID, Valid: true}
	}

	q := `
		INSERT INTO access_elevation_grants (id, tenant_id, grantee_user_id, action, case_id, justification, granted_at, expires_at, requested_by, revoked_at)
		VALUES ($1, $2, $3, $4, $5, $6, COALESCE(NULLIF($7, TIMESTAMPTZ '0001-01-01'), now()), $8, $9, $10)
		RETURNING ` + elevationGrantColumns

	row := r.exec.QueryRow(ctx, q, g.ID, g.TenantID, g.GranteeUserID, string(g.Action), caseID, g.Justification, g.GrantedAt, g.ExpiresAt, g.RequestedBy, g.RevokedAt)
	if err := scanGrant(row, g); err != nil {
		return wrapf("PostgresGrantRepository.Create", err)
	}
	return nil
}

// Get implements GrantRepository.
func (r *PostgresGrantRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*Grant, error) {
	q := `SELECT ` + elevationGrantColumns + ` FROM access_elevation_grants WHERE id = $1 AND tenant_id = $2`
	g := &Grant{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanGrant(row, g); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrGrantNotFound
		}
		return nil, wrapf("PostgresGrantRepository.Get", err)
	}
	return g, nil
}

// ListActive implements GrantRepository.
func (r *PostgresGrantRepository) ListActive(ctx context.Context, tenantID uuid.UUID, now time.Time) ([]Grant, error) {
	q := `SELECT ` + elevationGrantColumns + ` FROM access_elevation_grants WHERE tenant_id = $1 AND expires_at > $2 AND revoked_at IS NULL`
	return r.query(ctx, q, tenantID, now)
}

// ListAll implements GrantRepository.
func (r *PostgresGrantRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]Grant, error) {
	q := `SELECT ` + elevationGrantColumns + ` FROM access_elevation_grants WHERE tenant_id = $1`
	return r.query(ctx, q, tenantID)
}

func (r *PostgresGrantRepository) query(ctx context.Context, q string, args ...any) ([]Grant, error) {
	rows, err := r.exec.Query(ctx, q, args...)
	if err != nil {
		return nil, wrapf("PostgresGrantRepository.query", err)
	}
	defer rows.Close()

	out := make([]Grant, 0)
	for rows.Next() {
		var g Grant
		if err := scanGrant(rows, &g); err != nil {
			return nil, wrapf("PostgresGrantRepository.query", err)
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresGrantRepository.query", err)
	}
	return out, nil
}

// Revoke implements GrantRepository.
func (r *PostgresGrantRepository) Revoke(ctx context.Context, tenantID, id uuid.UUID, revokedAt time.Time) error {
	const q = `UPDATE access_elevation_grants SET revoked_at = $1 WHERE id = $2 AND tenant_id = $3 AND revoked_at IS NULL`
	tag, err := r.exec.Exec(ctx, q, revokedAt, id, tenantID)
	if err != nil {
		return wrapf("PostgresGrantRepository.Revoke", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrGrantNotFound
	}
	return nil
}

var _ GrantRepository = (*PostgresGrantRepository)(nil)

// PostgresReviewRepository is a PostgreSQL-backed ReviewRepository,
// storing Review rows in the `access_reviews` table.
type PostgresReviewRepository struct {
	exec persistence.Executor
}

// NewPostgresReviewRepository builds a PostgresReviewRepository bound
// to exec.
func NewPostgresReviewRepository(exec persistence.Executor) *PostgresReviewRepository {
	return &PostgresReviewRepository{exec: exec}
}

const reviewColumns = `id, tenant_id, subject_kind, subject_id, requested_by, due_at, decision, attested_by, attested_at, notes, created_at`

func scanReview(row rowScanner, r *Review) error {
	var subjectKind, decision string
	var attestedBy uuid.NullUUID
	if err := row.Scan(
		&r.ID, &r.TenantID, &subjectKind, &r.SubjectID, &r.RequestedBy, &r.DueAt,
		&decision, &attestedBy, &r.AttestedAt, &r.Notes, &r.CreatedAt,
	); err != nil {
		return err
	}
	r.SubjectKind = GrantKind(subjectKind)
	r.Decision = AttestationDecision(decision)
	if attestedBy.Valid {
		r.AttestedBy = attestedBy.UUID
	}
	return nil
}

// Create implements ReviewRepository.
func (r *PostgresReviewRepository) Create(ctx context.Context, tenantID uuid.UUID, rv *Review) error {
	if rv == nil {
		return ErrReviewNotFound
	}
	if rv.TenantID == uuid.Nil {
		rv.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, rv.TenantID); err != nil {
		return err
	}
	var attestedBy uuid.NullUUID
	if rv.AttestedBy != uuid.Nil {
		attestedBy = uuid.NullUUID{UUID: rv.AttestedBy, Valid: true}
	}

	q := `
		INSERT INTO access_reviews (id, tenant_id, subject_kind, subject_id, requested_by, due_at, decision, attested_by, attested_at, notes, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, COALESCE(NULLIF($11, TIMESTAMPTZ '0001-01-01'), now()))
		RETURNING ` + reviewColumns

	row := r.exec.QueryRow(ctx, q, rv.ID, rv.TenantID, string(rv.SubjectKind), rv.SubjectID, rv.RequestedBy, rv.DueAt, string(rv.Decision), attestedBy, rv.AttestedAt, rv.Notes, rv.CreatedAt)
	if err := scanReview(row, rv); err != nil {
		return wrapf("PostgresReviewRepository.Create", err)
	}
	return nil
}

// Get implements ReviewRepository.
func (r *PostgresReviewRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*Review, error) {
	q := `SELECT ` + reviewColumns + ` FROM access_reviews WHERE id = $1 AND tenant_id = $2`
	rv := &Review{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanReview(row, rv); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReviewNotFound
		}
		return nil, wrapf("PostgresReviewRepository.Get", err)
	}
	return rv, nil
}

// ListDue implements ReviewRepository.
func (r *PostgresReviewRepository) ListDue(ctx context.Context, tenantID uuid.UUID, asOf time.Time) ([]Review, error) {
	q := `SELECT ` + reviewColumns + ` FROM access_reviews WHERE tenant_id = $1 AND decision = '' AND due_at <= $2`
	return r.query(ctx, q, tenantID, asOf)
}

// ListAll implements ReviewRepository.
func (r *PostgresReviewRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]Review, error) {
	q := `SELECT ` + reviewColumns + ` FROM access_reviews WHERE tenant_id = $1`
	return r.query(ctx, q, tenantID)
}

func (r *PostgresReviewRepository) query(ctx context.Context, q string, args ...any) ([]Review, error) {
	rows, err := r.exec.Query(ctx, q, args...)
	if err != nil {
		return nil, wrapf("PostgresReviewRepository.query", err)
	}
	defer rows.Close()

	out := make([]Review, 0)
	for rows.Next() {
		var rv Review
		if err := scanReview(rows, &rv); err != nil {
			return nil, wrapf("PostgresReviewRepository.query", err)
		}
		out = append(out, rv)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresReviewRepository.query", err)
	}
	return out, nil
}

// Update implements ReviewRepository.
func (r *PostgresReviewRepository) Update(ctx context.Context, tenantID uuid.UUID, rv *Review) error {
	if rv == nil {
		return ErrReviewNotFound
	}
	var attestedBy uuid.NullUUID
	if rv.AttestedBy != uuid.Nil {
		attestedBy = uuid.NullUUID{UUID: rv.AttestedBy, Valid: true}
	}

	const q = `
		UPDATE access_reviews SET decision = $1, attested_by = $2, attested_at = $3, notes = $4
		WHERE id = $5 AND tenant_id = $6`

	tag, err := r.exec.Exec(ctx, q, string(rv.Decision), attestedBy, rv.AttestedAt, rv.Notes, rv.ID, tenantID)
	if err != nil {
		return wrapf("PostgresReviewRepository.Update", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrReviewNotFound
	}
	return nil
}

var _ ReviewRepository = (*PostgresReviewRepository)(nil)
