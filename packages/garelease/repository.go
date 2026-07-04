package garelease

import (
	"context"

	"github.com/google/uuid"
)

// ReleaseCandidateRepository persists ReleaseCandidate rows.
// ReleaseCandidate is platform-global, not tenant-scoped -- a single
// candidate is shared by every tenant of this deployment, mirroring
// packages/compliance.ControlRepository's identical shared-catalogue
// reasoning (see types.go's ReleaseCandidate doc comment). Unlike the
// tenant-scoped repositories elsewhere in this codebase, no method here
// takes or checks a tenantID.
type ReleaseCandidateRepository interface {
	Create(ctx context.Context, c *ReleaseCandidate) error
	Get(ctx context.Context, id uuid.UUID) (*ReleaseCandidate, error)
	GetByVersion(ctx context.Context, version string) (*ReleaseCandidate, error)
	List(ctx context.Context) ([]ReleaseCandidate, error)
}

// ReleaseRepository persists Release rows, equally platform-global.
type ReleaseRepository interface {
	Create(ctx context.Context, r *Release) error
	Get(ctx context.Context, id uuid.UUID) (*Release, error)
	GetByCandidateID(ctx context.Context, candidateID uuid.UUID) (*Release, error)
	List(ctx context.Context) ([]Release, error)
}
