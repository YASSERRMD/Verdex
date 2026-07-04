package pilot

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// DeploymentStatus tracks a PilotDeployment's real-world lifecycle
// state machine: Provisioning -> CorpusOnboarding -> Active ->
// Concluded. A closed enum, mirroring
// packages/vulnmanagement.Status's guarded-transition shape by
// reference -- this package imports neither vulnmanagement nor
// packages/caselifecycle.State, both of which this shape is modeled
// after.
type DeploymentStatus string

const (
	// DeploymentStatusProvisioning is a PilotDeployment's initial
	// state: the tenant-scoped deployment record has been created but
	// no jurisdiction/corpus onboarding has completed yet (task 1).
	DeploymentStatusProvisioning DeploymentStatus = "provisioning"

	// DeploymentStatusCorpusOnboarding means the pilot jurisdiction and
	// its supporting corpus are being onboarded (task 2), but the
	// deployment is not yet open to supervised case work.
	DeploymentStatusCorpusOnboarding DeploymentStatus = "corpus_onboarding"

	// DeploymentStatusActive means the pilot is live: supervised pilot
	// cases may be run, feedback collected, and findings triaged (tasks
	// 3-8).
	DeploymentStatusActive DeploymentStatus = "active"

	// DeploymentStatusConcluded is a terminal state: the pilot has
	// ended (successfully or otherwise) and its findings have been
	// captured into a final Report (task 9).
	DeploymentStatusConcluded DeploymentStatus = "concluded"
)

// IsValid reports whether s is one of the named DeploymentStatus
// constants.
func (s DeploymentStatus) IsValid() bool {
	switch s {
	case DeploymentStatusProvisioning, DeploymentStatusCorpusOnboarding, DeploymentStatusActive, DeploymentStatusConcluded:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (s DeploymentStatus) String() string { return string(s) }

// IsTerminal reports whether s is the state machine's terminal state
// (Concluded) -- no further transition is permitted out of a terminal
// state.
func (s DeploymentStatus) IsTerminal() bool {
	return s == DeploymentStatusConcluded
}

// allowedDeploymentTransitions is the single authoritative source of
// truth for which DeploymentStatus-to-DeploymentStatus moves
// CanTransitionDeployment permits, mirroring
// packages/vulnmanagement.allowedTransitions's map-of-slices shape by
// reference (this package does not import packages/vulnmanagement).
// This is a strictly linear lifecycle -- there is no sanctioned path
// back from a later stage to an earlier one; a pilot that needs to
// re-onboard a corpus starts a new PilotDeployment record rather than
// regressing an existing one.
var allowedDeploymentTransitions = map[DeploymentStatus][]DeploymentStatus{
	DeploymentStatusProvisioning:     {DeploymentStatusCorpusOnboarding},
	DeploymentStatusCorpusOnboarding: {DeploymentStatusActive},
	DeploymentStatusActive:           {DeploymentStatusConcluded},
	DeploymentStatusConcluded:        {},
}

// CanTransitionDeployment reports whether moving directly from `from`
// to `to` is permitted by allowedDeploymentTransitions.
func CanTransitionDeployment(from, to DeploymentStatus) bool {
	for _, s := range allowedDeploymentTransitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// PilotDeployment is a tenant-scoped controlled pilot (task 1):
// references a jurisdiction by code (mirroring the JurisdictionCode
// string-reference convention already used by
// packages/corpusupdater.CorpusUpdateJob, packages/issueagent, and
// packages/reasoningeval -- see doc/pilot.md's composition table for
// why this package does not import packages/jurisdiction directly), a
// Status tracking where in its lifecycle the pilot currently sits, and
// a start/end date bounding the pilot window.
type PilotDeployment struct {
	// ID uniquely identifies this pilot deployment.
	ID uuid.UUID `json:"id"`

	// TenantID is the tenant this pilot deployment belongs to.
	TenantID uuid.UUID `json:"tenant_id"`

	// Name is a short human-readable label for this pilot (e.g. "Dubai
	// Commercial Court Q3 pilot").
	Name string `json:"name"`

	// JurisdictionCode names the packages/jurisdiction jurisdiction
	// this pilot onboards, stored as a plain string reference rather
	// than a hard dependency on packages/jurisdiction's Go types.
	JurisdictionCode string `json:"jurisdiction_code"`

	// Status is this deployment's current position in the
	// Provisioning -> CorpusOnboarding -> Active -> Concluded lifecycle.
	Status DeploymentStatus `json:"status"`

	// StartDate is when the pilot's supervised case work is scheduled
	// (or began) to run.
	StartDate time.Time `json:"start_date"`

	// EndDate is when the pilot's supervised case work is scheduled (or
	// concluded) to end. Zero means open-ended (no end date set yet).
	EndDate time.Time `json:"end_date,omitempty"`

	// CreatedBy is the identity.User who provisioned this deployment.
	CreatedBy uuid.UUID `json:"created_by"`

	// CreatedAt and UpdatedAt are bookkeeping timestamps.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks d for structural well-formedness.
func (d *PilotDeployment) Validate() error {
	if d == nil {
		return ErrInvalidDeployment
	}
	if d.TenantID == uuid.Nil {
		return ErrEmptyTenantID
	}
	if strings.TrimSpace(d.Name) == "" {
		return wrapf("PilotDeployment.Validate", ErrInvalidDeployment)
	}
	if strings.TrimSpace(d.JurisdictionCode) == "" {
		return wrapf("PilotDeployment.Validate", ErrInvalidDeployment)
	}
	if !d.Status.IsValid() {
		return wrapf("PilotDeployment.Validate", ErrInvalidDeployment)
	}
	if d.StartDate.IsZero() {
		return wrapf("PilotDeployment.Validate", ErrInvalidDeployment)
	}
	if !d.EndDate.IsZero() && d.EndDate.Before(d.StartDate) {
		return wrapf("PilotDeployment.Validate", ErrInvalidDeployment)
	}
	return nil
}

// PilotCase is a supervised case run within a PilotDeployment (task
// 3): it references a real packages/caselifecycle.Case by ID only
// (never by value), mirroring packages/accessgovernance.CaseGrant's
// CaseID-reference convention exactly -- this package does not
// duplicate Case itself, only the pilot-supervision record layered on
// top of it.
type PilotCase struct {
	// ID uniquely identifies this pilot-case record.
	ID uuid.UUID `json:"id"`

	// TenantID is the tenant this pilot-case record belongs to.
	TenantID uuid.UUID `json:"tenant_id"`

	// DeploymentID references the PilotDeployment this case is being
	// run under.
	DeploymentID uuid.UUID `json:"deployment_id"`

	// CaseID identifies the packages/caselifecycle.Case being
	// supervised under this pilot. Reference only -- see the type doc
	// comment.
	CaseID uuid.UUID `json:"case_id"`

	// SupervisorUserID is the identity.User assigned to supervise this
	// case during the pilot.
	SupervisorUserID uuid.UUID `json:"supervisor_user_id"`

	// OutcomeObserved reports whether the supervisor has recorded that
	// the case reached (or was observed through) an outcome the pilot
	// can learn from. False until MarkOutcomeObserved is called.
	OutcomeObserved bool `json:"outcome_observed"`

	// AssignedAt is when this case was assigned into the pilot.
	AssignedAt time.Time `json:"assigned_at"`

	// ObservedAt is when OutcomeObserved was set true. Nil until then.
	ObservedAt *time.Time `json:"observed_at,omitempty"`

	// CreatedAt and UpdatedAt are bookkeeping timestamps.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks c for structural well-formedness.
func (c *PilotCase) Validate() error {
	if c == nil {
		return ErrInvalidCase
	}
	if c.TenantID == uuid.Nil {
		return ErrEmptyTenantID
	}
	if c.DeploymentID == uuid.Nil {
		return wrapf("PilotCase.Validate", ErrInvalidCase)
	}
	if c.CaseID == uuid.Nil {
		return wrapf("PilotCase.Validate", ErrInvalidCase)
	}
	if c.SupervisorUserID == uuid.Nil {
		return wrapf("PilotCase.Validate", ErrInvalidCase)
	}
	if c.AssignedAt.IsZero() {
		return wrapf("PilotCase.Validate", ErrInvalidCase)
	}
	return nil
}
