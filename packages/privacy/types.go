package privacy

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// DataCategory classifies a category of personal data this package's
// inventory, retention, and erasure machinery reasons about (task 1).
// Distinct from packages/pii.PIICategory: a PIICategory classifies a
// single detected span of text within a document ("this substring is
// an email address"); a DataCategory classifies a whole class of
// stored personal data at the system level ("case.parties holds
// contact-shaped personal data for every case in the tenant"). Where a
// DataCategory's content is fundamentally text that packages/pii can
// detect within, AnonymizeForAnalytics (anonymize.go) maps it onto the
// corresponding pii.PIICategory rather than re-deriving a parallel
// classification.
type DataCategory string

const (
	// CategoryIdentity covers personal data held about registered
	// platform users and their accounts (identity.User's Email, Name,
	// and similar fields; referenced by tag as "identity.user").
	CategoryIdentity DataCategory = "identity"

	// CategoryCaseParty covers personal data about the parties,
	// witnesses, and other named individuals appearing within a case's
	// substantive record (referenced by tag as "case.parties").
	CategoryCaseParty DataCategory = "case_party"

	// CategoryContact covers direct contact-channel data: email
	// addresses, phone numbers, and postal addresses, wherever stored
	// (referenced by tag as e.g. "case.parties", "identity.user").
	CategoryContact DataCategory = "contact"

	// CategoryIdentifier covers government- or institution-issued
	// identifier numbers embedded in case content: national IDs,
	// passport numbers, and similar (referenced by tag as
	// "case.parties").
	CategoryIdentifier DataCategory = "identifier"

	// CategoryFinancial covers financial account identifiers appearing
	// in case content: bank account numbers, card numbers, IBANs, and
	// similar (referenced by tag as "case.parties").
	CategoryFinancial DataCategory = "financial"

	// CategoryTranscript covers raw or lightly processed transcribed
	// speech/audio content captured during intake, which frequently
	// contains unredacted personal data before packages/pii's pipeline
	// runs over it (referenced by tag as "ingestion.transcript").
	CategoryTranscript DataCategory = "transcript"

	// CategoryBehavioral covers usage/analytics data describing how a
	// data subject interacted with the platform (referenced by tag as
	// "analytics.usage").
	CategoryBehavioral DataCategory = "behavioral"

	// CategoryOther covers personal data that does not fit a more
	// specific category above.
	CategoryOther DataCategory = "other"
)

// IsValid reports whether c is one of the named DataCategory
// constants.
func (c DataCategory) IsValid() bool {
	switch c {
	case CategoryIdentity, CategoryCaseParty, CategoryContact, CategoryIdentifier,
		CategoryFinancial, CategoryTranscript, CategoryBehavioral, CategoryOther:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (c DataCategory) String() string { return string(c) }

// Sensitivity classifies how sensitive a DataCategory's content is,
// driving the strength of the DeletionAction a RetentionPolicy assigns
// it and the redaction/anonymization applied by AnonymizeForAnalytics.
// Consistent in spirit with packages/pii's category taxonomy, but
// closed over sensitivity tiers rather than PII kind.
type Sensitivity string

const (
	// SensitivityLow covers data with minimal re-identification risk
	// on its own (e.g. aggregate behavioral counters).
	SensitivityLow Sensitivity = "low"

	// SensitivityMedium covers data that identifies a person but
	// carries limited harm potential on disclosure (e.g. a name alone).
	SensitivityMedium Sensitivity = "medium"

	// SensitivityHigh covers data whose disclosure creates meaningful
	// harm potential (e.g. contact details, case-party identifiers).
	SensitivityHigh Sensitivity = "high"

	// SensitivityCritical covers data requiring the strongest handling
	// (e.g. financial account numbers, government identifiers).
	SensitivityCritical Sensitivity = "critical"
)

// IsValid reports whether s is one of the named Sensitivity constants.
func (s Sensitivity) IsValid() bool {
	switch s {
	case SensitivityLow, SensitivityMedium, SensitivityHigh, SensitivityCritical:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (s Sensitivity) String() string { return string(s) }

// LegalBasis names the lawful ground under which personal data in a
// DataInventoryEntry is processed, or under which a ConsentRecord was
// granted. Modeled as a plain closed string type (not an import from
// any jurisdiction-specific regulation package) so this package stays
// usable across jurisdictions with differing legal-basis vocabularies.
type LegalBasis string

const (
	// BasisConsent means the data subject affirmatively consented to
	// this processing purpose. The authoritative record of that
	// consent is a ConsentRecord (consent.go); a DataInventoryEntry
	// with this basis is a claim that consent-gated purposes exist
	// somewhere in the tenant's ConsentRecord set, not a substitute for
	// checking one.
	BasisConsent LegalBasis = "consent"

	// BasisContract means processing is necessary to perform a
	// contract the data subject is party to (e.g. platform terms of
	// service).
	BasisContract LegalBasis = "contract"

	// BasisLegalObligation means processing is required to comply with
	// a legal obligation the tenant is subject to (e.g. statutory
	// record-keeping for judicial proceedings).
	BasisLegalObligation LegalBasis = "legal_obligation"

	// BasisLegitimateInterest means processing is necessary for a
	// legitimate interest of the tenant or a third party, balanced
	// against the data subject's rights.
	BasisLegitimateInterest LegalBasis = "legitimate_interest"

	// BasisPublicTask means processing is necessary for the
	// performance of a task carried out in the public interest (e.g. a
	// judicial or administrative proceeding).
	BasisPublicTask LegalBasis = "public_task"

	// BasisVitalInterest means processing is necessary to protect the
	// vital interests of the data subject or another natural person.
	BasisVitalInterest LegalBasis = "vital_interest"
)

// IsValid reports whether b is one of the named LegalBasis constants.
func (b LegalBasis) IsValid() bool {
	switch b {
	case BasisConsent, BasisContract, BasisLegalObligation, BasisLegitimateInterest, BasisPublicTask, BasisVitalInterest:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (b LegalBasis) String() string { return string(b) }

// DataInventoryEntry is a single registry row (task 1) describing what
// personal-data category is stored, where, on what legal basis, and
// for how long. SourceTag names the storing location by convention
// (e.g. "case.parties", "identity.user", "ingestion.transcript") --
// this package does not import the referenced packages, exactly as
// packages/accessgovernance.CaseGrant references
// packages/caselifecycle.Case by CaseID only.
type DataInventoryEntry struct {
	// ID uniquely identifies this inventory entry.
	ID uuid.UUID `json:"id"`

	// TenantID is the tenant this inventory entry belongs to.
	TenantID uuid.UUID `json:"tenant_id"`

	// Category classifies the kind of personal data this entry
	// describes.
	Category DataCategory `json:"category"`

	// SourceTag names, by convention, the system location this
	// category of data is stored -- e.g. "case.parties",
	// "identity.user", "ingestion.transcript". Reference only; no
	// package import is implied or required.
	SourceTag string `json:"source_tag"`

	// Sensitivity classifies how sensitive this category's content is.
	Sensitivity Sensitivity `json:"sensitivity"`

	// LegalBasis is the lawful ground for processing this category of
	// data at SourceTag.
	LegalBasis LegalBasis `json:"legal_basis"`

	// RetentionPeriod is how long this category's data is retained
	// from its creation/collection time before EnforceRetention
	// reports it due for a DeletionAction. Must be positive.
	RetentionPeriod time.Duration `json:"retention_period"`

	// Description is a short human-readable note about what this entry
	// covers (e.g. "party names and addresses attached to filings").
	Description string `json:"description,omitempty"`

	// CreatedBy is the identity.User who registered this entry.
	CreatedBy uuid.UUID `json:"created_by"`

	// CreatedAt and UpdatedAt are bookkeeping timestamps.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks e for structural well-formedness.
func (e *DataInventoryEntry) Validate() error {
	if e == nil {
		return ErrInvalidInventoryEntry
	}
	if e.TenantID == uuid.Nil {
		return ErrEmptyTenantID
	}
	if !e.Category.IsValid() {
		return wrapf("DataInventoryEntry.Validate", ErrInvalidDataCategory)
	}
	if strings.TrimSpace(e.SourceTag) == "" {
		return wrapf("DataInventoryEntry.Validate", ErrInvalidInventoryEntry)
	}
	if !e.Sensitivity.IsValid() {
		return wrapf("DataInventoryEntry.Validate", ErrInvalidSensitivity)
	}
	if !e.LegalBasis.IsValid() {
		return wrapf("DataInventoryEntry.Validate", ErrInvalidInventoryEntry)
	}
	if e.RetentionPeriod <= 0 {
		return wrapf("DataInventoryEntry.Validate", ErrInvalidInventoryEntry)
	}
	return nil
}
