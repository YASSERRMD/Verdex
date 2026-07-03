package caseversioning

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/YASSERRMD/verdex/packages/persistence"
)

// PostgresRepository is a PostgreSQL-backed Repository, storing
// Snapshot rows in the `case_version_snapshots` table (see
// packages/persistence/migrations/000014_create_case_version_snapshots.up.sql).
// It accepts a persistence.Executor per call, mirroring
// packages/annotations.PostgresRepository exactly, so callers can run it
// directly against a pool or compose it inside a transaction via
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
// packages/annotations.PostgresRepository's rowScanner convention.
type rowScanner interface {
	Scan(dest ...any) error
}

// encodePayload marshals s.Payload (nil, CaseMetadataPayload, or
// OpinionPayload) into the JSONB column value, tagging the wire
// encoding with which concrete type it is so decodePayload can rebuild
// the exact Go type on read — Payload is declared `any` on Snapshot
// precisely because it varies per ArtifactKind.
type payloadEnvelope struct {
	Kind string          `json:"kind"`
	Data json.RawMessage `json:"data"`
}

func encodePayload(kind ArtifactKind, payload any) ([]byte, error) {
	if payload == nil {
		return nil, nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(payloadEnvelope{Kind: string(kind), Data: data})
}

// decodePayload rebuilds the exact Go type a Snapshot's Payload held
// before persistence, using the kind tag embedded in the envelope
// itself (written by encodePayload) rather than requiring the caller to
// pass ArtifactKind back in — the raw bytes are self-describing.
func decodePayload(raw []byte) (any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var env payloadEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	switch ArtifactKind(env.Kind) {
	case ArtifactCaseMetadata:
		var p CaseMetadataPayload
		if err := json.Unmarshal(env.Data, &p); err != nil {
			return nil, err
		}
		return p, nil
	case ArtifactOpinion:
		var p OpinionPayload
		if err := json.Unmarshal(env.Data, &p); err != nil {
			return nil, err
		}
		return p, nil
	default:
		return nil, nil
	}
}

func scanSnapshot(row rowScanner, s *Snapshot) error {
	var artifactKind string
	var payloadRaw []byte
	if err := row.Scan(
		&s.ID, &s.TenantID, &s.CaseID, &artifactKind, &s.ArtifactRevisionRef,
		&payloadRaw, &s.CreatedBy, &s.Reason, &s.Label, &s.RestoredFromID, &s.CreatedAt,
	); err != nil {
		return err
	}
	s.ArtifactKind = ArtifactKind(artifactKind)
	payload, err := decodePayload(payloadRaw)
	if err != nil {
		return err
	}
	s.Payload = payload
	return nil
}

// Create implements Repository.
func (r *PostgresRepository) Create(ctx context.Context, tenantID uuid.UUID, s *Snapshot) error {
	if s == nil {
		return wrapf("PostgresRepository.Create", ErrNilSnapshot)
	}
	if s.TenantID == uuid.Nil {
		s.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, s.TenantID); err != nil {
		return err
	}
	if err := s.Validate(); err != nil {
		return err
	}
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}

	payloadJSON, err := encodePayload(s.ArtifactKind, s.Payload)
	if err != nil {
		return wrapf("PostgresRepository.Create", err)
	}

	const q = `
		INSERT INTO case_version_snapshots (id, tenant_id, case_id, artifact_kind, artifact_revision_ref, payload, created_by, reason, label, restored_from_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, now())
		RETURNING id, tenant_id, case_id, artifact_kind, artifact_revision_ref, payload, created_by, reason, label, restored_from_id, created_at`

	row := r.exec.QueryRow(ctx, q,
		s.ID, s.TenantID, s.CaseID, string(s.ArtifactKind), s.ArtifactRevisionRef,
		payloadJSON, s.CreatedBy, s.Reason, s.Label, s.RestoredFromID,
	)
	if err := scanSnapshot(row, s); err != nil {
		return wrapf("PostgresRepository.Create", err)
	}
	return nil
}

// Get implements Repository.
func (r *PostgresRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*Snapshot, error) {
	const q = `
		SELECT id, tenant_id, case_id, artifact_kind, artifact_revision_ref, payload, created_by, reason, label, restored_from_id, created_at
		FROM case_version_snapshots
		WHERE id = $1 AND tenant_id = $2`

	s := &Snapshot{}
	row := r.exec.QueryRow(ctx, q, id, tenantID)
	if err := scanSnapshot(row, s); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, wrapf("PostgresRepository.Get", err)
	}
	return s, nil
}

// ListByCase implements Repository.
func (r *PostgresRepository) ListByCase(ctx context.Context, tenantID, caseID uuid.UUID, filter SnapshotFilter) ([]*Snapshot, error) {
	q := `
		SELECT id, tenant_id, case_id, artifact_kind, artifact_revision_ref, payload, created_by, reason, label, restored_from_id, created_at
		FROM case_version_snapshots
		WHERE case_id = $1 AND tenant_id = $2`
	args := []any{caseID, tenantID}

	if filter.Kind != "" {
		args = append(args, string(filter.Kind))
		q += " AND artifact_kind = $3"
	}
	q += " ORDER BY created_at ASC"

	rows, err := r.exec.Query(ctx, q, args...)
	if err != nil {
		return nil, wrapf("PostgresRepository.ListByCase", err)
	}
	defer rows.Close()

	out := make([]*Snapshot, 0)
	for rows.Next() {
		s := &Snapshot{}
		if err := scanSnapshot(rows, s); err != nil {
			return nil, wrapf("PostgresRepository.ListByCase", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresRepository.ListByCase", err)
	}
	return out, nil
}

// Latest implements Repository.
func (r *PostgresRepository) Latest(ctx context.Context, tenantID, caseID uuid.UUID, kind ArtifactKind) (*Snapshot, error) {
	const q = `
		SELECT id, tenant_id, case_id, artifact_kind, artifact_revision_ref, payload, created_by, reason, label, restored_from_id, created_at
		FROM case_version_snapshots
		WHERE case_id = $1 AND tenant_id = $2 AND artifact_kind = $3
		ORDER BY created_at DESC
		LIMIT 1`

	s := &Snapshot{}
	row := r.exec.QueryRow(ctx, q, caseID, tenantID, string(kind))
	if err := scanSnapshot(row, s); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, wrapf("PostgresRepository.Latest", err)
	}
	return s, nil
}

var _ Repository = (*PostgresRepository)(nil)
