package compliance

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Framework names a legal/regulatory regime a Control may be mapped
// to. Deliberately an open string type, not a closed enum: a
// deployment's set of applicable frameworks varies by jurisdiction and
// customer, and a closed Go enum would force a code change (and a new
// phase) every time a tenant needed one more named framework. The
// constants below are the starter set this phase seeds
// (RegisterControl accepts any non-blank Framework, including ones not
// listed here) -- see doc/compliance.md for why this stays open.
type Framework string

const (
	// FrameworkUAEDataProtection covers the UAE's data-protection
	// regime (PDPL-style requirements: lawful basis for processing,
	// data-subject rights, cross-border transfer restriction, breach
	// notification). Controls mapped here describe requirement
	// *categories* this platform satisfies, not verbatim statutory
	// citations -- see doc/compliance.md for why.
	FrameworkUAEDataProtection Framework = "uae_data_protection"

	// FrameworkInternationalDataProtection covers international
	// data-protection frameworks used as a reference/applicable
	// overlay (e.g. GDPR-shaped requirement categories: lawful basis,
	// data-subject rights, breach notification, data minimization).
	// Mapped as an applicable reference overlay a deployment may opt
	// into via Profile, not as this platform's primary legal
	// basis.
	FrameworkInternationalDataProtection Framework = "international_data_protection"

	// FrameworkJudicialRecordsHandling covers requirements specific to
	// handling judicial/court records: retention schedules,
	// chain-of-custody, and the platform's own non-binding-analysis
	// guardrail (packages/guardrail).
	FrameworkJudicialRecordsHandling Framework = "judicial_records_handling"
)

// IsValid reports whether f is non-blank. Framework is deliberately
// open (see the type doc comment), so validity here means
// "structurally well-formed", not "one of the three named constants".
func (f Framework) IsValid() bool {
	return strings.TrimSpace(string(f)) != ""
}

// String satisfies fmt.Stringer.
func (f Framework) String() string { return string(f) }

// ControlCategory groups a Control by the kind of requirement it
// addresses, orthogonal to Framework (a single category, e.g.
// CategoryDataSubjectRights, commonly recurs across multiple
// frameworks).
type ControlCategory string

const (
	// CategoryLawfulBasis covers controls establishing and recording a
	// lawful basis for processing personal data.
	CategoryLawfulBasis ControlCategory = "lawful_basis"

	// CategoryDataSubjectRights covers controls implementing
	// data-subject rights: access, erasure, rectification, and consent
	// withdrawal.
	CategoryDataSubjectRights ControlCategory = "data_subject_rights"

	// CategoryCrossBorderTransfer covers controls restricting or
	// gating the transfer of personal data across jurisdictional
	// boundaries.
	CategoryCrossBorderTransfer ControlCategory = "cross_border_transfer"

	// CategoryBreachNotification covers controls detecting and
	// reporting personal-data breaches within a required window.
	CategoryBreachNotification ControlCategory = "breach_notification"

	// CategoryRecordRetention covers controls governing how long
	// records (judicial or otherwise) are retained before deletion or
	// archival.
	CategoryRecordRetention ControlCategory = "record_retention"

	// CategoryChainOfCustody covers controls preserving a tamper-evident
	// record of who accessed or modified a piece of content and when.
	CategoryChainOfCustody ControlCategory = "chain_of_custody"

	// CategoryNonBindingDisclaimer covers controls enforcing that
	// AI-generated reasoning output is labeled as a non-binding draft
	// analysis, never a verdict.
	CategoryNonBindingDisclaimer ControlCategory = "non_binding_disclaimer"

	// CategoryAccessControl covers controls governing who may perform
	// which operations (RBAC, tenant isolation, break-glass access).
	CategoryAccessControl ControlCategory = "access_control"

	// CategoryAuditability covers controls ensuring actions are
	// recorded in a durable, queryable trail.
	CategoryAuditability ControlCategory = "auditability"
)

// IsValid reports whether c is one of the named ControlCategory
// constants.
func (c ControlCategory) IsValid() bool {
	switch c {
	case CategoryLawfulBasis, CategoryDataSubjectRights, CategoryCrossBorderTransfer,
		CategoryBreachNotification, CategoryRecordRetention, CategoryChainOfCustody,
		CategoryNonBindingDisclaimer, CategoryAccessControl, CategoryAuditability:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (c ControlCategory) String() string { return string(c) }

// Control is a single catalogued compliance control (task 1): a named,
// framework-mapped requirement this platform can be evaluated against.
// A Control describes what must be true ("cross-border transfers of
// personal data are restricted and logged"); ControlEvidence
// (evidence.go) is what proves it actually is true for a given
// tenant/deployment.
type Control struct {
	// ID uniquely identifies this control.
	ID uuid.UUID `json:"id"`

	// Code is a short, stable, human-referenceable identifier (e.g.
	// "UAE-DP-01", "JRH-03"), distinct from ID (a uuid.UUID primary
	// key). Code is what a compliance report or dashboard displays;
	// ID is what foreign keys (ControlEvidence.ControlID) reference.
	Code string `json:"code"`

	// Title is a short human-readable name for this control.
	Title string `json:"title"`

	// Description explains what the control requires in plain
	// language. Framed as a mapped requirement category, never as a
	// verbatim statutory quotation this package cannot verify (see
	// doc/compliance.md).
	Description string `json:"description"`

	// Framework is the legal/regulatory regime this control is mapped
	// to.
	Framework Framework `json:"framework"`

	// Category classifies the kind of requirement this control
	// addresses.
	Category ControlCategory `json:"category"`

	// MappedTo lists, by string tag, the platform features this
	// control conceptually maps to (e.g. "packages/privacy.SAR",
	// "packages/auditlog", "packages/dataresidency"). Reference only:
	// this package does not import the tagged packages just to name
	// them, exactly as packages/privacy.DataInventoryEntry.SourceTag
	// references storage locations by convention rather than by
	// import.
	MappedTo []string `json:"mapped_to,omitempty"`

	// CreatedBy is the identity.User who catalogued this control.
	CreatedBy uuid.UUID `json:"created_by"`

	// CreatedAt and UpdatedAt are bookkeeping timestamps.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks c for structural well-formedness.
func (c *Control) Validate() error {
	if c == nil {
		return ErrInvalidControl
	}
	if strings.TrimSpace(c.Code) == "" {
		return wrapf("Control.Validate", ErrInvalidControl)
	}
	if strings.TrimSpace(c.Title) == "" {
		return wrapf("Control.Validate", ErrInvalidControl)
	}
	if !c.Framework.IsValid() {
		return wrapf("Control.Validate", ErrInvalidFramework)
	}
	if !c.Category.IsValid() {
		return wrapf("Control.Validate", ErrInvalidControl)
	}
	return nil
}
