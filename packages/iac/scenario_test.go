package iac

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

// TestScenario_CloudDeploymentEndToEnd exercises every piece this
// phase adds together in one realistic sequence, proving they
// actually compose rather than only working in isolation (task 9):
//
//  1. Build and validate a cloud-tier DeploymentProfile.
//  2. Validate the real committed infra/cloud/ manifests against it.
//  3. Build a SecretInjectionPlan for the same deployment/tier.
//  4. Run a Dev-stage DeploymentVerification and promote to Staging.
//  5. Run a Staging-stage DeploymentVerification and promote to Prod.
//  6. Compute a canary traffic schedule for the Prod rollout.
func TestScenario_CloudDeploymentEndToEnd(t *testing.T) {
	deploymentID := uuid.New()
	tenantID := uuid.New()

	// Step 1: DeploymentProfile.
	profile := DeploymentProfile{
		DeploymentID: deploymentID,
		TenantID:     tenantID,
		Tier:         TierCloud,
		Region:       "eu-west-1",
	}
	if err := profile.Validate(); err != nil {
		t.Fatalf("cloud profile should validate: %v", err)
	}
	if profile.Tier.InfraDir() != "infra/cloud" {
		t.Fatalf("expected infra dir infra/cloud, got %q", profile.Tier.InfraDir())
	}

	// Step 2: validate the real committed manifests for this tier.
	infraDir := filepath.Join("..", "..", profile.Tier.InfraDir())
	for _, file := range []string{"docker-compose.yml", "configmap.yaml", "deployment.yaml", "service.yaml"} {
		report, err := ValidateManifest(profile.Tier, filepath.Join(infraDir, file))
		if err != nil {
			t.Fatalf("ValidateManifest(%s): unexpected error: %v", file, err)
		}
		if !report.Passed() {
			t.Fatalf("ValidateManifest(%s) did not pass: %+v", file, report.Failures())
		}
	}

	// Step 3: SecretInjectionPlan.
	plan, err := DefaultPlanForTier(deploymentID.String(), profile.Tier)
	if err != nil {
		t.Fatalf("unexpected error building secret plan: %v", err)
	}
	if err := plan.Validate(); err != nil {
		t.Fatalf("secret plan should validate: %v", err)
	}

	// Step 4: Dev verification and promotion.
	pipeline, err := NewPromotionPipeline(deploymentID.String())
	if err != nil {
		t.Fatalf("unexpected error creating pipeline: %v", err)
	}

	devChecklist := Checklist{Checks: []Check{
		{Kind: CheckKindHealthEndpoint, Name: "gateway /readyz (dev)", Run: func() error { return nil }},
		{Kind: CheckKindMigrationVersion, Name: "schema migration matches (dev)", Run: func() error { return nil }},
	}}
	devReport, err := RunDeploymentVerification(deploymentID.String(), StageDev, devChecklist)
	if err != nil {
		t.Fatalf("unexpected error running dev verification: %v", err)
	}
	if !devReport.Passed() {
		t.Fatalf("expected dev verification to pass: %+v", devReport.Failures())
	}
	if err := pipeline.RecordVerification(devReport); err != nil {
		t.Fatalf("unexpected error recording dev verification: %v", err)
	}
	if err := pipeline.Promote(); err != nil {
		t.Fatalf("expected Dev -> Staging promotion to succeed: %v", err)
	}
	if pipeline.CurrentStage != StageStaging {
		t.Fatalf("expected CurrentStage Staging, got %v", pipeline.CurrentStage)
	}

	// Step 5: Staging verification (including a guardrail smoke test
	// this time) and promotion to Prod.
	stagingChecklist := Checklist{Checks: []Check{
		{Kind: CheckKindHealthEndpoint, Name: "gateway /readyz (staging)", Run: func() error { return nil }},
		{Kind: CheckKindMigrationVersion, Name: "schema migration matches (staging)", Run: func() error { return nil }},
		{Kind: CheckKindGuardrailSmokeTest, Name: "guardrail disclaimer present (staging)", Run: func() error { return nil }},
	}}
	stagingReport, err := RunDeploymentVerification(deploymentID.String(), StageStaging, stagingChecklist)
	if err != nil {
		t.Fatalf("unexpected error running staging verification: %v", err)
	}
	if !stagingReport.Passed() {
		t.Fatalf("expected staging verification to pass: %+v", stagingReport.Failures())
	}
	if err := pipeline.RecordVerification(stagingReport); err != nil {
		t.Fatalf("unexpected error recording staging verification: %v", err)
	}
	if err := pipeline.Promote(); err != nil {
		t.Fatalf("expected Staging -> Prod promotion to succeed: %v", err)
	}
	if pipeline.CurrentStage != StageProd {
		t.Fatalf("expected CurrentStage Prod, got %v", pipeline.CurrentStage)
	}

	// Step 6: canary rollout schedule for the newly-promoted Prod
	// release.
	canary := DefaultCanaryPlan()
	if err := canary.Validate(); err != nil {
		t.Fatalf("canary plan should validate: %v", err)
	}
	strategy := RolloutStrategyCanary
	if !strategy.IsValid() {
		t.Fatal("expected RolloutStrategyCanary to be valid")
	}
	firstStepPct, err := canary.TrafficPercentageAt(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if firstStepPct <= 0 || firstStepPct >= 100 {
		t.Fatalf("expected first canary step to be a partial rollout, got %v%%", firstStepPct)
	}
}

// TestScenario_AirgappedDeploymentComposesWithAirgapProfile proves the
// air-gapped tier's DeploymentProfile, its infra/ manifests, and its
// SecretInjectionPlan are mutually consistent -- no KMS reference
// anywhere, every manifest passes, and the profile requires an
// AirgapConformanceRef exactly where packages/airgapped.Profile's own
// conformance concept is expected to be recorded.
func TestScenario_AirgappedDeploymentComposesWithAirgapProfile(t *testing.T) {
	deploymentID := uuid.New()
	tenantID := uuid.New()
	airgapProfileID := uuid.New()

	profile := DeploymentProfile{
		DeploymentID:         deploymentID,
		TenantID:             tenantID,
		Tier:                 TierAirgapped,
		AirgapConformanceRef: airgapProfileID.String(),
	}
	if err := profile.Validate(); err != nil {
		t.Fatalf("airgapped profile should validate: %v", err)
	}

	infraDir := filepath.Join("..", "..", profile.Tier.InfraDir())
	for _, file := range []string{"docker-compose.yml", "configmap.yaml", "deployment.yaml", "service.yaml", "profile-composition.yaml"} {
		report, err := ValidateManifest(profile.Tier, filepath.Join(infraDir, file))
		if err != nil {
			t.Fatalf("ValidateManifest(%s): unexpected error: %v", file, err)
		}
		if !report.Passed() {
			t.Fatalf("ValidateManifest(%s) did not pass: %+v", file, report.Failures())
		}
	}

	plan, err := DefaultPlanForTier(deploymentID.String(), profile.Tier)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, s := range plan.Secrets {
		if s.Mechanism == InjectionMechanismKMSReference {
			t.Fatalf("air-gapped plan must never reference a KMS, got %+v", s)
		}
	}
}

// TestManifestKind_UnknownShapeStillReportsSuccessfully proves
// ValidateManifest degrades gracefully (rather than erroring) for a
// YAML document whose shape none of this package's classifiers
// recognize -- it is reported as ManifestKindUnknown and "not
// evaluated", not rejected outright.
func TestManifestKind_UnknownShapeStillReportsSuccessfully(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "unrecognized.yaml")
	if err := os.WriteFile(path, []byte("some_top_level_key: some_value\n"), 0o600); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	report, err := ValidateManifest(TierCloud, path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.DocumentKinds) != 1 || report.DocumentKinds[0] != ManifestKindUnknown {
		t.Fatalf("expected a single ManifestKindUnknown document, got %+v", report.DocumentKinds)
	}
	if !report.Passed() {
		t.Errorf("expected an unrecognized-but-parseable document to still pass (nothing to evaluate), got failures: %+v", report.Failures())
	}
}

func TestStringMethods(t *testing.T) {
	if got := RolloutStrategyCanary.String(); got != "canary" {
		t.Errorf("RolloutStrategy.String() = %q, want %q", got, "canary")
	}
	if got := InjectionMechanismEnvVar.String(); got != "env_var" {
		t.Errorf("InjectionMechanism.String() = %q, want %q", got, "env_var")
	}
	if got := TierCloud.String(); got != "cloud" {
		t.Errorf("Tier.String() = %q, want %q", got, "cloud")
	}
	if got := StageProd.String(); got != "prod" {
		t.Errorf("Stage.String() = %q, want %q", got, "prod")
	}
}

func TestValidateManifest_UnrecognizedTierDoesNotPanicOnAirgapCompositionOutsideAirgapped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile-composition.yaml")
	content := `
offline_registry_host: verdex-registry.local
config_profile_name_ref: airgapped
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	report, err := ValidateManifest(TierCloud, path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Passed() {
		t.Error("expected the airgap composition document to fail when evaluated against a non-airgapped tier")
	}
}

// sanity-check that the sentinel errors this scenario relies on are
// distinguishable via errors.Is, guarding against an accidental future
// refactor that collapses two distinct error meanings into one.
func TestSentinelErrorsAreDistinct(t *testing.T) {
	if errors.Is(ErrRegionRequiredForCloud, ErrRegionNotAllowedOutsideCloud) {
		t.Error("ErrRegionRequiredForCloud and ErrRegionNotAllowedOutsideCloud must be distinct sentinels")
	}
	if errors.Is(ErrStageNotVerified, ErrTerminalStage) {
		t.Error("ErrStageNotVerified and ErrTerminalStage must be distinct sentinels")
	}
}
