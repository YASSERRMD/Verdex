package jurisdiction

import (
	"time"

	"github.com/google/uuid"
)

// ProceduralRule describes a specific procedural code or statute that governs
// practice before a court in this jurisdiction.
type ProceduralRule struct {
	// Code is a short machine-readable identifier (e.g. "CPC", "CrPC", "DIFC-RDC").
	Code string `json:"code"`

	// Name is the human-readable name of the procedural instrument.
	Name string `json:"name"`

	// Description provides a brief overview of the rule's scope and applicability.
	Description string `json:"description"`
}

// Jurisdiction represents a court or judicial authority within a national or
// sub-national legal system.
type Jurisdiction struct {
	// ID is the globally unique identifier for the jurisdiction record.
	ID uuid.UUID `json:"id"`

	// CountryCode is the ISO 3166-1 alpha-2 country code (e.g. "AE", "PK", "IN").
	CountryCode string `json:"country_code"`

	// CountryName is the full English name of the sovereign state.
	CountryName string `json:"country_name"`

	// CourtLevel indicates the tier of the court within the national hierarchy.
	CourtLevel CourtLevel `json:"court_level"`

	// CourtName is the official name of the court or judicial authority.
	CourtName string `json:"court_name"`

	// LegalFamily classifies the primary legal tradition applicable to this court.
	LegalFamily LegalFamily `json:"legal_family"`

	// Languages lists the official language codes (ISO 639-1) used in proceedings
	// before this court (e.g. ["ar", "en"]).
	Languages []string `json:"languages"`

	// ProceduralRules lists the procedural codes or statutes that govern practice
	// before this court.
	ProceduralRules []ProceduralRule `json:"procedural_rules,omitempty"`

	// CreatedAt is the timestamp when this record was first persisted.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is the timestamp of the most recent modification to this record.
	UpdatedAt time.Time `json:"updated_at"`
}
