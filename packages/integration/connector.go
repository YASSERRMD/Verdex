package integration

import (
	"context"
	"time"
)

// Connector is the contract every concrete external-system adapter
// must satisfy (task 1). Verdex routes all court case-management
// system calls through this interface so that adapters for different
// courts/registries can be registered and swapped without touching
// business logic -- mirroring exactly how
// packages/provider.LLMProvider lets model-provider adapters be
// registered and swapped, just applied to external case-management
// systems instead of model providers.
//
// Implementations MUST be safe for concurrent use from multiple
// goroutines.
type Connector interface {
	// ID returns the stable, unique identifier for this connector
	// instance (e.g. "efiling-dubai-courts", "sandbox"). The value must
	// be non-empty and match the key used when registering with the
	// Registry.
	ID() string

	// Capabilities returns the static capability descriptor for this
	// connector. The returned value must not be mutated by the caller.
	Capabilities() ConnectorCapability

	// ImportCases retrieves cases from the external system that were
	// created or updated at or after since, in the external system's
	// own representation (InboundCase) -- callers apply a FieldMapping
	// to convert InboundCase.Fields into this platform's case/party/
	// document shape (see packages/caselifecycle.Case by tag, not
	// import). A zero since means "import everything the external
	// system will return".
	ImportCases(ctx context.Context, since time.Time) ([]InboundCase, error)

	// DeliverReport sends an OutboundReport (typically built from
	// packages/reportexport.Report by tag, not import) to the external
	// system and returns a DeliveryReceipt confirming what the external
	// system accepted.
	DeliverReport(ctx context.Context, report OutboundReport) (DeliveryReceipt, error)

	// Ping verifies that the external system's endpoint is reachable
	// and the configured credentials are accepted. It should be fast (a
	// single lightweight call) and honour ctx for timeouts. Callers
	// should Ping before relying on ImportCases/DeliverReport, exactly
	// as packages/provider.LLMProvider.HealthCheck is used before
	// relying on Chat/Embed.
	Ping(ctx context.Context) error
}

// ConnectorCapability describes what a specific connector can do,
// mirroring packages/provider.Capability's descriptor shape applied to
// court case-management systems instead of model providers.
type ConnectorCapability struct {
	// ConnectorID identifies the connector (e.g. "efiling-dubai-courts").
	ConnectorID string `json:"connector_id"`

	// SystemName is a human-readable name for the external system
	// (e.g. "Dubai Courts e-Filing Portal").
	SystemName string `json:"system_name"`

	// SupportsImport reports whether ImportCases is functional for this
	// connector.
	SupportsImport bool `json:"supports_import"`

	// SupportsDelivery reports whether DeliverReport is functional for
	// this connector.
	SupportsDelivery bool `json:"supports_delivery"`

	// MaxBatchSize is the maximum number of InboundCase records
	// ImportCases returns in one call. Zero means unbounded/unknown.
	MaxBatchSize int `json:"max_batch_size"`

	// Region declares the jurisdiction/locality this connector's
	// upstream system is deployed in (e.g. "ae", "uk", "local"),
	// mirroring packages/provider.Capability.Region's doc comment on
	// treating an empty Region as failing any data-residency allow-list
	// check rather than silently passing it.
	Region string `json:"region,omitempty"`
}

// InboundCase is a single case record as returned by a Connector's
// ImportCases call, in the external system's own field representation
// -- deliberately not packages/caselifecycle.Case itself, since the
// external system's schema is not this platform's schema. A
// FieldMapping (fieldmapping.go) converts Fields into this platform's
// shape; this package references packages/caselifecycle.Case's
// ID/TenantID/JurisdictionID/CategoryID/Title/Reference/State/Metadata
// fields by name only, never by import, keeping this package's
// dependency footprint thin (mirroring how
// packages/accessgovernance.CaseGrant references
// packages/caselifecycle.Case by CaseID only).
type InboundCase struct {
	// ExternalID is the external system's own identifier for this case
	// (its docket/case number or internal ID) -- distinct from any
	// packages/caselifecycle.Case.ID this platform later assigns.
	ExternalID string `json:"external_id"`

	// ExternalUpdatedAt is when the external system last modified this
	// record, used by ImportRun/Reconcile to detect drift and by
	// ImportCases(since) callers to avoid re-processing unchanged
	// cases.
	ExternalUpdatedAt time.Time `json:"external_updated_at"`

	// Fields carries the external system's raw field values keyed by
	// its own field names (e.g. "docket_no", "case_title",
	// "filing_date"). A FieldMapping's Apply converts this into
	// MappedFields addressed by this platform's field names.
	Fields map[string]string `json:"fields"`
}

// OutboundReport is what DeliverReport sends to the external system --
// deliberately not packages/reportexport.Report itself. This package
// references packages/reportexport.Report's
// CaseID/TenantID/CaseTitle/CaseReference/JurisdictionKey fields by
// name only (see doc.go), so OutboundReport carries just enough to
// address and describe the delivery; callers build Payload from a
// Report using whichever render format
// (packages/reportexport.ExportResult, e.g. PDF/DOCX/Markdown bytes)
// the external system expects.
type OutboundReport struct {
	// CaseExternalID is the external system's identifier for the case
	// this report concerns (typically the ExternalID an earlier
	// InboundCase carried, or a value a FieldMapping.Reverse produced
	// from this platform's case reference).
	CaseExternalID string `json:"case_external_id"`

	// ReportKind is a free-form label describing what kind of report
	// this is (e.g. "opinion_summary", "compliance_export"), so a
	// connector or its audit trail can distinguish report types without
	// this package modeling a closed enum of every possible report a
	// deployment might send.
	ReportKind string `json:"report_kind"`

	// Format names the byte encoding of Payload (e.g. "pdf", "docx",
	// "markdown"), mirroring packages/reportexport's own render
	// functions (pdf.go, docx.go, markdown.go).
	Format string `json:"format"`

	// Payload is the rendered report bytes.
	Payload []byte `json:"payload"`

	// Metadata is a free-form set of additional fields the external
	// system's delivery API may require (e.g. a cover-letter subject, a
	// filing category code).
	Metadata map[string]string `json:"metadata,omitempty"`
}

// DeliveryReceipt confirms what the external system accepted for one
// DeliverReport call.
type DeliveryReceipt struct {
	// ExternalReceiptID is the external system's own confirmation
	// identifier for this delivery (a filing receipt number, a message
	// ID, etc.), empty if the external system does not issue one.
	ExternalReceiptID string `json:"external_receipt_id"`

	// AcceptedAt is when the external system acknowledged the
	// delivery.
	AcceptedAt time.Time `json:"accepted_at"`

	// Accepted reports whether the external system accepted the
	// delivery. A Connector implementation must return a non-nil error
	// from DeliverReport (not merely Accepted=false) for hard failures;
	// Accepted=false with a nil error models a soft/provisional
	// rejection the external system itself reports (e.g. "queued for
	// manual review") rather than a call failure.
	Accepted bool `json:"accepted"`

	// Detail is a free-form human-readable message from the external
	// system (e.g. a rejection reason).
	Detail string `json:"detail,omitempty"`
}
