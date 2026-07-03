package notifications

import (
	"context"
	"errors"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/YASSERRMD/verdex/packages/persistence"
)

// PostgresRepository is a PostgreSQL-backed Repository, storing
// Notification rows in the `notifications` table (see
// packages/persistence/migrations/000016_create_notifications.up.sql).
// It accepts a persistence.Executor per call, mirroring
// packages/caseversioning.PostgresRepository exactly, so callers can
// run it directly against a pool or compose it inside a transaction
// via persistence.WithTx or packages/tenancy.WithTenantScope.
type PostgresRepository struct {
	exec persistence.Executor
}

// NewPostgresRepository builds a PostgresRepository bound to exec.
func NewPostgresRepository(exec persistence.Executor) *PostgresRepository {
	return &PostgresRepository{exec: exec}
}

// rowScanner is satisfied by both pgx.Row (QueryRow) and pgx.Rows
// (Query, iterated with Next).
type rowScanner interface {
	Scan(dest ...any) error
}

func scanNotification(row rowScanner, n *Notification) error {
	var kind string
	if err := row.Scan(
		&n.ID, &n.TenantID, &n.RecipientID, &kind, &n.Title, &n.Body,
		&n.CaseID, &n.RelatedEntityID, &n.CreatedAt, &n.ReadAt,
	); err != nil {
		return err
	}
	n.Kind = Kind(kind)
	return nil
}

// Create implements Repository.
func (r *PostgresRepository) Create(ctx context.Context, tenantID uuid.UUID, n *Notification) error {
	if n == nil {
		return wrapf("PostgresRepository.Create", ErrNilNotification)
	}
	if n.TenantID == uuid.Nil {
		n.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, n.TenantID); err != nil {
		return err
	}
	if err := n.Validate(); err != nil {
		return err
	}
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}

	const q = `
		INSERT INTO notifications (id, tenant_id, recipient_id, kind, title, body, case_id, related_entity_id, created_at, read_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now(), $9)
		RETURNING id, tenant_id, recipient_id, kind, title, body, case_id, related_entity_id, created_at, read_at`

	row := r.exec.QueryRow(ctx, q,
		n.ID, n.TenantID, n.RecipientID, string(n.Kind), n.Title, n.Body,
		n.CaseID, n.RelatedEntityID, n.ReadAt,
	)
	if err := scanNotification(row, n); err != nil {
		return wrapf("PostgresRepository.Create", err)
	}
	return nil
}

// Get implements Repository.
func (r *PostgresRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*Notification, error) {
	const q = `
		SELECT id, tenant_id, recipient_id, kind, title, body, case_id, related_entity_id, created_at, read_at
		FROM notifications
		WHERE id = $1 AND tenant_id = $2`

	n := &Notification{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanNotification(row, n); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, wrapf("PostgresRepository.Get", err)
	}
	return n, nil
}

// ListForRecipient implements Repository.
func (r *PostgresRepository) ListForRecipient(ctx context.Context, tenantID, recipientID uuid.UUID, filter Filter) ([]*Notification, error) {
	q := `
		SELECT id, tenant_id, recipient_id, kind, title, body, case_id, related_entity_id, created_at, read_at
		FROM notifications
		WHERE tenant_id = $1 AND recipient_id = $2`
	args := []any{tenantID, recipientID}

	if filter.UnreadOnly {
		q += " AND read_at IS NULL"
	}
	if filter.Kind != "" {
		args = append(args, string(filter.Kind))
		q += " AND kind = $" + strconv.Itoa(len(args))
	}
	q += " ORDER BY created_at DESC"
	if filter.Limit > 0 {
		args = append(args, filter.Limit)
		q += " LIMIT $" + strconv.Itoa(len(args))
	}

	rows, err := r.exec.Query(ctx, q, args...)
	if err != nil {
		return nil, wrapf("PostgresRepository.ListForRecipient", err)
	}
	defer rows.Close()

	out := make([]*Notification, 0)
	for rows.Next() {
		n := &Notification{}
		if err := scanNotification(rows, n); err != nil {
			return nil, wrapf("PostgresRepository.ListForRecipient", err)
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresRepository.ListForRecipient", err)
	}
	return out, nil
}

// UnreadCount implements Repository.
func (r *PostgresRepository) UnreadCount(ctx context.Context, tenantID, recipientID uuid.UUID) (int, error) {
	const q = `
		SELECT count(*) FROM notifications
		WHERE tenant_id = $1 AND recipient_id = $2 AND read_at IS NULL`

	var count int
	row := r.exec.QueryRow(ctx, q, tenantID, recipientID)
	if err := row.Scan(&count); err != nil {
		return 0, wrapf("PostgresRepository.UnreadCount", err)
	}
	return count, nil
}

// MarkRead implements Repository.
func (r *PostgresRepository) MarkRead(ctx context.Context, tenantID, recipientID, id uuid.UUID) error {
	const q = `
		UPDATE notifications SET read_at = now()
		WHERE id = $1 AND tenant_id = $2 AND recipient_id = $3 AND read_at IS NULL`

	tag, err := r.exec.Exec(ctx, q, id, tenantID, recipientID)
	if err != nil {
		return wrapf("PostgresRepository.MarkRead", err)
	}
	if tag.RowsAffected() == 0 {
		// Either the row does not exist / is not visible, or it was
		// already read — distinguish the two so MarkRead stays
		// idempotent for the common "already read" case.
		if _, err := r.Get(ctx, tenantID, id); err != nil {
			return err
		}
	}
	return nil
}

// MarkAllRead implements Repository.
func (r *PostgresRepository) MarkAllRead(ctx context.Context, tenantID, recipientID uuid.UUID) (int, error) {
	const q = `
		UPDATE notifications SET read_at = now()
		WHERE tenant_id = $1 AND recipient_id = $2 AND read_at IS NULL`

	tag, err := r.exec.Exec(ctx, q, tenantID, recipientID)
	if err != nil {
		return 0, wrapf("PostgresRepository.MarkAllRead", err)
	}
	return int(tag.RowsAffected()), nil
}

var _ Repository = (*PostgresRepository)(nil)
