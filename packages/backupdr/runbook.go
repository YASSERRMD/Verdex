package backupdr

import "strings"

// RunbookStep is a single ordered step within a Runbook: what to do,
// and who (by role) owns doing it.
type RunbookStep struct {
	// Order is this step's 1-based position within the Runbook. Steps
	// are executed in ascending Order.
	Order int `json:"order"`

	// Description says what to do at this step.
	Description string `json:"description"`

	// OwnerRole names the role responsible for executing this step
	// (e.g. "on-call engineer", "tenant administrator", "judge/clerk
	// liaison") -- a free-form label, not an identity.Role constant:
	// a DR runbook step's owner is an operational responsibility, not
	// necessarily a role this platform's RBAC model itself grants
	// permissions to.
	OwnerRole string `json:"owner_role"`
}

// Validate checks s for structural well-formedness.
func (s RunbookStep) Validate() error {
	if s.Order <= 0 {
		return wrapf("RunbookStep.Validate", ErrInvalidRunbook)
	}
	if strings.TrimSpace(s.Description) == "" {
		return wrapf("RunbookStep.Validate", ErrInvalidRunbook)
	}
	if strings.TrimSpace(s.OwnerRole) == "" {
		return wrapf("RunbookStep.Validate", ErrInvalidRunbook)
	}
	return nil
}

// Runbook is a structured, ordered disaster-recovery procedure (task
// 7): the data-model counterpart to the human-readable
// doc/dr-runbook.md artifact. Name identifies which scenario this
// Runbook addresses (e.g. "region outage", "corpus corruption");
// Steps is the ordered procedure, each with a Description and an
// OwnerRole.
type Runbook struct {
	// Name identifies the DR scenario this Runbook addresses.
	Name string `json:"name"`

	// Steps is the ordered procedure. Validate requires at least one
	// step and requires Order values to be unique and to appear in
	// ascending order.
	Steps []RunbookStep `json:"steps"`
}

// Validate checks r for structural well-formedness: a non-blank Name,
// at least one step, and Steps sorted by strictly ascending, unique
// Order values (so OrderedSteps below never has to silently re-sort a
// malformed Runbook).
func (r Runbook) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return wrapf("Runbook.Validate", ErrInvalidRunbook)
	}
	if len(r.Steps) == 0 {
		return wrapf("Runbook.Validate", ErrInvalidRunbook)
	}
	prevOrder := 0
	for _, s := range r.Steps {
		if err := s.Validate(); err != nil {
			return err
		}
		if s.Order <= prevOrder {
			return wrapf("Runbook.Validate", ErrInvalidRunbook)
		}
		prevOrder = s.Order
	}
	return nil
}

// StepCount returns len(r.Steps), a small convenience for callers (a
// dashboard, a doc generator) that just want the procedure's length.
func (r Runbook) StepCount() int {
	return len(r.Steps)
}

// OwnerRoles returns the distinct set of OwnerRole values across every
// step, in first-seen order -- who needs to be in the room for this
// Runbook to be executable end to end.
func (r Runbook) OwnerRoles() []string {
	seen := make(map[string]struct{}, len(r.Steps))
	out := make([]string, 0, len(r.Steps))
	for _, s := range r.Steps {
		if _, ok := seen[s.OwnerRole]; ok {
			continue
		}
		seen[s.OwnerRole] = struct{}{}
		out = append(out, s.OwnerRole)
	}
	return out
}

// DefaultDRRunbook returns this platform's starter DR runbook: the
// structured counterpart to doc/dr-runbook.md, kept in the same order
// as that document's numbered procedure so the two never drift apart
// silently. Deployments may register additional/alternate Runbooks
// (e.g. per DataClass, per scenario) through their own tooling; this
// function exists so the data model is never empty out of the box.
func DefaultDRRunbook() Runbook {
	return Runbook{
		Name: "region_outage",
		Steps: []RunbookStep{
			{Order: 1, Description: "Declare the incident and page the on-call rotation.", OwnerRole: "on-call engineer"},
			{Order: 2, Description: "Confirm scope: which tenants and DataClasses are affected.", OwnerRole: "incident commander"},
			{Order: 3, Description: "Identify the nearest recovery point per affected DataClass via ResolveRecoveryPoint.", OwnerRole: "on-call engineer"},
			{Order: 4, Description: "Verify the candidate BackupRecord's integrity via VerifyIntegrity before restoring from it.", OwnerRole: "on-call engineer"},
			{Order: 5, Description: "Restore from the verified cross-region or offline copy per BackupPolicy.CrossRegionRequired.", OwnerRole: "on-call engineer"},
			{Order: 6, Description: "Run post-restore verification checks against the restored DataClass.", OwnerRole: "on-call engineer"},
			{Order: 7, Description: "Evaluate the restore's actual duration against the DataClass's RTO Target via EvaluateRTO.", OwnerRole: "incident commander"},
			{Order: 8, Description: "Notify affected tenant administrators and, where applicable, record a compliance/breach-notification event.", OwnerRole: "tenant administrator liaison"},
			{Order: 9, Description: "Record the incident and restore outcome as a RestoreDrill-shaped entry for the post-incident review.", OwnerRole: "incident commander"},
			{Order: 10, Description: "Hold a post-incident review and file follow-up actions.", OwnerRole: "incident commander"},
		},
	}
}
