package iac

import (
	"fmt"
	"time"
)

// Stage names one step of an environment promotion pipeline. A closed
// enum, exactly like Tier: the three stages and their strict linear
// order (Dev -> Staging -> Prod) are a structural property of every
// deployment this platform promotes, not a per-tenant configuration
// value.
type Stage string

const (
	// StageDev is the first, non-gated stage: promoting *into* Dev
	// requires nothing (there is no stage before it).
	StageDev Stage = "dev"

	// StageStaging is the second stage. Promoting into it requires
	// Dev's own DeploymentVerification to have passed.
	StageStaging Stage = "staging"

	// StageProd is the terminal stage. Promoting into it requires
	// Staging's DeploymentVerification to have passed -- this is the
	// gate the brief calls out explicitly: "assert promotion to Prod
	// is blocked without a passing Staging verification".
	StageProd Stage = "prod"
)

// stageOrder fixes the strict linear sequence every PromotionPipeline
// follows. A map (not a slice index) mirrors
// packages/setup.validTransitions's state-machine convention,
// including its "empty target slice means terminal" idiom.
var stageOrder = map[Stage]Stage{
	StageDev:     StageStaging,
	StageStaging: StageProd,
	// StageProd has no entry: it is terminal, matching
	// packages/setup.StateLocked's convention of a state absent from
	// the transition map entirely rather than present with an empty
	// slice (Go's map zero value already reads as "no next stage"
	// either way; the entry is omitted here for the same reason
	// packages/setup's own table omits it -- see
	// packages/setup/state.go's StateLocked entry, which *is* present
	// with an explicit empty slice; this package's zero-value-map-miss
	// idiom is equivalent but slightly more compact, documented here so
	// the difference is deliberate, not an oversight).
}

// IsValid reports whether s is one of the named Stage constants.
func (s Stage) IsValid() bool {
	switch s {
	case StageDev, StageStaging, StageProd:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (s Stage) String() string { return string(s) }

// IsTerminal reports whether s has no valid outgoing promotion,
// mirroring packages/setup.SetupState.IsTerminal's convention.
func (s Stage) IsTerminal() bool {
	_, ok := stageOrder[s]
	return s.IsValid() && !ok
}

// Next returns the stage s promotes into, and false if s is terminal
// or not a recognized Stage.
func (s Stage) Next() (Stage, bool) {
	next, ok := stageOrder[s]
	return next, ok
}

// PromotionPipeline is the environment-promotion state machine (task
// 5): a deployment moves Dev -> Staging -> Prod, and each promotion
// requires the CURRENT stage's most recent DeploymentVerificationReport
// to have passed before advancing. This is a real gated transition,
// not an unconditional state bump -- Promote refuses to advance
// without a passing verification on file, exactly the same
// fail-closed posture packages/setup.SetupWizard's CanTransitionTo and
// packages/airgapped.Profile.Validate already establish for this
// codebase.
type PromotionPipeline struct {
	// DeploymentID identifies the deployment this pipeline promotes,
	// matching packages/iac.DeploymentProfile.DeploymentID.
	DeploymentID string `json:"deployment_id"`

	// CurrentStage is the stage this deployment currently occupies.
	CurrentStage Stage `json:"current_stage"`

	// verifications records the most recent DeploymentVerificationReport
	// received for each Stage via RecordVerification. Promote consults
	// this map for CurrentStage's entry before allowing a transition to
	// CurrentStage.Next(). Unexported: mutated only through
	// RecordVerification/Promote so a caller cannot forge a passing
	// report by direct field assignment.
	verifications map[Stage]DeploymentVerificationReport

	// History records every stage this pipeline has actually occupied,
	// in order, starting with the initial stage. Promote appends to it
	// on every successful transition.
	History []Stage `json:"history"`

	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// NewPromotionPipeline builds a PromotionPipeline for deploymentID,
// starting at StageDev (the only stage a pipeline may begin at -- a
// deployment is always built up from Dev, never dropped directly into
// Staging or Prod).
//
// Returns ErrEmptyDeploymentID if deploymentID is blank.
func NewPromotionPipeline(deploymentID string) (*PromotionPipeline, error) {
	if deploymentID == "" {
		return nil, ErrEmptyDeploymentID
	}
	now := time.Now().UTC()
	return &PromotionPipeline{
		DeploymentID:  deploymentID,
		CurrentStage:  StageDev,
		verifications: make(map[Stage]DeploymentVerificationReport),
		History:       []Stage{StageDev},
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

// RecordVerification attaches report as the most recent
// DeploymentVerificationReport for report.Stage, so a later Promote
// call from that stage can consult it. Recording a verification for a
// stage other than p.CurrentStage is permitted (e.g. re-running Dev's
// checklist after Staging has already been reached, for audit
// purposes) but does not retroactively unblock any transition already
// evaluated.
//
// Returns ErrNilPipeline if p is nil, and ErrInvalidStage if
// report.Stage is not a recognized Stage.
func (p *PromotionPipeline) RecordVerification(report DeploymentVerificationReport) error {
	if p == nil {
		return ErrNilPipeline
	}
	if !report.Stage.IsValid() {
		return wrapf("RecordVerification", ErrInvalidStage)
	}
	if p.verifications == nil {
		p.verifications = make(map[Stage]DeploymentVerificationReport)
	}
	p.verifications[report.Stage] = report
	p.UpdatedAt = time.Now().UTC()
	return nil
}

// LatestVerification returns the most recently recorded
// DeploymentVerificationReport for stage, and false if none has been
// recorded.
func (p *PromotionPipeline) LatestVerification(stage Stage) (DeploymentVerificationReport, bool) {
	if p == nil || p.verifications == nil {
		return DeploymentVerificationReport{}, false
	}
	report, ok := p.verifications[stage]
	return report, ok
}

// Promote attempts to advance p from its CurrentStage to the next
// stage in sequence. This is the real gate the brief calls for:
// promotion succeeds only if a DeploymentVerificationReport has been
// recorded for CurrentStage (via RecordVerification) AND that
// report's Passed() is true. There is no bypass path.
//
// Returns ErrNilPipeline if p is nil, ErrTerminalStage if p is already
// at StageProd, ErrStageNotVerified if CurrentStage has no recorded
// verification or its most recent one did not pass, and
// ErrNoValidTransition if CurrentStage is not a recognized Stage (should
// not happen for a pipeline built via NewPromotionPipeline, but
// guarded against direct struct construction/mutation).
func (p *PromotionPipeline) Promote() error {
	if p == nil {
		return ErrNilPipeline
	}
	if p.CurrentStage.IsTerminal() {
		return wrapf("Promote", ErrTerminalStage)
	}

	next, ok := p.CurrentStage.Next()
	if !ok {
		return wrapf("Promote", ErrNoValidTransition)
	}

	report, recorded := p.LatestVerification(p.CurrentStage)
	if !recorded || !report.Passed() {
		return wrapf("Promote", fmt.Errorf("%w: stage %q", ErrStageNotVerified, p.CurrentStage))
	}

	p.CurrentStage = next
	p.History = append(p.History, next)
	p.UpdatedAt = time.Now().UTC()
	return nil
}
