package dataresidency

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// DataClassRule narrows AllowedRegions further for a specific class of
// data (e.g. "pii", "case_document", "privileged"), letting a
// deployment declare that most data may stay within a broad region set
// while a more sensitive class is pinned to a narrower one. A
// DataClassRule with an empty AllowedRegions list falls back to the
// enclosing ResidencyPolicy.AllowedRegions.
type DataClassRule struct {
	// DataClass names the class this rule governs (e.g. "pii").
	DataClass string `json:"data_class"`

	// AllowedRegions restricts this data class to a subset (or an
	// unrelated set) of the policy's AllowedRegions. Empty means "defer
	// to the policy-level AllowedRegions".
	AllowedRegions []string `json:"allowed_regions,omitempty"`
}

// ResidencyPolicy is the residency/sovereignty policy attached to a
// tenancy deployment (packages/persistence.Deployment /
// packages/tenancy's deployment concept). It does not replace or
// duplicate Deployment -- it is a separate value keyed by DeploymentID
// that composes with it, mirroring how packages/keymanagement's
// KeyMetadata is keyed by TenantID rather than embedded inside a
// tenancy type.
type ResidencyPolicy struct {
	// DeploymentID identifies the deployment this policy governs.
	DeploymentID uuid.UUID `json:"deployment_id"`

	// AllowedRegions lists the region codes (e.g. "eu", "us", "local")
	// data belonging to this deployment may be stored or processed in.
	// An empty list combined with StrictMode=true is the air-gapped
	// preset (see AirGappedPreset): no region is allowed at all except
	// through router's local-only path, which never leaves the
	// deployment's own host.
	AllowedRegions []string `json:"allowed_regions"`

	// DataClassRules optionally narrows specific data classes to a
	// tighter region set than AllowedRegions.
	DataClassRules []DataClassRule `json:"data_class_rules,omitempty"`

	// StrictMode, when true, forbids ANY cross-region operation, even
	// between two regions that both appear in AllowedRegions. This is
	// stricter than a plain allow-list: AllowedRegions says "these
	// regions are acceptable places for data to live"; StrictMode says
	// "data must never move between regions at all, period" (used by
	// the air-gapped preset and by deployments with a single pinned
	// home region).
	StrictMode bool `json:"strict_mode"`

	// CreatedAt and UpdatedAt are informational bookkeeping fields; this
	// package does not require a database to construct or evaluate a
	// ResidencyPolicy in memory, but a caller that persists one (e.g.
	// via a repository composed on top of packages/persistence) will
	// want them populated.
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// Validate checks p for internal consistency. It does not check
// against live configuration (see Verify for that).
func (p *ResidencyPolicy) Validate() error {
	if p == nil {
		return ErrNilPolicy
	}
	if p.DeploymentID == uuid.Nil {
		return ErrEmptyDeploymentID
	}
	for _, r := range p.AllowedRegions {
		if strings.TrimSpace(r) == "" {
			return wrapf("Validate", ErrEmptyRegion)
		}
	}
	for _, rule := range p.DataClassRules {
		if strings.TrimSpace(rule.DataClass) == "" {
			return wrapf("Validate", errEmptyDataClass)
		}
	}
	return nil
}

// AllowsRegion reports whether region is permitted by p's
// AllowedRegions list. An empty AllowedRegions list allows nothing --
// this package fails closed, matching the air-gapped preset's intent
// (no region is trusted unless explicitly declared).
func (p *ResidencyPolicy) AllowsRegion(region string) bool {
	if p == nil || region == "" {
		return false
	}
	for _, r := range p.AllowedRegions {
		if strings.EqualFold(r, region) {
			return true
		}
	}
	return false
}

// AllowedRegionsFor returns the effective allowed-region list for
// dataClass: the matching DataClassRule's AllowedRegions if one exists
// and is non-empty, otherwise p's policy-level AllowedRegions.
func (p *ResidencyPolicy) AllowedRegionsFor(dataClass string) []string {
	if p == nil {
		return nil
	}
	for _, rule := range p.DataClassRules {
		if strings.EqualFold(rule.DataClass, dataClass) && len(rule.AllowedRegions) > 0 {
			return rule.AllowedRegions
		}
	}
	return p.AllowedRegions
}

// AirGappedPreset returns a ResidencyPolicy suitable for a fully
// air-gapped deployment: StrictMode is true and AllowedRegions is
// empty, so every region-based check fails closed. Phase 078 scope
// stops at this policy-side preset; Phase 079 builds the full offline
// deployment tier around it. Composing this preset with
// router.RoutingPolicy.AirGappedOnly is the caller's responsibility --
// see ComposeWithRouterAirGap in airgap.go, which validates that
// composition explicitly.
func AirGappedPreset(deploymentID uuid.UUID) ResidencyPolicy {
	return ResidencyPolicy{
		DeploymentID:   deploymentID,
		AllowedRegions: []string{},
		StrictMode:     true,
	}
}

// RegionPin pins a deployment's storage locality: the region its
// database connection is expected to live in, and the host pattern(s)
// that resolve to that region. This is a real, testable check (task
// 2) -- Validate parses the configured DSN's host and asserts it
// matches one of HostPatterns, rather than trusting a comment.
type RegionPin struct {
	// DeploymentID identifies the deployment this pin governs.
	DeploymentID uuid.UUID `json:"deployment_id"`

	// Region is the expected storage region code (e.g. "eu", "us").
	Region string `json:"region"`

	// HostPatterns lists host substrings that are considered to
	// resolve to Region (e.g. "eu-west-1.rds.amazonaws.com",
	// ".eu.", "localhost" for a local/air-gapped deployment). A
	// configured DSN host matches the pin if it contains any pattern
	// in this list as a substring (case-insensitive).
	HostPatterns []string `json:"host_patterns"`
}

// Validate checks p for internal consistency (not against live
// configuration -- see ValidateDSN for that).
func (p *RegionPin) Validate() error {
	if p == nil {
		return ErrNilPolicy
	}
	if p.DeploymentID == uuid.Nil {
		return ErrEmptyDeploymentID
	}
	if strings.TrimSpace(p.Region) == "" {
		return ErrEmptyRegion
	}
	if len(p.HostPatterns) == 0 {
		return wrapf("Validate", errNoHostPatterns)
	}
	return nil
}

// CheckKind classifies a single check performed by Verify, so a Report
// can enumerate exactly what was inspected and what failed.
type CheckKind string

const (
	// CheckStorageRegion verifies the live database DSN's host matches
	// the deployment's RegionPin.
	CheckStorageRegion CheckKind = "storage_region"

	// CheckProviderRegions verifies every provider region currently in
	// use is allowed by the policy.
	CheckProviderRegions CheckKind = "provider_regions"

	// CheckAirGapComposition verifies an air-gapped policy is composed
	// with router's AirGappedOnly flag.
	CheckAirGapComposition CheckKind = "air_gap_composition"
)

// CheckResult is the outcome of a single Verify check.
type CheckResult struct {
	Kind   CheckKind `json:"kind"`
	Passed bool      `json:"passed"`
	Detail string    `json:"detail,omitempty"`
	Region string    `json:"region,omitempty"`
}

// Report is the result of Verify: a point-in-time assessment of
// whether a deployment's live configuration actually satisfies its
// ResidencyPolicy.
type Report struct {
	DeploymentID uuid.UUID     `json:"deployment_id"`
	GeneratedAt  time.Time     `json:"generated_at"`
	Checks       []CheckResult `json:"checks"`
}

// Passed reports whether every check in r succeeded. A Report with no
// checks at all is considered not passed -- Verify never returns an
// empty, vacuously-true report.
func (r *Report) Passed() bool {
	if r == nil || len(r.Checks) == 0 {
		return false
	}
	for _, c := range r.Checks {
		if !c.Passed {
			return false
		}
	}
	return true
}

// Failures returns the subset of r.Checks that did not pass.
func (r *Report) Failures() []CheckResult {
	if r == nil {
		return nil
	}
	var out []CheckResult
	for _, c := range r.Checks {
		if !c.Passed {
			out = append(out, c)
		}
	}
	return out
}
