package dataresidency

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/router"
)

// PolicySource resolves the ResidencyPolicy and RegionPin currently in
// effect for a deployment. A caller typically backs this with a small
// repository composed over packages/persistence (mirroring
// packages/keymanagement's repository pattern), but Verifier accepts
// it as an interface so tests can substitute an in-memory fake without
// requiring a database.
type PolicySource interface {
	// Policy returns the ResidencyPolicy for deploymentID, or an error
	// if none is configured.
	Policy(ctx context.Context, deploymentID uuid.UUID) (*ResidencyPolicy, error)
	// RegionPin returns the RegionPin for deploymentID, or an error if
	// none is configured.
	RegionPin(ctx context.Context, deploymentID uuid.UUID) (*RegionPin, error)
}

// LiveConfig reports the actually-running configuration Verify checks
// the policy against: the database DSN currently in use, and the set
// of provider regions the deployment is actively routing to. Both are
// supplied as functions (rather than static values) so Verify reflects
// the true live state at call time -- suitable for both a one-shot
// startup check and a periodic re-check against a config that may
// rotate (e.g. a DSN updated by an operator, or a provider region
// added to the routing policy).
type LiveConfig struct {
	// DatabaseDSN returns the currently configured database connection
	// string (as accepted by packages/persistence.Open).
	DatabaseDSN func(ctx context.Context) (string, error)

	// ProviderRegionsInUse returns the region codes of every provider
	// currently reachable via the deployment's router configuration
	// (e.g. every provider.Capability.Region in the router's registry
	// that AirGappedOnly/routing rules would actually allow selecting).
	ProviderRegionsInUse func(ctx context.Context) ([]string, error)

	// RoutingPolicy is the router.RoutingPolicy currently in effect,
	// used to check air-gap composition (CheckAirGapComposition).
	RoutingPolicy router.RoutingPolicy
}

// Verifier runs Verify against a PolicySource and LiveConfig.
type Verifier struct {
	policies PolicySource
	live     LiveConfig
	clock    func() time.Time
}

// NewVerifier builds a Verifier. Returns ErrNilVerifier if policies is
// nil, DatabaseDSN is nil, or ProviderRegionsInUse is nil -- every
// dependency Verify needs to produce a real (not vacuous) report must
// be supplied.
func NewVerifier(policies PolicySource, live LiveConfig) (*Verifier, error) {
	if policies == nil {
		return nil, wrapf("NewVerifier", ErrNilVerifier)
	}
	if live.DatabaseDSN == nil || live.ProviderRegionsInUse == nil {
		return nil, wrapf("NewVerifier", ErrNilVerifier)
	}
	return &Verifier{policies: policies, live: live, clock: time.Now}, nil
}

func (v *Verifier) now() time.Time {
	if v.clock != nil {
		return v.clock().UTC()
	}
	return time.Now().UTC()
}

// Verify runs the startup/periodic residency check (task 5) for
// deploymentID: it loads the deployment's ResidencyPolicy and
// RegionPin, then asserts the live database DSN's host matches the
// pin's region, every provider region currently in use is allowed by
// the policy, and (if the policy is an air-gapped preset) that it is
// consistently composed with router's AirGappedOnly flag. It returns a
// Report enumerating every check performed and its outcome -- Verify
// itself never returns a non-nil error just because a check failed;
// failures are represented in the Report so callers can inspect
// exactly what is wrong. Verify only returns a non-nil error for a
// structural problem (e.g. no policy configured for deploymentID, or a
// dependency failure reaching live config).
func (v *Verifier) Verify(ctx context.Context, deploymentID uuid.UUID) (Report, error) {
	if deploymentID == uuid.Nil {
		return Report{}, wrapf("Verify", ErrEmptyDeploymentID)
	}

	policy, err := v.policies.Policy(ctx, deploymentID)
	if err != nil {
		return Report{}, wrapf("Verify", err)
	}
	if err := policy.Validate(); err != nil {
		return Report{}, wrapf("Verify", err)
	}

	report := Report{
		DeploymentID: deploymentID,
		GeneratedAt:  v.now(),
	}

	report.Checks = append(report.Checks, v.checkStorageRegion(ctx, deploymentID))
	report.Checks = append(report.Checks, v.checkProviderRegions(ctx, policy))
	report.Checks = append(report.Checks, v.checkAirGapComposition(policy))

	return report, nil
}

func (v *Verifier) checkStorageRegion(ctx context.Context, deploymentID uuid.UUID) CheckResult {
	pin, err := v.policies.RegionPin(ctx, deploymentID)
	if err != nil {
		return CheckResult{Kind: CheckStorageRegion, Passed: false, Detail: err.Error()}
	}
	dsn, err := v.live.DatabaseDSN(ctx)
	if err != nil {
		return CheckResult{Kind: CheckStorageRegion, Passed: false, Detail: err.Error()}
	}
	if err := pin.ValidateDSN(dsn); err != nil {
		return CheckResult{Kind: CheckStorageRegion, Passed: false, Region: pin.Region, Detail: err.Error()}
	}
	return CheckResult{Kind: CheckStorageRegion, Passed: true, Region: pin.Region}
}

func (v *Verifier) checkProviderRegions(ctx context.Context, policy *ResidencyPolicy) CheckResult {
	regions, err := v.live.ProviderRegionsInUse(ctx)
	if err != nil {
		return CheckResult{Kind: CheckProviderRegions, Passed: false, Detail: err.Error()}
	}
	for _, r := range regions {
		if !policy.AllowsRegion(r) {
			return CheckResult{
				Kind:   CheckProviderRegions,
				Passed: false,
				Region: r,
				Detail: "provider region " + r + " is not in the deployment's AllowedRegions",
			}
		}
	}
	return CheckResult{Kind: CheckProviderRegions, Passed: true}
}

func (v *Verifier) checkAirGapComposition(policy *ResidencyPolicy) CheckResult {
	if err := ComposeWithRouterAirGap(policy, v.live.RoutingPolicy); err != nil {
		return CheckResult{Kind: CheckAirGapComposition, Passed: false, Detail: err.Error()}
	}
	return CheckResult{Kind: CheckAirGapComposition, Passed: true}
}
