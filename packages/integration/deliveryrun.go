package integration

import (
	"time"

	"github.com/google/uuid"
)

// DeliveryRunStatus classifies the outcome of one DeliveryRun (task
// 3).
type DeliveryRunStatus string

const (
	// DeliveryRunStatusAccepted means the external system accepted the
	// delivery (DeliveryReceipt.Accepted was true).
	DeliveryRunStatusAccepted DeliveryRunStatus = "accepted"

	// DeliveryRunStatusRejected means the external system responded
	// without error but declined the delivery (DeliveryReceipt.Accepted
	// was false) -- a soft rejection, not a call failure.
	DeliveryRunStatusRejected DeliveryRunStatus = "rejected"

	// DeliveryRunStatusFailed means the call itself failed (e.g.
	// Connector.DeliverReport returned an error after retries were
	// exhausted).
	DeliveryRunStatusFailed DeliveryRunStatus = "failed"
)

// IsValid reports whether s is one of the named DeliveryRunStatus
// constants.
func (s DeliveryRunStatus) IsValid() bool {
	switch s {
	case DeliveryRunStatusAccepted, DeliveryRunStatusRejected, DeliveryRunStatusFailed:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (s DeliveryRunStatus) String() string { return string(s) }

// DeliveryRun is the durable, tenant-scoped record of one outbound
// report delivery attempt through a ConnectorConfig (task 3): which
// report (by external case ID and kind), when it was attempted, and
// the DeliveryReceipt the external system returned. Like ImportRun,
// DeliveryRun stores summary/outcome data -- OutboundReport.Payload
// itself is not persisted by this package.
type DeliveryRun struct {
	// ID uniquely identifies this delivery run.
	ID uuid.UUID `json:"id"`

	// TenantID scopes this run to a tenant.
	TenantID uuid.UUID `json:"tenant_id"`

	// ConnectorConfigID identifies which ConnectorConfig this run used.
	ConnectorConfigID uuid.UUID `json:"connector_config_id"`

	// CaseExternalID is the OutboundReport.CaseExternalID this run
	// delivered.
	CaseExternalID string `json:"case_external_id"`

	// ReportKind mirrors OutboundReport.ReportKind.
	ReportKind string `json:"report_kind"`

	// Status is this run's outcome.
	Status DeliveryRunStatus `json:"status"`

	// ExternalReceiptID mirrors DeliveryReceipt.ExternalReceiptID, if
	// any.
	ExternalReceiptID string `json:"external_receipt_id,omitempty"`

	// Detail carries DeliveryReceipt.Detail or a failure message,
	// depending on Status.
	Detail string `json:"detail,omitempty"`

	// AttemptCount is how many attempts WithRetry made before this run
	// reached its final Status.
	AttemptCount int `json:"attempt_count"`

	// StartedAt and FinishedAt bound the run's wall-clock duration.
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`

	// TriggeredBy is the identity.User (or system actor) who triggered
	// this run.
	TriggeredBy uuid.UUID `json:"triggered_by"`
}

// Validate checks r for structural well-formedness.
func (r *DeliveryRun) Validate() error {
	if r == nil {
		return ErrInvalidDeliveryRun
	}
	if r.TenantID == uuid.Nil {
		return wrapf("DeliveryRun.Validate", ErrEmptyTenantID)
	}
	if r.ConnectorConfigID == uuid.Nil {
		return wrapf("DeliveryRun.Validate", ErrInvalidDeliveryRun)
	}
	if !r.Status.IsValid() {
		return wrapf("DeliveryRun.Validate", ErrInvalidDeliveryRun)
	}
	return nil
}

// statusFromReceipt derives a DeliveryRunStatus from a successful
// DeliverReport call's receipt (a call failure is handled separately
// by the caller, which never reaches this helper).
func statusFromReceipt(receipt DeliveryReceipt) DeliveryRunStatus {
	if receipt.Accepted {
		return DeliveryRunStatusAccepted
	}
	return DeliveryRunStatusRejected
}
