package iac

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Tier names the deployment tier a DeploymentProfile targets. A closed
// enum (unlike packages/compliance.Framework's deliberately open
// string type) because a tier determines which concrete infra/
// manifests and packages/airgapped/packages/dataresidency composition
// rules apply -- adding a fourth tier is a real new capability, not a
// per-customer configuration value, so it warrants a new phase and a
// new constant rather than an open string a caller could invent at
// runtime.
type Tier string

const (
	// TierCloud is a managed-cloud deployment: infra/cloud/'s
	// manifests, a selected packages/dataresidency region, and no
	// air-gap constraints.
	TierCloud Tier = "cloud"

	// TierOnPrem is a customer-premises deployment: infra/onprem/'s
	// manifests, explicit local-storage/no-managed-database
	// assumptions, and typically (but not necessarily) a single pinned
	// region under packages/dataresidency.
	TierOnPrem Tier = "onprem"

	// TierAirgapped is a zero-egress deployment composing with the
	// packages/airgapped.Profile already defined in Phase 079:
	// infra/airgapped/'s manifests reference bundled images by digest,
	// never pull from an external registry, and every
	// packages/dataresidency policy is the AirGappedPreset.
	TierAirgapped Tier = "airgapped"
)

// allTiers is the exhaustive set of recognized Tier values, used by
// IsValid, mirroring packages/keymanagement.allKeyStates's convention.
var allTiers = map[Tier]struct{}{
	TierCloud:     {},
	TierOnPrem:    {},
	TierAirgapped: {},
}

// IsValid reports whether t is one of the named Tier constants.
func (t Tier) IsValid() bool {
	_, ok := allTiers[t]
	return ok
}

// String satisfies fmt.Stringer.
func (t Tier) String() string { return string(t) }

// InfraDir returns the infra/ subdirectory this tier's real manifests
// live under, relative to the repository root (e.g. "infra/cloud").
// ValidateManifest does not require callers to use this convention --
// manifestPath is caller-supplied -- but every manifest this phase
// commits follows it, and doc/deployment.md documents why.
func (t Tier) InfraDir() string {
	switch t {
	case TierCloud:
		return "infra/cloud"
	case TierOnPrem:
		return "infra/onprem"
	case TierAirgapped:
		return "infra/airgapped"
	default:
		return ""
	}
}

// DeploymentProfile is the per-tier deployment declaration this phase
// adds: which Tier a tenant's deployment targets, plus the tier-
// specific fields needed to compose with the platform's existing
// deployment-shaped concepts rather than duplicate them:
//
//   - DeploymentID/TenantID mirror packages/persistence.Deployment's
//     identity fields (ID/TenantID) -- this type does not replace that
//     row, it is a separate value keyed by the same DeploymentID,
//     exactly the composition pattern
//     packages/dataresidency.ResidencyPolicy and
//     packages/keymanagement.KeyMetadata already established.
//   - SetupProfileName names the packages/setup wizard's completed
//     "deployment profile" concept (country/court/language/provider
//     selection -- see packages/setup.SetupWizard) this deployment was
//     provisioned through. This package does not import
//     packages/setup or re-derive that selection; it references the
//     tenant's setup wizard by TenantID (the same key
//     packages/setup.Repository.GetByTenant already uses), because
//     packages/setup governs *what* a deployment reasons about
//     (jurisdiction, court, language) while DeploymentProfile governs
//     *how* it is deployed (tier, region, secrets, rollout) -- two
//     orthogonal concerns over the same tenant.
//   - Region applies to TierCloud only (task: "region if Cloud"),
//     composing with packages/dataresidency.RegionPin/ResidencyPolicy
//     by naming the same region code, not by importing that package
//     to re-validate it -- a deployment's actual residency enforcement
//     stays exactly where Phase 078 put it.
//   - AirgapConformanceRef applies to TierAirgapped only: it is the
//     DeploymentID of the packages/airgapped.Profile this deployment's
//     zero-egress conformance was already certified against (see
//     packages/airgapped.Conformance/ConformanceReport). This package
//     does not construct or validate a second Profile; it only
//     records which one this deployment composes with.
type DeploymentProfile struct {
	// DeploymentID identifies the deployment this profile governs,
	// matching packages/persistence.Deployment.ID.
	DeploymentID uuid.UUID `json:"deployment_id"`

	// TenantID identifies the owning tenant, matching
	// packages/persistence.Deployment.TenantID and the key
	// packages/setup.Repository is keyed by.
	TenantID uuid.UUID `json:"tenant_id"`

	// Tier is the deployment tier this profile targets.
	Tier Tier `json:"tier"`

	// SetupProfileName references, by name only, the packages/setup
	// wizard state this deployment was provisioned through (e.g.
	// packages/setup's ConfigProfileName-style constant, or a free-form
	// label such as "uae-supreme-court-en-ar" describing the completed
	// wizard's jurisdiction/court/language selection). Reference only:
	// this package never dereferences it into a live
	// packages/setup.SetupWizard.
	SetupProfileName string `json:"setup_profile_name,omitempty"`

	// Region is the packages/dataresidency region code this deployment
	// is pinned to. Required when Tier is TierCloud; must be empty
	// otherwise (on-prem and air-gapped deployments do not select a
	// managed-cloud region -- see packages/dataresidency.AirGappedPreset
	// for why the air-gapped case in particular has no allowed
	// region at all).
	Region string `json:"region,omitempty"`

	// AirgapConformanceRef is the DeploymentID (as a string, matching
	// packages/airgapped.ConformanceReport.DeploymentID's own string
	// encoding of a uuid.UUID) of the packages/airgapped.Profile this
	// deployment's zero-egress conformance was certified against.
	// Required when Tier is TierAirgapped; must be empty otherwise.
	AirgapConformanceRef string `json:"airgap_conformance_ref,omitempty"`

	// CreatedAt and UpdatedAt are informational bookkeeping fields,
	// mirroring packages/dataresidency.ResidencyPolicy's convention.
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// Validate checks p for internal consistency. It does not reach out to
// packages/setup, packages/airgapped, or packages/dataresidency to
// verify the referenced names/IDs actually resolve -- this is a
// structural check only, mirroring
// packages/dataresidency.ResidencyPolicy.Validate's scope.
func (p *DeploymentProfile) Validate() error {
	if p == nil {
		return wrapf("Validate", ErrEmptyDeploymentID)
	}
	if p.DeploymentID == uuid.Nil {
		return wrapf("Validate", ErrEmptyDeploymentID)
	}
	if p.TenantID == uuid.Nil {
		return wrapf("Validate", ErrEmptyTenantID)
	}
	if !p.Tier.IsValid() {
		return wrapf("Validate", ErrInvalidTier)
	}

	switch p.Tier {
	case TierCloud:
		if strings.TrimSpace(p.Region) == "" {
			return wrapf("Validate", ErrRegionRequiredForCloud)
		}
		if strings.TrimSpace(p.AirgapConformanceRef) != "" {
			return wrapf("Validate", ErrAirgapConformanceRefRequired)
		}
	case TierAirgapped:
		if strings.TrimSpace(p.AirgapConformanceRef) == "" {
			return wrapf("Validate", ErrAirgapConformanceRefRequired)
		}
		if strings.TrimSpace(p.Region) != "" {
			return wrapf("Validate", ErrRegionNotAllowedOutsideCloud)
		}
	case TierOnPrem:
		if strings.TrimSpace(p.Region) != "" {
			return wrapf("Validate", ErrRegionNotAllowedOutsideCloud)
		}
	}
	return nil
}
