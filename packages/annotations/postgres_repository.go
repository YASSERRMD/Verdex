package annotations

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/YASSERRMD/verdex/packages/persistence"
)

// PostgresRepository is a PostgreSQL-backed Repository, storing
// Annotation rows in the `annotations` table, Mention rows in the
// `annotation_mentions` table, and AuditRecord rows in the
// `annotation_audit_events` table (see
// packages/persistence/migrations/000012_create_annotations.up.sql).
// It accepts a persistence.Executor per call, mirroring
// packages/casesearch.PostgresRepository exactly, so callers can run
// it directly against a pool or compose it inside a transaction via
// persistence.WithTx or packages/tenancy.WithTenantScope.
type PostgresRepository struct {
	exec persistence.Executor
}

// NewPostgresRepository builds a PostgresRepository bound to exec.
func NewPostgresRepository(exec persistence.Executor) *PostgresRepository {
	return &PostgresRepository{exec: exec}
}

// rowScanner is satisfied by both pgx.Row (QueryRow) and pgx.Rows
// (Query, iterated with Next), mirroring
// packages/casesearch.PostgresRepository's rowScanner convention.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanAnnotation(row rowScanner, a *Annotation) error {
	var anchorType string
	if err := row.Scan(
		&a.ID, &a.TenantID, &a.CaseID, &a.AuthorID, &a.Body,
		&anchorType, &a.AnchorID, &a.ParentID,
		&a.Resolved, &a.ResolvedBy, &a.ResolvedAt,
		&a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return err
	}
	a.AnchorType = AnchorType(anchorType)
	return nil
}

// Create implements Repository.
func (r *PostgresRepository) Create(ctx context.Context, tenantID uuid.UUID, a *Annotation) error {
	if a == nil {
		return wrapf("PostgresRepository.Create", ErrNilAnnotation)
	}
	if a.TenantID == uuid.Nil {
		a.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, a.TenantID); err != nil {
		return err
	}
	if err := a.Validate(); err != nil {
		return err
	}

	if a.ParentID != nil {
		parent, err := r.Get(ctx, tenantID, *a.ParentID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return ErrParentNotFound
			}
			return wrapf("PostgresRepository.Create", err)
		}
		if parent.CaseID != a.CaseID {
			return ErrParentNotFound
		}
		if parent.IsReply() {
			return ErrParentIsReply
		}
	}

	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}

	const q = `
		INSERT INTO annotations (id, tenant_id, case_id, author_id, body, anchor_type, anchor_id, parent_id, resolved, resolved_by, resolved_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, false, NULL, NULL, now(), now())
		RETURNING id, tenant_id, case_id, author_id, body, anchor_type, anchor_id, parent_id, resolved, resolved_by, resolved_at, created_at, updated_at`

	row := r.exec.QueryRow(ctx, q,
		a.ID, a.TenantID, a.CaseID, a.AuthorID, a.Body, string(a.AnchorType), a.AnchorID, a.ParentID,
	)
	if err := scanAnnotation(row, a); err != nil {
		return wrapf("PostgresRepository.Create", err)
	}
	if err := r.insertMentions(ctx, a); err != nil {
		return wrapf("PostgresRepository.Create", err)
	}
	return nil
}

// insertMentions parses a.Body for "@<userID>" tokens and inserts a
// Mention row for each.
func (r *PostgresRepository) insertMentions(ctx context.Context, a *Annotation) error {
	mentioned := ExtractMentions(a.Body)
	if len(mentioned) == 0 {
		return nil
	}
	const q = `
		INSERT INTO annotation_mentions (id, annotation_id, case_id, tenant_id, author_id, mentioned_user_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	for _, userID := range mentioned {
		if _, err := r.exec.Exec(ctx, q, uuid.New(), a.ID, a.CaseID, a.TenantID, a.AuthorID, userID, a.UpdatedAt); err != nil {
			return err
		}
	}
	return nil
}

// Get implements Repository.
func (r *PostgresRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*Annotation, error) {
	const q = `
		SELECT id, tenant_id, case_id, author_id, body, anchor_type, anchor_id, parent_id, resolved, resolved_by, resolved_at, created_at, updated_at
		FROM annotations
		WHERE id = $1 AND tenant_id = $2`

	a := &Annotation{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanAnnotation(row, a); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, wrapf("PostgresRepository.Get", err)
	}
	return a, nil
}

// ListByCase implements Repository.
func (r *PostgresRepository) ListByCase(ctx context.Context, tenantID, caseID uuid.UUID, filter AnchorFilter) ([]*Annotation, error) {
	q := `
		SELECT id, tenant_id, case_id, author_id, body, anchor_type, anchor_id, parent_id, resolved, resolved_by, resolved_at, created_at, updated_at
		FROM annotations
		WHERE case_id = $1 AND tenant_id = $2`
	args := []any{caseID, tenantID}

	if filter.Type != "" {
		args = append(args, string(filter.Type))
		q += fmt.Sprintf(" AND anchor_type = $%d", len(args))
		if filter.ID != "" {
			args = append(args, filter.ID)
			q += fmt.Sprintf(" AND anchor_id = $%d", len(args))
		}
	}
	q += " ORDER BY created_at ASC"

	rows, err := r.exec.Query(ctx, q, args...)
	if err != nil {
		return nil, wrapf("PostgresRepository.ListByCase", err)
	}
	defer rows.Close()

	var out []*Annotation
	for rows.Next() {
		a := &Annotation{}
		if err := scanAnnotation(rows, a); err != nil {
			return nil, wrapf("PostgresRepository.ListByCase", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresRepository.ListByCase", err)
	}
	return out, nil
}

// Thread implements Repository.
func (r *PostgresRepository) Thread(ctx context.Context, tenantID, rootID uuid.UUID) ([]*Annotation, error) {
	root, err := r.Get(ctx, tenantID, rootID)
	if err != nil {
		return nil, err
	}

	const q = `
		SELECT id, tenant_id, case_id, author_id, body, anchor_type, anchor_id, parent_id, resolved, resolved_by, resolved_at, created_at, updated_at
		FROM annotations
		WHERE parent_id = $1 AND tenant_id = $2
		ORDER BY created_at ASC`

	rows, err := r.exec.Query(ctx, q, rootID, tenantID)
	if err != nil {
		return nil, wrapf("PostgresRepository.Thread", err)
	}
	defer rows.Close()

	out := []*Annotation{root}
	for rows.Next() {
		a := &Annotation{}
		if err := scanAnnotation(rows, a); err != nil {
			return nil, wrapf("PostgresRepository.Thread", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresRepository.Thread", err)
	}
	return out, nil
}

// UpdateBody implements Repository.
func (r *PostgresRepository) UpdateBody(ctx context.Context, tenantID, id uuid.UUID, body string) (*Annotation, error) {
	const q = `
		UPDATE annotations
		SET body = $3, updated_at = now()
		WHERE id = $1 AND tenant_id = $2
		RETURNING id, tenant_id, case_id, author_id, body, anchor_type, anchor_id, parent_id, resolved, resolved_by, resolved_at, created_at, updated_at`

	a := &Annotation{}
	row := r.exec.QueryRow(ctx, q, id, tenantID, body)
	if err := scanAnnotation(row, a); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, wrapf("PostgresRepository.UpdateBody", err)
	}
	if err := r.insertMentions(ctx, a); err != nil {
		return nil, wrapf("PostgresRepository.UpdateBody", err)
	}
	return a, nil
}

// Delete implements Repository. Deleting a thread root cascades to its
// replies via the `parent_id` FK's ON DELETE CASCADE.
func (r *PostgresRepository) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	const q = `DELETE FROM annotations WHERE id = $1 AND tenant_id = $2`

	tag, err := r.exec.Exec(ctx, q, id, tenantID)
	if err != nil {
		return wrapf("PostgresRepository.Delete", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Resolve implements Repository.
func (r *PostgresRepository) Resolve(ctx context.Context, tenantID, id, resolvedBy uuid.UUID) (*Annotation, error) {
	existing, err := r.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if existing.Resolved {
		return nil, ErrAlreadyResolved
	}

	const q = `
		UPDATE annotations
		SET resolved = true, resolved_by = $3, resolved_at = now(), updated_at = now()
		WHERE id = $1 AND tenant_id = $2
		RETURNING id, tenant_id, case_id, author_id, body, anchor_type, anchor_id, parent_id, resolved, resolved_by, resolved_at, created_at, updated_at`

	a := &Annotation{}
	row := r.exec.QueryRow(ctx, q, id, tenantID, resolvedBy)
	if err := scanAnnotation(row, a); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, wrapf("PostgresRepository.Resolve", err)
	}
	return a, nil
}

// Reopen implements Repository.
func (r *PostgresRepository) Reopen(ctx context.Context, tenantID, id uuid.UUID) (*Annotation, error) {
	existing, err := r.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if !existing.Resolved {
		return nil, ErrNotResolved
	}

	const q = `
		UPDATE annotations
		SET resolved = false, resolved_by = NULL, resolved_at = NULL, updated_at = now()
		WHERE id = $1 AND tenant_id = $2
		RETURNING id, tenant_id, case_id, author_id, body, anchor_type, anchor_id, parent_id, resolved, resolved_by, resolved_at, created_at, updated_at`

	a := &Annotation{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanAnnotation(row, a); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, wrapf("PostgresRepository.Reopen", err)
	}
	return a, nil
}

// MentionsFor implements Repository.
func (r *PostgresRepository) MentionsFor(ctx context.Context, tenantID, userID uuid.UUID) ([]Mention, error) {
	const q = `
		SELECT annotation_id, case_id, tenant_id, author_id, mentioned_user_id, created_at
		FROM annotation_mentions
		WHERE tenant_id = $1 AND mentioned_user_id = $2
		ORDER BY created_at DESC`

	rows, err := r.exec.Query(ctx, q, tenantID, userID)
	if err != nil {
		return nil, wrapf("PostgresRepository.MentionsFor", err)
	}
	defer rows.Close()

	out := make([]Mention, 0)
	for rows.Next() {
		var m Mention
		if err := rows.Scan(&m.AnnotationID, &m.CaseID, &m.TenantID, &m.AuthorID, &m.MentionedUserID, &m.CreatedAt); err != nil {
			return nil, wrapf("PostgresRepository.MentionsFor", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresRepository.MentionsFor", err)
	}
	return out, nil
}

// AppendAudit implements Repository.
func (r *PostgresRepository) AppendAudit(ctx context.Context, tenantID uuid.UUID, rec *AuditRecord) error {
	if rec == nil {
		return wrapf("PostgresRepository.AppendAudit", ErrNilAnnotation)
	}
	if err := requireMatchingTenant(tenantID, rec.TenantID); err != nil {
		return err
	}
	if rec.ID == uuid.Nil {
		rec.ID = uuid.New()
	}

	const q = `
		INSERT INTO annotation_audit_events (id, annotation_id, case_id, tenant_id, verb, actor, occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	if _, err := r.exec.Exec(ctx, q, rec.ID, rec.AnnotationID, rec.CaseID, rec.TenantID, string(rec.Verb), rec.Actor, rec.OccurredAt); err != nil {
		return wrapf("PostgresRepository.AppendAudit", err)
	}
	return nil
}

// ListAudit implements Repository.
func (r *PostgresRepository) ListAudit(ctx context.Context, tenantID, annotationID uuid.UUID) ([]*AuditRecord, error) {
	const q = `
		SELECT id, annotation_id, case_id, tenant_id, verb, actor, occurred_at
		FROM annotation_audit_events
		WHERE annotation_id = $1 AND tenant_id = $2
		ORDER BY occurred_at ASC`

	rows, err := r.exec.Query(ctx, q, annotationID, tenantID)
	if err != nil {
		return nil, wrapf("PostgresRepository.ListAudit", err)
	}
	defer rows.Close()

	var out []*AuditRecord
	for rows.Next() {
		rec := &AuditRecord{}
		var verb string
		if err := rows.Scan(&rec.ID, &rec.AnnotationID, &rec.CaseID, &rec.TenantID, &verb, &rec.Actor, &rec.OccurredAt); err != nil {
			return nil, wrapf("PostgresRepository.ListAudit", err)
		}
		rec.Verb = AuditVerb(verb)
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresRepository.ListAudit", err)
	}
	return out, nil
}

var _ Repository = (*PostgresRepository)(nil)
