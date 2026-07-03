package signoff

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/guardrail"
)

// GateImpl is the real, persisted packages/guardrail.SignoffGate
// implementation this phase supplies. It is deliberately minimal —
// backed by nothing but a Repository and a tenantID — so it can be
// constructed and handed to guardrail.CanFinalize without pulling in
// this package's full Service (which also requires a
// CaseVersionReader and enforces write-side permission/acknowledgement
// rules that have no bearing on a pure status read).
//
// See doc/signoff-workflow.md for the exact wiring example that
// swaps guardrail.NoSignoffRecordedGate for GateImpl.
type GateImpl struct {
	repo     Repository
	tenantID uuid.UUID
}

// NewGate builds a GateImpl reading sign-off status from repo, scoped
// to tenantID. Returns ErrNilRepository if repo is nil.
func NewGate(repo Repository, tenantID uuid.UUID) (*GateImpl, error) {
	if repo == nil {
		return nil, ErrNilRepository
	}
	return &GateImpl{repo: repo, tenantID: tenantID}, nil
}

// Status implements guardrail.SignoffGate. A case with no
// SignoffRecord yet (never entered the review workflow) reports
// guardrail.SignoffPending — the same fail-closed default
// guardrail.NoSignoffRecordedGate reports, so a caller cannot
// finalize a case sign-off has never even been requested for.
func (g *GateImpl) Status(ctx context.Context, caseID string) (guardrail.SignoffStatus, error) {
	if caseID == "" {
		return guardrail.SignoffPending, guardrail.ErrEmptyCaseID
	}
	id, err := uuid.Parse(caseID)
	if err != nil {
		return guardrail.SignoffPending, fmt.Errorf("signoff: GateImpl.Status: invalid case id %q: %w", caseID, err)
	}

	rec, err := g.repo.Get(ctx, g.tenantID, id)
	if err != nil {
		if err == ErrNotFound {
			return guardrail.SignoffPending, nil
		}
		return guardrail.SignoffPending, fmt.Errorf("signoff: GateImpl.Status: %w", err)
	}
	return rec.Status, nil
}

var _ guardrail.SignoffGate = (*GateImpl)(nil)
