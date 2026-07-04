package threatmodel

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// StrideCategory classifies a Threat using the STRIDE taxonomy
// (Spoofing, Tampering, Repudiation, Information Disclosure, Denial of
// Service, Elevation of Privilege). Deliberately a closed enum, unlike
// packages/compliance.Framework: STRIDE is a fixed, well-known
// taxonomy, not a set that varies by jurisdiction or customer, so
// there is no reason to leave it open the way Framework is.
type StrideCategory string

const (
	// StrideSpoofing covers threats where an actor impersonates
	// another user, service, or component.
	StrideSpoofing StrideCategory = "spoofing"

	// StrideTampering covers threats where data or code is modified
	// without authorization, in transit or at rest.
	StrideTampering StrideCategory = "tampering"

	// StrideRepudiation covers threats where an actor denies having
	// performed an action and the platform cannot prove otherwise.
	StrideRepudiation StrideCategory = "repudiation"

	// StrideInformationDisclosure covers threats where information is
	// exposed to an actor not authorized to see it.
	StrideInformationDisclosure StrideCategory = "information_disclosure"

	// StrideDenialOfService covers threats where a service is degraded
	// or made unavailable to legitimate users.
	StrideDenialOfService StrideCategory = "denial_of_service"

	// StrideElevationOfPrivilege covers threats where an actor gains
	// capabilities beyond what they were granted.
	StrideElevationOfPrivilege StrideCategory = "elevation_of_privilege"
)

// IsValid reports whether c is one of the named StrideCategory
// constants.
func (c StrideCategory) IsValid() bool {
	switch c {
	case StrideSpoofing, StrideTampering, StrideRepudiation, StrideInformationDisclosure,
		StrideDenialOfService, StrideElevationOfPrivilege:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (c StrideCategory) String() string { return string(c) }

// Severity ranks how serious a Threat is if left unmitigated. A closed
// enum: severity is an internal risk-rating scale this package
// defines and owns, not an open/extensible taxonomy.
type Severity string

const (
	// SeverityLow means limited impact and/or very low likelihood.
	SeverityLow Severity = "low"

	// SeverityMedium means moderate impact or likelihood.
	SeverityMedium Severity = "medium"

	// SeverityHigh means significant impact, plausible likelihood.
	SeverityHigh Severity = "high"

	// SeverityCritical means severe impact (e.g. cross-tenant data
	// exposure, full compromise) and must be tracked to a Mitigation
	// with status at least MitigationImplemented before a component
	// ships.
	SeverityCritical Severity = "critical"
)

// IsValid reports whether s is one of the named Severity constants.
func (s Severity) IsValid() bool {
	switch s {
	case SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (s Severity) String() string { return string(s) }

// rank returns s's position in the low-to-critical ordering, used by
// comparison helpers (e.g. sorting a ThreatModel's Threats by
// descending severity in reporting code).
func (s Severity) rank() int {
	switch s {
	case SeverityLow:
		return 0
	case SeverityMedium:
		return 1
	case SeverityHigh:
		return 2
	case SeverityCritical:
		return 3
	}
	return -1
}

// MitigationStatus tracks a Mitigation's real-world implementation
// state, distinct from whether the *catalogue entry describing it* has
// been merged. A Mitigation can be catalogued (the Go struct literal
// exists in seed.go) while its status is still MitigationPlanned --
// the catalogue documents intent to mitigate, the status documents
// whether that intent has actually been realized and checked.
type MitigationStatus string

const (
	// MitigationPlanned means the mitigation has been identified and
	// catalogued but the referenced control does not yet exist or is
	// not yet wired up.
	MitigationPlanned MitigationStatus = "planned"

	// MitigationImplemented means the referenced control exists and is
	// in place, but has not been independently verified as effective
	// against this specific Threat.
	MitigationImplemented MitigationStatus = "implemented"

	// MitigationVerified means the referenced control has been
	// independently verified (e.g. by a passing test named in
	// ReferenceTag, a security review, or a penetration test) to
	// actually mitigate this Threat.
	MitigationVerified MitigationStatus = "verified"
)

// IsValid reports whether s is one of the named MitigationStatus
// constants.
func (s MitigationStatus) IsValid() bool {
	switch s {
	case MitigationPlanned, MitigationImplemented, MitigationVerified:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (s MitigationStatus) String() string { return string(s) }

// rank returns s's position in the planned-to-verified ordering, used
// by CanTransitionMitigation to enforce forward-only progress.
func (s MitigationStatus) rank() int {
	switch s {
	case MitigationPlanned:
		return 0
	case MitigationImplemented:
		return 1
	case MitigationVerified:
		return 2
	}
	return -1
}

// allowedMitigationTransitions maps each MitigationStatus to the set
// of statuses a transition may move to, mirroring
// packages/privacy.SARStatus's allowedTransitions-map + CanTransition-
// guard shape by reference (this package does not import
// packages/privacy). Regression (e.g. Verified -> Planned) is not a
// blanket transition: it is only reachable via ResetMitigation, used
// when a previously verified control is found to have regressed and
// must be re-verified from scratch, never a silent same-map entry.
var allowedMitigationTransitions = map[MitigationStatus][]MitigationStatus{
	MitigationPlanned:      {MitigationImplemented},
	MitigationImplemented:  {MitigationVerified, MitigationPlanned},
	MitigationVerified:     {},
}

// CanTransitionMitigation reports whether from -> to is a permitted
// mitigation status transition. Verified is terminal via this
// function -- a regression from Verified must go through
// ResetMitigation (engine.go), which records an explicit audited
// reason, rather than silently allowing Verified -> Implemented here.
func CanTransitionMitigation(from, to MitigationStatus) bool {
	for _, allowed := range allowedMitigationTransitions[from] {
		if allowed == to {
			return true
		}
	}
	return false
}

// Component names a platform component/service a ThreatModel
// describes, by tag/name only -- e.g. "gateway", "ingestion",
// "reasoning-orchestration" -- never by importing the tagged package,
// exactly mirroring packages/compliance.Control.MappedTo's convention.
type Component struct {
	// Name is the short, stable tag identifying this component (e.g.
	// "gateway", "ingestion", "reasoning-orchestration").
	Name string `json:"name"`

	// PackageTag names, by convention, the real package this component
	// corresponds to (e.g. "packages/gateway"). Reference only -- this
	// package does not import PackageTag's named package.
	PackageTag string `json:"package_tag"`

	// Description is a short human-readable summary of what this
	// component does and why it is in scope for threat modeling.
	Description string `json:"description"`
}

// IsValid reports whether c is structurally well-formed.
func (c Component) IsValid() bool {
	return strings.TrimSpace(c.Name) != "" && strings.TrimSpace(c.PackageTag) != ""
}

// Mitigation is a single control that reduces or eliminates a Threat.
// Mirrors packages/compliance.Control.MappedTo's string-tag convention
// via ReferenceTag: this package never imports whatever ReferenceTag
// names.
type Mitigation struct {
	// ID uniquely identifies this mitigation.
	ID uuid.UUID `json:"id"`

	// Title is a short human-readable name for this mitigation.
	Title string `json:"title"`

	// Description explains what the mitigation does and how it
	// reduces the associated Threat.
	Description string `json:"description"`

	// Status tracks this mitigation's real-world implementation state.
	Status MitigationStatus `json:"status"`

	// ReferenceTag names, by string convention, the real control that
	// implements this mitigation (e.g.
	// "packages/identity.RequirePermission",
	// "packages/encryption.TLS", "packages/guardrail.CheckText").
	// Reference only -- this package does not import the tagged
	// package. Must be non-blank: an uncatalogued mitigation with no
	// pointer back to an actual control is not a real mitigation, just
	// an aspiration.
	ReferenceTag string `json:"reference_tag"`
}

// Validate checks m for structural well-formedness.
func (m Mitigation) Validate() error {
	if strings.TrimSpace(m.Title) == "" {
		return wrapf("Mitigation.Validate", ErrInvalidMitigation)
	}
	if !m.Status.IsValid() {
		return wrapf("Mitigation.Validate", ErrInvalidMitigation)
	}
	if strings.TrimSpace(m.ReferenceTag) == "" {
		return wrapf("Mitigation.Validate", ErrInvalidMitigation)
	}
	return nil
}

// Threat is a single catalogued threat against a Component: a STRIDE
// category, a severity, and one or more Mitigations.
type Threat struct {
	// ID uniquely identifies this threat.
	ID uuid.UUID `json:"id"`

	// Title is a short human-readable name for this threat.
	Title string `json:"title"`

	// Description explains the threat scenario in plain language: who
	// the attacker is, what they do, and what the impact would be.
	Description string `json:"description"`

	// Category is this threat's STRIDE classification.
	Category StrideCategory `json:"category"`

	// Severity ranks how serious this threat is if left unmitigated.
	Severity Severity `json:"severity"`

	// Mitigations lists the controls that reduce or eliminate this
	// threat. A threat with zero Mitigations is a genuine gap --
	// Validate does not require at least one, so an incompletely
	// mitigated threat can still be catalogued and surfaced by
	// UnmitigatedThreats rather than being impossible to represent.
	Mitigations []Mitigation `json:"mitigations,omitempty"`
}

// Validate checks t for structural well-formedness, including every
// element of Mitigations.
func (t Threat) Validate() error {
	if strings.TrimSpace(t.Title) == "" {
		return wrapf("Threat.Validate", ErrInvalidThreat)
	}
	if !t.Category.IsValid() {
		return wrapf("Threat.Validate", ErrInvalidThreat)
	}
	if !t.Severity.IsValid() {
		return wrapf("Threat.Validate", ErrInvalidThreat)
	}
	for _, m := range t.Mitigations {
		if err := m.Validate(); err != nil {
			return wrapf("Threat.Validate", err)
		}
	}
	return nil
}

// StrongestMitigationStatus returns the highest-ranked MitigationStatus
// among t's Mitigations, and false if t has no Mitigations at all --
// used by gap-style reporting to classify a threat as unmitigated
// (no Mitigations), partially mitigated (best status below Verified),
// or fully mitigated (best status is Verified).
func (t Threat) StrongestMitigationStatus() (MitigationStatus, bool) {
	if len(t.Mitigations) == 0 {
		return "", false
	}
	best := t.Mitigations[0].Status
	for _, m := range t.Mitigations[1:] {
		if m.Status.rank() > best.rank() {
			best = m.Status
		}
	}
	return best, true
}

// ThreatModel is a single named platform component/service's full
// threat catalogue: the Component it describes plus every catalogued
// Threat against it (task 1).
type ThreatModel struct {
	// ID uniquely identifies this threat model.
	ID uuid.UUID `json:"id"`

	// Component is the platform component/service this threat model
	// describes.
	Component Component `json:"component"`

	// Threats lists every catalogued Threat against Component.
	Threats []Threat `json:"threats"`

	// CreatedAt and UpdatedAt are bookkeeping timestamps. Since a
	// ThreatModel is versioned-in-code data (see doc.go's persistence
	// discussion), these track when the seed/catalogue entry was
	// authored, not a runtime mutation.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks tm for structural well-formedness, including its
// Component and every element of Threats.
func (tm ThreatModel) Validate() error {
	if !tm.Component.IsValid() {
		return wrapf("ThreatModel.Validate", ErrInvalidComponent)
	}
	if len(tm.Threats) == 0 {
		return wrapf("ThreatModel.Validate", ErrInvalidThreatModel)
	}
	for _, t := range tm.Threats {
		if err := t.Validate(); err != nil {
			return wrapf("ThreatModel.Validate", err)
		}
	}
	return nil
}

// UnmitigatedThreats returns every Threat in tm with zero Mitigations
// catalogued -- a genuine, visible gap rather than a silently dropped
// one.
func (tm ThreatModel) UnmitigatedThreats() []Threat {
	out := make([]Threat, 0)
	for _, t := range tm.Threats {
		if len(t.Mitigations) == 0 {
			out = append(out, t)
		}
	}
	return out
}

// ThreatsBySeverity returns every Threat in tm carrying severity,
// convenience for reporting code that wants to drill into (e.g.)
// every SeverityCritical threat across a catalogue.
func (tm ThreatModel) ThreatsBySeverity(severity Severity) []Threat {
	out := make([]Threat, 0)
	for _, t := range tm.Threats {
		if t.Severity == severity {
			out = append(out, t)
		}
	}
	return out
}
