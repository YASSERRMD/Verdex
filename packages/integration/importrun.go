package integration

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// ImportRunStatus classifies the outcome of one ImportRun (task 2),
// mirroring packages/keymanagement.KeyState's small-closed-string-enum
// convention.
type ImportRunStatus string

const (
	// ImportRunStatusSucceeded means every InboundCase the connector
	// returned was recorded without error.
	ImportRunStatusSucceeded ImportRunStatus = "succeeded"

	// ImportRunStatusPartial means the connector returned cases but at
	// least one failed FieldMapping.Apply or downstream processing;
	// FailedExternalIDs lists which.
	ImportRunStatusPartial ImportRunStatus = "partial"

	// ImportRunStatusFailed means the run could not complete at all
	// (e.g. Connector.ImportCases itself returned an error after
	// retries were exhausted).
	ImportRunStatusFailed ImportRunStatus = "failed"
)

// IsValid reports whether s is one of the named ImportRunStatus
// constants.
func (s ImportRunStatus) IsValid() bool {
	switch s {
	case ImportRunStatusSucceeded, ImportRunStatusPartial, ImportRunStatusFailed:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (s ImportRunStatus) String() string { return string(s) }

// ImportRun is the durable, tenant-scoped record of one inbound case
// import attempt through a ConnectorConfig (task 2): when it ran,
// which window it covered, how many InboundCase records the connector
// returned, how many were successfully mapped, and which external IDs
// (if any) failed. ImportRun stores summary/outcome data only -- the
// InboundCase payloads themselves are transient (mirroring this
// repository's transcribe-and-discard convention for binary ingestion
// artifacts) and are not persisted by this package once mapped onto a
// packages/caselifecycle.Case by the caller.
type ImportRun struct {
	// ID uniquely identifies this import run.
	ID uuid.UUID `json:"id"`

	// TenantID scopes this run to a tenant.
	TenantID uuid.UUID `json:"tenant_id"`

	// ConnectorConfigID identifies which ConnectorConfig this run used.
	ConnectorConfigID uuid.UUID `json:"connector_config_id"`

	// Since is the window start ImportCases was called with.
	Since time.Time `json:"since"`

	// Status is this run's outcome.
	Status ImportRunStatus `json:"status"`

	// ImportedCount is how many InboundCase records the connector
	// returned.
	ImportedCount int `json:"imported_count"`

	// MappedCount is how many of those were successfully translated by
	// a FieldMapping and accepted by the caller.
	MappedCount int `json:"mapped_count"`

	// FailedExternalIDs lists the InboundCase.ExternalID values that
	// failed mapping or downstream acceptance, for ImportRunStatusPartial
	// runs.
	FailedExternalIDs []string `json:"failed_external_ids,omitempty"`

	// ImportedExternalIDs lists every InboundCase.ExternalID the
	// connector returned in this run, used by Reconcile to compare
	// against an expected target set.
	ImportedExternalIDs []string `json:"imported_external_ids,omitempty"`

	// ErrorMessage carries the failure detail for
	// ImportRunStatusFailed runs. Empty otherwise.
	ErrorMessage string `json:"error_message,omitempty"`

	// StartedAt and FinishedAt bound the run's wall-clock duration.
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`

	// TriggeredBy is the identity.User (or system actor) who triggered
	// this run.
	TriggeredBy uuid.UUID `json:"triggered_by"`
}

// Validate checks r for structural well-formedness.
func (r *ImportRun) Validate() error {
	if r == nil {
		return ErrInvalidImportRun
	}
	if r.TenantID == uuid.Nil {
		return wrapf("ImportRun.Validate", ErrEmptyTenantID)
	}
	if r.ConnectorConfigID == uuid.Nil {
		return wrapf("ImportRun.Validate", ErrInvalidImportRun)
	}
	if !r.Status.IsValid() {
		return wrapf("ImportRun.Validate", ErrInvalidImportRun)
	}
	return nil
}

// summarizeImportRun derives Status/ImportedCount/MappedCount from the
// raw ImportCases result and per-case mapping outcomes, so callers
// never hand-roll this bookkeeping inconsistently. failedIDs must be a
// subset of the ExternalID values in cases.
func summarizeImportRun(cases []InboundCase, failedIDs []string) (status ImportRunStatus, importedIDs []string) {
	importedIDs = make([]string, 0, len(cases))
	for _, c := range cases {
		if strings.TrimSpace(c.ExternalID) != "" {
			importedIDs = append(importedIDs, c.ExternalID)
		}
	}
	switch {
	case len(failedIDs) == 0:
		status = ImportRunStatusSucceeded
	case len(failedIDs) < len(cases):
		status = ImportRunStatusPartial
	default:
		if len(cases) == 0 {
			status = ImportRunStatusSucceeded
		} else {
			status = ImportRunStatusPartial
		}
	}
	return status, importedIDs
}
