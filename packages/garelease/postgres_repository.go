package garelease

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/YASSERRMD/verdex/packages/persistence"
)

// rowScanner is the subset of pgx.Row / pgx.Rows this file's scan
// helpers depend on, mirroring packages/compliance.rowScanner and
// packages/privacy.rowScanner exactly.
type rowScanner interface {
	Scan(dest ...any) error
}

// PostgresReleaseCandidateRepository is a PostgreSQL-backed
// ReleaseCandidateRepository, storing ReleaseCandidate rows in the
// `garelease_candidates` table (see
// packages/persistence/migrations/000046_create_garelease.up.sql).
// Like packages/compliance.PostgresControlRepository, this repository
// is never wrapped by packages/tenancy.WithTenantScope:
// garelease_candidates carries no tenant_id column and no RLS policy,
// since a ReleaseCandidate is shared, platform-global data (see
// types.go's ReleaseCandidate doc comment). It accepts a
// persistence.Executor per call, mirroring
// packages/compliance.PostgresControlRepository exactly.
type PostgresReleaseCandidateRepository struct {
	exec persistence.Executor
}

// NewPostgresReleaseCandidateRepository builds a
// PostgresReleaseCandidateRepository bound to exec.
func NewPostgresReleaseCandidateRepository(exec persistence.Executor) *PostgresReleaseCandidateRepository {
	return &PostgresReleaseCandidateRepository{exec: exec}
}

const candidateColumns = `id, version, commit_sha, readiness, frozen_by, frozen_at`

func scanCandidate(row rowScanner, c *ReleaseCandidate) error {
	var readinessJSON []byte
	if err := row.Scan(&c.ID, &c.Version, &c.CommitSHA, &readinessJSON, &c.FrozenBy, &c.FrozenAt); err != nil {
		return err
	}
	if len(readinessJSON) > 0 {
		if err := json.Unmarshal(readinessJSON, &c.Readiness); err != nil {
			return err
		}
	}
	return nil
}

// Create implements ReleaseCandidateRepository.
func (r *PostgresReleaseCandidateRepository) Create(ctx context.Context, c *ReleaseCandidate) error {
	if c == nil {
		return ErrInvalidCandidate
	}
	readinessJSON, err := json.Marshal(c.Readiness)
	if err != nil {
		return wrapf("PostgresReleaseCandidateRepository.Create", err)
	}

	q := `
		INSERT INTO garelease_candidates (id, version, commit_sha, readiness, frozen_by, frozen_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING ` + candidateColumns

	row := r.exec.QueryRow(ctx, q, c.ID, c.Version, c.CommitSHA, readinessJSON, c.FrozenBy, c.FrozenAt)
	if err := scanCandidate(row, c); err != nil {
		if isUniqueViolation(err) {
			return wrapf("PostgresReleaseCandidateRepository.Create", ErrInvalidCandidate)
		}
		return wrapf("PostgresReleaseCandidateRepository.Create", err)
	}
	return nil
}

// Get implements ReleaseCandidateRepository.
func (r *PostgresReleaseCandidateRepository) Get(ctx context.Context, id uuid.UUID) (*ReleaseCandidate, error) {
	q := `SELECT ` + candidateColumns + ` FROM garelease_candidates WHERE id = $1`
	c := &ReleaseCandidate{}
	row := r.exec.QueryRow(ctx, q, id)
	if err := scanCandidate(row, c); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCandidateNotFound
		}
		return nil, wrapf("PostgresReleaseCandidateRepository.Get", err)
	}
	return c, nil
}

// GetByVersion implements ReleaseCandidateRepository.
func (r *PostgresReleaseCandidateRepository) GetByVersion(ctx context.Context, version string) (*ReleaseCandidate, error) {
	q := `SELECT ` + candidateColumns + ` FROM garelease_candidates WHERE version = $1`
	c := &ReleaseCandidate{}
	row := r.exec.QueryRow(ctx, q, version)
	if err := scanCandidate(row, c); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCandidateNotFound
		}
		return nil, wrapf("PostgresReleaseCandidateRepository.GetByVersion", err)
	}
	return c, nil
}

// List implements ReleaseCandidateRepository.
func (r *PostgresReleaseCandidateRepository) List(ctx context.Context) ([]ReleaseCandidate, error) {
	q := `SELECT ` + candidateColumns + ` FROM garelease_candidates ORDER BY frozen_at ASC`
	rows, err := r.exec.Query(ctx, q)
	if err != nil {
		return nil, wrapf("PostgresReleaseCandidateRepository.List", err)
	}
	defer rows.Close()

	out := make([]ReleaseCandidate, 0)
	for rows.Next() {
		var c ReleaseCandidate
		if err := scanCandidate(rows, &c); err != nil {
			return nil, wrapf("PostgresReleaseCandidateRepository.List", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresReleaseCandidateRepository.List", err)
	}
	return out, nil
}

var _ ReleaseCandidateRepository = (*PostgresReleaseCandidateRepository)(nil)

// isUniqueViolation reports whether err represents a Postgres unique
// constraint violation (SQLSTATE 23505), mirroring
// packages/compliance's identical helper of the same name.
func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}

// PostgresReleaseRepository is a PostgreSQL-backed ReleaseRepository,
// storing Release rows in the `garelease_releases` table. Equally
// never wrapped by packages/tenancy.WithTenantScope.
type PostgresReleaseRepository struct {
	exec persistence.Executor
}

// NewPostgresReleaseRepository builds a PostgresReleaseRepository bound
// to exec.
func NewPostgresReleaseRepository(exec persistence.Executor) *PostgresReleaseRepository {
	return &PostgresReleaseRepository{exec: exec}
}

const releaseColumns = `id, candidate_id, version, commit_sha, cut_by, cut_at`

func scanRelease(row rowScanner, r *Release) error {
	return row.Scan(&r.ID, &r.CandidateID, &r.Version, &r.CommitSHA, &r.CutBy, &r.CutAt)
}

// Create implements ReleaseRepository.
func (r *PostgresReleaseRepository) Create(ctx context.Context, rel *Release) error {
	if rel == nil {
		return ErrInvalidRelease
	}
	q := `
		INSERT INTO garelease_releases (id, candidate_id, version, commit_sha, cut_by, cut_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING ` + releaseColumns

	row := r.exec.QueryRow(ctx, q, rel.ID, rel.CandidateID, rel.Version, rel.CommitSHA, rel.CutBy, rel.CutAt)
	if err := scanRelease(row, rel); err != nil {
		if isUniqueViolation(err) {
			return ErrAlreadyReleased
		}
		return wrapf("PostgresReleaseRepository.Create", err)
	}
	return nil
}

// Get implements ReleaseRepository.
func (r *PostgresReleaseRepository) Get(ctx context.Context, id uuid.UUID) (*Release, error) {
	q := `SELECT ` + releaseColumns + ` FROM garelease_releases WHERE id = $1`
	rel := &Release{}
	row := r.exec.QueryRow(ctx, q, id)
	if err := scanRelease(row, rel); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReleaseNotFound
		}
		return nil, wrapf("PostgresReleaseRepository.Get", err)
	}
	return rel, nil
}

// GetByCandidateID implements ReleaseRepository.
func (r *PostgresReleaseRepository) GetByCandidateID(ctx context.Context, candidateID uuid.UUID) (*Release, error) {
	q := `SELECT ` + releaseColumns + ` FROM garelease_releases WHERE candidate_id = $1`
	rel := &Release{}
	row := r.exec.QueryRow(ctx, q, candidateID)
	if err := scanRelease(row, rel); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReleaseNotFound
		}
		return nil, wrapf("PostgresReleaseRepository.GetByCandidateID", err)
	}
	return rel, nil
}

// List implements ReleaseRepository.
func (r *PostgresReleaseRepository) List(ctx context.Context) ([]Release, error) {
	q := `SELECT ` + releaseColumns + ` FROM garelease_releases ORDER BY cut_at ASC`
	rows, err := r.exec.Query(ctx, q)
	if err != nil {
		return nil, wrapf("PostgresReleaseRepository.List", err)
	}
	defer rows.Close()

	out := make([]Release, 0)
	for rows.Next() {
		var rel Release
		if err := scanRelease(rows, &rel); err != nil {
			return nil, wrapf("PostgresReleaseRepository.List", err)
		}
		out = append(out, rel)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapf("PostgresReleaseRepository.List", err)
	}
	return out, nil
}

var _ ReleaseRepository = (*PostgresReleaseRepository)(nil)
