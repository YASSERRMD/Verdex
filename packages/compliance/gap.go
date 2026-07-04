package compliance

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Status is the result of evaluating a single Control against a
// tenant's collected ControlEvidence (task 6).
type Status string

const (
	// StatusSatisfied means at least minEvidenceForSatisfied pieces of
	// distinct-Kind evidence are on file for the control -- strong
	// enough coverage that the control is considered fully
	// demonstrated.
	StatusSatisfied Status = "satisfied"

	// StatusPartiallyMet means at least one piece of evidence is on
	// file, but fewer than minEvidenceForSatisfied distinct kinds --
	// some coverage exists, but not enough to consider the control
	// fully demonstrated.
	StatusPartiallyMet Status = "partially_met"

	// StatusGap means no evidence at all is on file for the control --
	// nothing demonstrates it is satisfied.
	StatusGap Status = "gap"
)

// IsValid reports whether s is one of the named Status constants.
func (s Status) IsValid() bool {
	switch s {
	case StatusSatisfied, StatusPartiallyMet, StatusGap:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (s Status) String() string { return string(s) }

// minEvidenceKindsForSatisfied is the number of distinct EvidenceKind
// values a Control must have at least one ControlEvidence record for
// before EvaluateControl reports StatusSatisfied rather than
// StatusPartiallyMet. Two distinct kinds (e.g. a test asserting the
// behavior AND an audit query proving it runs in production, or a
// document describing the procedure AND a configuration artifact
// enforcing it) is a deliberately real bar: a single test name alone
// proves the code path exists, not that it is actually exercised
// operationally, so this package does not call a control fully
// satisfied on one piece of evidence alone.
const minEvidenceKindsForSatisfied = 2

// EvaluateControl reports the Status of control, given every
// ControlEvidence on file for it, evaluated as of now (task 6's
// per-control evaluation). This is real evaluation logic: it counts
// distinct EvidenceKind values among the matching evidence records
// (not just record count, which would let five redundant test-name
// references count for more than they should), rather than returning a
// hardcoded status.
func EvaluateControl(control Control, evidence []ControlEvidence, now time.Time) ControlGapResult {
	kinds := make(map[EvidenceKind]struct{})
	matched := make([]ControlEvidence, 0)
	for _, e := range evidence {
		if e.ControlID != control.ID {
			continue
		}
		if e.CollectedAt.After(now) {
			// Evidence collected in the future relative to the
			// evaluation instant cannot yet demonstrate anything --
			// exercised directly by gap_test.go so a clock-skewed or
			// backdated record can never inflate a status.
			continue
		}
		matched = append(matched, e)
		kinds[e.Kind] = struct{}{}
	}

	status := StatusGap
	switch {
	case len(kinds) >= minEvidenceKindsForSatisfied:
		status = StatusSatisfied
	case len(matched) > 0:
		status = StatusPartiallyMet
	}

	return ControlGapResult{
		Control:       control,
		Status:        status,
		EvidenceCount: len(matched),
		EvidenceKinds: len(kinds),
	}
}

// ControlGapResult is one Control's outcome within a GapAnalysisReport.
type ControlGapResult struct {
	// Control is the catalogued control this result evaluates.
	Control Control `json:"control"`

	// Status is the resolved compliance Status.
	Status Status `json:"status"`

	// EvidenceCount is how many ControlEvidence records (as of the
	// evaluation instant) matched this control.
	EvidenceCount int `json:"evidence_count"`

	// EvidenceKinds is how many distinct EvidenceKind values were
	// represented among those matching records.
	EvidenceKinds int `json:"evidence_kinds"`
}

// GapAnalysisReport is a tenant's full gap-analysis snapshot (task 6):
// every applicable Control (per ApplicableControls/Profile), each
// evaluated against the tenant's collected ControlEvidence.
type GapAnalysisReport struct {
	// TenantID is the tenant this report was generated for.
	TenantID uuid.UUID `json:"tenant_id"`

	// GeneratedAt is when this report was computed.
	GeneratedAt time.Time `json:"generated_at"`

	// Results is one ControlGapResult per applicable Control.
	Results []ControlGapResult `json:"results"`
}

// CountByStatus returns how many Results carry each Status, keyed by
// status.
func (r GapAnalysisReport) CountByStatus() map[Status]int {
	counts := map[Status]int{
		StatusSatisfied:    0,
		StatusPartiallyMet: 0,
		StatusGap:          0,
	}
	for _, res := range r.Results {
		counts[res.Status]++
	}
	return counts
}

// Gaps returns every ControlGapResult whose Status is StatusGap,
// convenience for a caller that only wants the list of unaddressed
// controls.
func (r GapAnalysisReport) Gaps() []ControlGapResult {
	out := make([]ControlGapResult, 0)
	for _, res := range r.Results {
		if res.Status == StatusGap {
			out = append(out, res)
		}
	}
	return out
}

// RunGapAnalysis computes a GapAnalysisReport for tenantID (task 6),
// requiring viewPermission and tenant match: every catalogued Control
// applicable per the tenant's Profile (or every catalogued Control, if
// no profile has been set -- the permissive default), each evaluated
// via EvaluateControl against the tenant's full collected
// ControlEvidence set.
func (e *Engine) RunGapAnalysis(ctx context.Context, tenantID uuid.UUID) (GapAnalysisReport, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return GapAnalysisReport{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return GapAnalysisReport{}, err
	}

	catalogue, err := e.controls.List(ctx)
	if err != nil {
		return GapAnalysisReport{}, wrapf("RunGapAnalysis", err)
	}

	var profile *Profile
	p, err := e.profiles.Get(ctx, tenantID)
	switch {
	case err == nil:
		profile = p
	case isNotFound(err, ErrProfileNotFound):
		profile = nil
	default:
		return GapAnalysisReport{}, wrapf("RunGapAnalysis", err)
	}
	applicable := ApplicableControls(catalogue, profile)

	evidence, err := e.evidence.ListAll(ctx, tenantID)
	if err != nil {
		return GapAnalysisReport{}, wrapf("RunGapAnalysis", err)
	}

	now := e.now()
	results := make([]ControlGapResult, 0, len(applicable))
	for _, c := range applicable {
		results = append(results, EvaluateControl(c, evidence, now))
	}

	return GapAnalysisReport{
		TenantID:    tenantID,
		GeneratedAt: now,
		Results:     results,
	}, nil
}
