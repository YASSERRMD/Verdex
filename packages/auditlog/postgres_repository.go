package auditlog

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/YASSERRMD/verdex/packages/persistence"
)

// PostgresRepository is a PostgreSQL-backed Repository, storing Event
// rows in the `audit_events` table (see
// packages/persistence/migrations/000020_create_auditlog.up.sql). It
// accepts a persistence.Executor per call, mirroring
// packages/keymanagement.PostgresRepository exactly, so callers can
// run it directly against a pool or compose it inside a transaction
// via persistence.WithTx or packages/tenancy.WithTenantScope.
type PostgresRepository struct {
	exec persistence.Executor
}

// NewPostgresRepository builds a PostgresRepository bound to exec.
func NewPostgresRepository(exec persistence.Executor) *PostgresRepository {
	return &PostgresRepository{exec: exec}
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEvent(row rowScanner, e *Event) error {
	var caseID uuid.NullUUID
	if err := row.Scan(
		&e.ID, &e.TenantID, &e.Time, &e.Actor, &e.Action, &e.Target, &e.Outcome,
		&e.Kind, &caseID, &e.Detail, &e.PrevHash, &e.ChainHash,
	); err != nil {
		return err
	}
	if caseID.Valid {
		e.CaseID = caseID.UUID
	}
	return nil
}

// Append implements Repository.
func (r *PostgresRepository) Append(ctx context.Context, tenantID uuid.UUID, event *Event) error {
	if event == nil {
		return wrapf("PostgresRepository.Append", ErrNilEvent)
	}
	if event.TenantID == uuid.Nil {
		event.TenantID = tenantID
	}
	if event.TenantID != tenantID {
		return wrapf("PostgresRepository.Append", ErrCrossTenantAccess)
	}
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}

	var caseID *uuid.UUID
	if event.CaseID != uuid.Nil {
		caseID = &event.CaseID
	}

	const q = `
		INSERT INTO audit_events
			(id, tenant_id, occurred_at, actor, action, target, outcome, kind, case_id, detail, prev_hash, chain_hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	_, err := r.exec.Exec(ctx, q,
		event.ID, event.TenantID, event.Time, event.Actor, event.Action, event.Target, event.Outcome,
		string(event.Kind), caseID, event.Detail, event.PrevHash, event.ChainHash,
	)
	if err != nil {
		return wrapf("PostgresRepository.Append", err)
	}
	return nil
}

// Last implements Repository.
func (r *PostgresRepository) Last(ctx context.Context, tenantID uuid.UUID) (*Event, error) {
	const q = `
		SELECT id, tenant_id, occurred_at, actor, action, target, outcome, kind, case_id, detail, prev_hash, chain_hash
		FROM audit_events
		WHERE tenant_id = $1
		ORDER BY occurred_at DESC, id DESC
		LIMIT 1`

	e := &Event{}
	row := r.exec.QueryRow(ctx, q, tenantID)
	if err := scanEvent(row, e); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, wrapf("PostgresRepository.Last", err)
	}
	return e, nil
}

// ListAll implements Repository.
func (r *PostgresRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]Event, error) {
	const q = `
		SELECT id, tenant_id, occurred_at, actor, action, target, outcome, kind, case_id, detail, prev_hash, chain_hash
		FROM audit_events
		WHERE tenant_id = $1
		ORDER BY occurred_at ASC, id ASC`

	return r.queryRows(ctx, q, tenantID)
}

// Query implements Repository.
func (r *PostgresRepository) Query(ctx context.Context, tenantID uuid.UUID, filter Filter) ([]Event, error) {
	q := `
		SELECT id, tenant_id, occurred_at, actor, action, target, outcome, kind, case_id, detail, prev_hash, chain_hash
		FROM audit_events
		WHERE tenant_id = $1`
	args := []any{tenantID}

	if filter.Actor != "" {
		args = append(args, filter.Actor)
		q += " AND actor = $" + strconv.Itoa(len(args))
	}
	if filter.CaseID != uuid.Nil {
		args = append(args, filter.CaseID)
		q += " AND case_id = $" + strconv.Itoa(len(args))
	}
	if filter.Action != "" {
		args = append(args, filter.Action)
		q += " AND action = $" + strconv.Itoa(len(args))
	}
	if len(filter.Kinds) > 0 {
		kinds := make([]string, len(filter.Kinds))
		for i, k := range filter.Kinds {
			kinds[i] = string(k)
		}
		args = append(args, kinds)
		q += " AND kind = ANY($" + strconv.Itoa(len(args)) + ")"
	}
	if !filter.Since.IsZero() {
		args = append(args, filter.Since)
		q += " AND occurred_at >= $" + strconv.Itoa(len(args))
	}
	if !filter.Until.IsZero() {
		args = append(args, filter.Until)
		q += " AND occurred_at <= $" + strconv.Itoa(len(args))
	}

	q += " ORDER BY occurred_at ASC, id ASC"

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultQueryLimit
	}
	args = append(args, limit)
	q += " LIMIT $" + strconv.Itoa(len(args))

	return r.queryRows(ctx, q, args...)
}

func (r *PostgresRepository) queryRows(ctx context.Context, q string, args ...any) ([]Event, error) {
	rows, err := r.exec.Query(ctx, q, args...)
	if err != nil {
		return nil, wrapf("PostgresRepository.queryRows", err)
	}
	defer rows.Close()

	out := make([]Event, 0)
	for rows.Next() {
		e := Event{}
		if err := scanEvent(rows, &e); err != nil {
			return nil, wrapf("PostgresRepository.queryRows", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresRepository.queryRows", err)
	}
	return out, nil
}

// PurgeBefore implements Repository.
func (r *PostgresRepository) PurgeBefore(ctx context.Context, tenantID uuid.UUID, cutoff time.Time) (int, error) {
	const q = `DELETE FROM audit_events WHERE tenant_id = $1 AND occurred_at < $2`

	tag, err := r.exec.Exec(ctx, q, tenantID, cutoff)
	if err != nil {
		return 0, wrapf("PostgresRepository.PurgeBefore", err)
	}
	return int(tag.RowsAffected()), nil
}

var _ Repository = (*PostgresRepository)(nil)
