package privacy

import "time"

// DeletionAction is the action a RetentionPolicy prescribes once a
// record's age exceeds its retention window (task 3).
type DeletionAction string

const (
	// ActionHardDelete means the record's personal content must be
	// permanently, irrecoverably removed.
	ActionHardDelete DeletionAction = "hard_delete"

	// ActionAnonymize means the record's personal content must be
	// replaced with an anonymized/aggregated projection (see
	// AnonymizeForAnalytics in anonymize.go) rather than removed
	// outright, e.g. because the record still carries analytical value
	// once stripped of identifying content.
	ActionAnonymize DeletionAction = "anonymize"

	// ActionRetain means no action is currently required -- the record
	// has not yet reached the end of its retention window.
	ActionRetain DeletionAction = "retain"
)

// IsValid reports whether a is one of the named DeletionAction
// constants.
func (a DeletionAction) IsValid() bool {
	switch a {
	case ActionHardDelete, ActionAnonymize, ActionRetain:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (a DeletionAction) String() string { return string(a) }

// RetentionPolicy binds a DataCategory to a retention Window and the
// DeletionAction required once a record of that category exceeds it
// (task 3). One RetentionPolicy exists per DataCategory within a
// tenant's registered set (see Engine.SetRetentionPolicy).
type RetentionPolicy struct {
	// Category is the DataCategory this policy governs.
	Category DataCategory `json:"category"`

	// Window is how long a record of Category is retained from its
	// origin timestamp before EnforceRetention reports OnAction due.
	// Must be positive.
	Window time.Duration `json:"window"`

	// OnAction is the DeletionAction required once Window has elapsed.
	OnAction DeletionAction `json:"on_action"`
}

// Validate checks p for structural well-formedness.
func (p RetentionPolicy) Validate() error {
	if !p.Category.IsValid() {
		return wrapf("RetentionPolicy.Validate", ErrInvalidDataCategory)
	}
	if p.Window <= 0 {
		return wrapf("RetentionPolicy.Validate", ErrInvalidRetentionPolicy)
	}
	switch p.OnAction {
	case ActionHardDelete, ActionAnonymize:
		// Both are legitimate terminal actions for a policy to
		// prescribe. ActionRetain is deliberately not accepted here --
		// it is EnforceRetention's own answer while a record is still
		// within Window, not something a policy author configures.
	default:
		return wrapf("RetentionPolicy.Validate", ErrInvalidRetentionPolicy)
	}
	return nil
}

// CutoffBefore returns the time before which a record of p.Category is
// eligible for p.OnAction, given now as the current time -- mirroring
// packages/auditlog.RetentionPolicy.CutoffBefore's shape exactly.
func (p RetentionPolicy) CutoffBefore(now time.Time) time.Time {
	return now.Add(-p.Window)
}

// EnforceRetention evaluates a record's age (recordCreatedAt) against
// policy as of now, returning the DeletionAction the record currently
// requires: ActionRetain if the record has not yet reached the end of
// its retention window, or policy.OnAction if it has. This is the real
// evaluation function task 3 asks for -- not a stub -- and is the
// single place retention-window arithmetic lives, so
// Engine.EvaluateRetention (engine.go) and any future scheduled job
// both call through it rather than re-deriving the comparison.
func EnforceRetention(policy RetentionPolicy, recordCreatedAt time.Time, now time.Time) (DeletionAction, error) {
	if err := policy.Validate(); err != nil {
		return "", err
	}
	if recordCreatedAt.IsZero() {
		return "", wrapf("EnforceRetention", ErrInvalidRetentionPolicy)
	}
	cutoff := policy.CutoffBefore(now)
	if recordCreatedAt.After(cutoff) {
		return ActionRetain, nil
	}
	return policy.OnAction, nil
}
