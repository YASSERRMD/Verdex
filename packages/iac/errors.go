package iac

import (
	"errors"
	"fmt"
)

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrInvalidTier is returned when a Tier value is not one of the
	// named constants.
	ErrInvalidTier = errors.New("iac: invalid deployment tier")

	// ErrEmptyDeploymentID is returned when a DeploymentProfile is
	// constructed or validated with a zero DeploymentID.
	ErrEmptyDeploymentID = errors.New("iac: deployment id is required")

	// ErrEmptyTenantID is returned when a DeploymentProfile is
	// constructed or validated with a zero TenantID.
	ErrEmptyTenantID = errors.New("iac: tenant id is required")

	// ErrRegionRequiredForCloud is returned when a Cloud-tier
	// DeploymentProfile carries no Region.
	ErrRegionRequiredForCloud = errors.New("iac: region is required for the cloud tier")

	// ErrRegionNotAllowedOutsideCloud is returned when a non-Cloud-tier
	// DeploymentProfile declares a Region -- on-prem and air-gapped
	// deployments do not select a cloud region.
	ErrRegionNotAllowedOutsideCloud = errors.New("iac: region only applies to the cloud tier")

	// ErrAirgapConformanceRefRequired is returned when an Airgapped-tier
	// DeploymentProfile carries no AirgapConformanceRef -- see
	// DeploymentProfile.AirgapConformanceRef's doc comment for what this
	// composes with.
	ErrAirgapConformanceRefRequired = errors.New("iac: air-gap conformance reference is required for the air-gapped tier")

	// ErrEmptyManifestPath is returned when ValidateManifest is called
	// with a blank manifestPath.
	ErrEmptyManifestPath = errors.New("iac: manifest path is required")

	// ErrManifestNotFound is returned when ValidateManifest cannot read
	// the file at manifestPath.
	ErrManifestNotFound = errors.New("iac: manifest file not found")

	// ErrManifestNotYAML is returned when a manifest file does not parse
	// as YAML at all.
	ErrManifestNotYAML = errors.New("iac: manifest does not parse as YAML")

	// ErrNilPipeline is returned by PromotionPipeline methods called on
	// a nil receiver.
	ErrNilPipeline = errors.New("iac: promotion pipeline must not be nil")

	// ErrInvalidStage is returned when a Stage value is not one of the
	// named constants.
	ErrInvalidStage = errors.New("iac: invalid promotion stage")

	// ErrStageNotVerified is returned when PromotionPipeline.Promote is
	// called but the current stage has no passing DeploymentVerification
	// on file.
	ErrStageNotVerified = errors.New("iac: current stage has no passing deployment verification")

	// ErrNoValidTransition is returned when PromotionPipeline.Promote
	// would move a stage to a target that is not its immediate
	// successor (Dev -> Staging -> Prod, no skipping).
	ErrNoValidTransition = errors.New("iac: no valid promotion transition from the current stage")

	// ErrTerminalStage is returned when PromotionPipeline.Promote is
	// called while already at the terminal (Prod) stage.
	ErrTerminalStage = errors.New("iac: pipeline is already at its terminal stage")

	// ErrInvalidSecretMechanism is returned when a SecretRef names a
	// InjectionMechanism that is not one of the named constants.
	ErrInvalidSecretMechanism = errors.New("iac: invalid secret injection mechanism")

	// ErrEmptySecretName is returned when a SecretRef carries a blank
	// Name.
	ErrEmptySecretName = errors.New("iac: secret name is required")

	// ErrInvalidRolloutStrategy is returned when a RolloutStrategy value
	// is not one of the named constants.
	ErrInvalidRolloutStrategy = errors.New("iac: invalid rollout strategy")

	// ErrEmptyCanaryStages is returned when a CanaryPlan carries no
	// Stages.
	ErrEmptyCanaryStages = errors.New("iac: canary plan must declare at least one stage")

	// ErrCanaryStageOutOfRange is returned when a CanaryPlan step index
	// requested from TrafficPercentageAt falls outside the declared
	// Stages.
	ErrCanaryStageOutOfRange = errors.New("iac: canary step index out of range")

	// ErrInvalidTrafficPercentage is returned when a canary stage's
	// TrafficPercent is not between 0 and 100 inclusive, or stages are
	// not non-decreasing.
	ErrInvalidTrafficPercentage = errors.New("iac: invalid canary traffic percentage")

	// ErrEmptyChecklist is returned when a DeploymentVerification is run
	// with no Checks configured.
	ErrEmptyChecklist = errors.New("iac: deployment verification checklist must not be empty")

	// ErrNilCheckFunc is returned when a Check has a nil Run function.
	ErrNilCheckFunc = errors.New("iac: verification check must have a non-nil run function")
)

// wrapf mirrors the fmt.Errorf("pkg: fn: %w", err) convention used
// throughout this repository's packages.
func wrapf(fn string, err error) error {
	return fmt.Errorf("iac: %s: %w", fn, err)
}
