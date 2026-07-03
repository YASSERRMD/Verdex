package notifications

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/YASSERRMD/verdex/packages/persistence"
)

// PostgresPreferenceRepository is a PostgreSQL-backed
// PreferenceRepository, storing rows in the
// `notification_preferences` table (see
// packages/persistence/migrations/000016_create_notifications.up.sql).
// It accepts a persistence.Executor per call, mirroring
// PostgresRepository exactly.
type PostgresPreferenceRepository struct {
	exec persistence.Executor
}

// NewPostgresPreferenceRepository builds a
// PostgresPreferenceRepository bound to exec.
func NewPostgresPreferenceRepository(exec persistence.Executor) *PostgresPreferenceRepository {
	return &PostgresPreferenceRepository{exec: exec}
}

func scanPreference(row rowScanner, p *Preference) error {
	var kind string
	var channels []string
	if err := row.Scan(&p.TenantID, &p.UserID, &kind, &p.Enabled, &channels); err != nil {
		return err
	}
	p.Kind = Kind(kind)
	p.Channels = make([]Channel, len(channels))
	for i, c := range channels {
		p.Channels[i] = Channel(c)
	}
	return nil
}

func channelsToStrings(channels []Channel) []string {
	out := make([]string, len(channels))
	for i, c := range channels {
		out[i] = string(c)
	}
	return out
}

// Upsert implements PreferenceRepository.
func (r *PostgresPreferenceRepository) Upsert(ctx context.Context, tenantID uuid.UUID, p *Preference) error {
	if p == nil {
		return wrapf("PostgresPreferenceRepository.Upsert", ErrNilPreference)
	}
	if p.TenantID == uuid.Nil {
		p.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, p.TenantID); err != nil {
		return err
	}
	if err := p.Validate(); err != nil {
		return err
	}

	const q = `
		INSERT INTO notification_preferences (tenant_id, user_id, kind, enabled, channels)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (tenant_id, user_id, kind)
		DO UPDATE SET enabled = EXCLUDED.enabled, channels = EXCLUDED.channels
		RETURNING tenant_id, user_id, kind, enabled, channels`

	row := r.exec.QueryRow(ctx, q, p.TenantID, p.UserID, string(p.Kind), p.Enabled, channelsToStrings(p.Channels))
	if err := scanPreference(row, p); err != nil {
		return wrapf("PostgresPreferenceRepository.Upsert", err)
	}
	return nil
}

// Get implements PreferenceRepository.
func (r *PostgresPreferenceRepository) Get(ctx context.Context, tenantID, userID uuid.UUID, kind Kind) (*Preference, error) {
	const q = `
		SELECT tenant_id, user_id, kind, enabled, channels
		FROM notification_preferences
		WHERE tenant_id = $1 AND user_id = $2 AND kind = $3`

	p := &Preference{}
	row := r.exec.QueryRow(ctx, q, tenantID, userID, string(kind))
	if err := scanPreference(row, p); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, wrapf("PostgresPreferenceRepository.Get", err)
	}
	return p, nil
}

// ListForUser implements PreferenceRepository.
func (r *PostgresPreferenceRepository) ListForUser(ctx context.Context, tenantID, userID uuid.UUID) ([]*Preference, error) {
	const q = `
		SELECT tenant_id, user_id, kind, enabled, channels
		FROM notification_preferences
		WHERE tenant_id = $1 AND user_id = $2
		ORDER BY kind ASC`

	rows, err := r.exec.Query(ctx, q, tenantID, userID)
	if err != nil {
		return nil, wrapf("PostgresPreferenceRepository.ListForUser", err)
	}
	defer rows.Close()

	out := make([]*Preference, 0)
	for rows.Next() {
		p := &Preference{}
		if err := scanPreference(rows, p); err != nil {
			return nil, wrapf("PostgresPreferenceRepository.ListForUser", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresPreferenceRepository.ListForUser", err)
	}
	return out, nil
}

var _ PreferenceRepository = (*PostgresPreferenceRepository)(nil)
