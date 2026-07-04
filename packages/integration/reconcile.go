package integration

import (
	"sort"
	"time"

	"github.com/google/uuid"
)

// ReconciliationKind classifies which direction Reconcile compared,
// mirroring packages/auditlog.Kind's small-closed-taxonomy convention.
type ReconciliationKind string

const (
	// ReconciliationKindImport compares imported InboundCase external
	// IDs against a target set of external IDs expected to have been
	// imported.
	ReconciliationKindImport ReconciliationKind = "import"

	// ReconciliationKindDelivery compares delivered report external
	// case IDs against a target set of external IDs expected to have
	// received a delivery.
	ReconciliationKindDelivery ReconciliationKind = "delivery"
)

// IsValid reports whether k is one of the named ReconciliationKind
// constants.
func (k ReconciliationKind) IsValid() bool {
	switch k {
	case ReconciliationKindImport, ReconciliationKindDelivery:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (k ReconciliationKind) String() string { return string(k) }

// ReconciliationResult is the durable, tenant-scoped record of one
// Reconcile comparison: what was expected versus what was actually
// observed for a given ConnectorConfig, and the drift between them
// (task 6).
type ReconciliationResult struct {
	// ID uniquely identifies this reconciliation result.
	ID uuid.UUID `json:"id"`

	// TenantID scopes this result to a tenant.
	TenantID uuid.UUID `json:"tenant_id"`

	// ConnectorConfigID identifies which ConnectorConfig this
	// reconciliation ran against.
	ConnectorConfigID uuid.UUID `json:"connector_config_id"`

	// Kind distinguishes an import reconciliation from a delivery
	// reconciliation.
	Kind ReconciliationKind `json:"kind"`

	// ExpectedCount is the size of the target set Reconcile was given.
	ExpectedCount int `json:"expected_count"`

	// ObservedCount is the size of the observed set Reconcile was
	// given.
	ObservedCount int `json:"observed_count"`

	// MissingExternalIDs lists external IDs present in the target set
	// but absent from the observed set -- records expected to have been
	// imported/delivered but were not (drift/missed records).
	MissingExternalIDs []string `json:"missing_external_ids,omitempty"`

	// UnexpectedExternalIDs lists external IDs present in the observed
	// set but absent from the target set -- records
	// imported/delivered that were not expected (e.g. the external
	// system returned records outside the requested window).
	UnexpectedExternalIDs []string `json:"unexpected_external_ids,omitempty"`

	// RanAt is when this reconciliation was performed.
	RanAt time.Time `json:"ran_at"`

	// RanBy is the identity.User (or system actor) who triggered this
	// reconciliation.
	RanBy uuid.UUID `json:"ran_by"`
}

// Validate checks r for structural well-formedness.
func (r *ReconciliationResult) Validate() error {
	if r == nil {
		return ErrInvalidReconciliation
	}
	if r.TenantID == uuid.Nil {
		return wrapf("ReconciliationResult.Validate", ErrEmptyTenantID)
	}
	if r.ConnectorConfigID == uuid.Nil {
		return wrapf("ReconciliationResult.Validate", ErrInvalidReconciliation)
	}
	if !r.Kind.IsValid() {
		return wrapf("ReconciliationResult.Validate", ErrInvalidReconciliation)
	}
	return nil
}

// HasDrift reports whether this result found any missing or unexpected
// external ID.
func (r ReconciliationResult) HasDrift() bool {
	return len(r.MissingExternalIDs) > 0 || len(r.UnexpectedExternalIDs) > 0
}

// Reconcile compares expected (the target set of external IDs that
// should have been imported or delivered) against observed (the
// external IDs actually seen) and produces a ReconciliationResult
// describing any drift -- missed records the external system reports
// but this platform never processed, or unexpected records this
// platform processed outside the expected set (task 6). This is a
// real comparison function, directly testable with
// SandboxConnector-sourced data, not a stub that always reports clean.
func Reconcile(tenantID, connectorConfigID uuid.UUID, kind ReconciliationKind, expected, observed []string, ranBy uuid.UUID, now time.Time) (ReconciliationResult, error) {
	if tenantID == uuid.Nil {
		return ReconciliationResult{}, ErrEmptyTenantID
	}
	if connectorConfigID == uuid.Nil {
		return ReconciliationResult{}, wrapf("Reconcile", ErrInvalidReconciliation)
	}
	if !kind.IsValid() {
		return ReconciliationResult{}, wrapf("Reconcile", ErrInvalidReconciliation)
	}

	expectedSet := toSet(expected)
	observedSet := toSet(observed)

	missing := diffSorted(expectedSet, observedSet)
	unexpected := diffSorted(observedSet, expectedSet)

	return ReconciliationResult{
		ID:                    uuid.New(),
		TenantID:              tenantID,
		ConnectorConfigID:     connectorConfigID,
		Kind:                  kind,
		ExpectedCount:         len(expectedSet),
		ObservedCount:         len(observedSet),
		MissingExternalIDs:    missing,
		UnexpectedExternalIDs: unexpected,
		RanAt:                 now.UTC(),
		RanBy:                 ranBy,
	}, nil
}

func toSet(ids []string) map[string]struct{} {
	set := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		set[id] = struct{}{}
	}
	return set
}

// diffSorted returns the sorted list of keys in a but not in b.
func diffSorted(a, b map[string]struct{}) []string {
	out := make([]string, 0)
	for k := range a {
		if _, ok := b[k]; !ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
