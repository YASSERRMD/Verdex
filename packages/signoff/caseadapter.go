package signoff

import (
	"context"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/caselifecycle"
)

// CaselifecycleVersionReader adapts a packages/caselifecycle.Repository
// into this package's CaseVersionReader, so callers who already hold a
// caselifecycle.Repository (Postgres- or in-memory-backed) do not need
// to write their own adapter — see doc/signoff-workflow.md for the
// full wiring example.
type CaselifecycleVersionReader struct {
	Repo caselifecycle.Repository
}

// NewCaselifecycleVersionReader builds a CaselifecycleVersionReader
// backed by repo.
func NewCaselifecycleVersionReader(repo caselifecycle.Repository) *CaselifecycleVersionReader {
	return &CaselifecycleVersionReader{Repo: repo}
}

// CaseVersion implements CaseVersionReader by returning the case's
// current caselifecycle.Case.MetadataVersion.
func (a *CaselifecycleVersionReader) CaseVersion(ctx context.Context, tenantID, caseID uuid.UUID) (int, error) {
	c, err := a.Repo.Get(ctx, tenantID, caseID)
	if err != nil {
		return 0, err
	}
	return c.MetadataVersion, nil
}

var _ CaseVersionReader = (*CaselifecycleVersionReader)(nil)
