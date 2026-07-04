// Package iac is Phase 094: reproducible deployments across this
// platform's three tiers -- cloud, on-prem, and air-gapped. It draws
// on the deployment-profile concept the first-run setup wizard
// established in Phase 008 (packages/setup), the fully offline
// deployment tier assembled in Phase 079 (packages/airgapped), the
// region-pinning/residency policy added around Phase 078
// (packages/dataresidency), the key-management and encryption-at-rest
// machinery added in Phases 076/075 (packages/keymanagement,
// packages/encryption), the health-endpoint conventions from Phase
// 003 (packages/observability), the migration runner from
// packages/persistence, and the non-binding guardrail from Phase 057
// (packages/guardrail), composing them into one tier-aware deployment
// model plus the handful of genuinely new pieces (promotion gating,
// secret-injection planning, rollout-strategy arithmetic, deployment
// verification, and real infra/ manifests) a reproducible deployment
// still needs -- rather than duplicating any of them.
//
// Real infrastructure-as-code artifacts live under infra/ at the
// repository root (infra/cloud/, infra/onprem/, infra/airgapped/), not
// under this package -- see "What is under infra/, and why" below.
// This package is the Go-side validation, orchestration, and planning
// layer over those committed files.
//
// # What is new here
//
//   - Tier / DeploymentProfile (types.go): a closed Cloud/OnPrem/
//     Airgapped enum plus a per-deployment declaration composing with
//     packages/setup's wizard concept (SetupProfileName, by name
//     only), packages/dataresidency's region concept (Region, cloud-
//     only), and packages/airgapped's Profile (AirgapConformanceRef,
//     air-gapped-only) by reference -- no import of any of those
//     packages, following packages/compliance.Control.MappedTo's
//     reference-tag convention (task: DeploymentProfile).
//   - ValidateManifest (validate.go, validate_rules.go): real
//     structural YAML validation of the committed infra/<tier>/
//     manifests -- container hardening (non-root, read-only rootfs,
//     dropped capabilities) mirroring
//     packages/threatmodel/doc/Dockerfile.hardened's runtime notes,
//     secret values referenced never literal, cloud-only region keys,
//     and air-gapped images digest-pinned against an offline registry
//     with imagePullPolicy Never. Proven against every manifest this
//     phase actually committed (the positive case) and twelve
//     deliberately-broken fixtures (the negative case) -- see
//     validate_test.go (task 9 / "the validator").
//   - Stage / PromotionPipeline (promotion.go): a real gated Dev ->
//     Staging -> Prod state machine mirroring
//     packages/setup.SetupWizard's transition convention. Promote
//     refuses to advance without a passing
//     DeploymentVerificationReport recorded for the current stage --
//     there is no bypass (task 5).
//   - InjectionMechanism / SecretRef / SecretInjectionPlan
//     (secretplan.go): which secrets (by name/reference only, never
//     value) get injected into which tier via which mechanism
//     (env var / mounted file / KMS reference), composing with
//     packages/keymanagement and packages/encryption by reference tag
//     (SecretRef.ComposesWith). InjectionMechanismKMSReference is
//     rejected outright for TierAirgapped: there is no reachable KMS
//     under packages/airgapped's zero-egress guarantee (task 6).
//   - RolloutStrategy / CanaryPlan (rollout.go): BlueGreen/Canary/
//     Direct plus a real traffic-percentage-at-step calculator
//     (TrafficPercentageAt) -- the stepped arithmetic a real traffic-
//     splitting orchestrator applies (task 7).
//   - CheckKind / Check / Checklist / DeploymentVerificationReport /
//     RunDeploymentVerification (deploymentverification.go): a
//     checklist of post-deploy checks (health endpoint reachable,
//     migration version matches, guardrail smoke test passes) with
//     real pass/fail aggregation. Every Check.Run is caller-supplied --
//     this package never dials a live endpoint, queries
//     packages/persistence's migration table, or calls
//     packages/guardrail itself; the named CheckKind constants record
//     what a real check composes with, by reference (task 8).
//
// # What is under infra/, and why
//
// infra/cloud/, infra/onprem/, and infra/airgapped/ (at the repository
// root, not under packages/iac/) hold the actual manifests this phase
// commits: a docker-compose.yml for local smoke-testing each tier's
// shape, plus a minimal Kubernetes ConfigMap/Deployment/Service set
// (and, where relevant, a PersistentVolumeClaim) for the gateway/
// router services this monorepo would run. All three tiers share the
// same service names (gateway, router, postgres) and the same
// VERDEX_-prefixed environment-variable convention
// (packages/config/env.go's EnvPrefix) and env://VAR_NAME secret-
// reference scheme (packages/config/secrets.go) -- internally
// consistent rather than exhaustive, a handful of well-formed
// manifests per tier, not a production-grade Helm chart:
//
//   - infra/cloud/: assumes a managed database is available (the
//     committed postgres service is a local stand-in for smoke-
//     testing only, not the production shape); declares a
//     dataresidency region; a ClusterIP Service (an external cloud
//     load balancer is expected downstream); 3 replicas.
//   - infra/onprem/: postgres is first-class, backed by a local
//     volume/PVC -- there is no managed-database fallback for this
//     tier; no dataresidency region key (data resides on customer
//     hardware); a NodePort Service (no external cloud load balancer
//     to defer to); 2 replicas.
//   - infra/airgapped/: composes with packages/airgapped.Profile
//     (Phase 079) rather than re-deriving its zero-egress guarantee --
//     every image is pinned by digest against an offline registry
//     host, never a public tag or registry pull; imagePullPolicy is
//     Never; every published port binds loopback only; 1 replica.
//     profile-composition.yaml is a structurally validated cross-
//     reference (parsed and checked by ValidateManifest exactly like
//     the Kubernetes manifests) recording which packages/airgapped.Profile
//     fields this deployment composes with.
//
// This phase deliberately does not add a real Dockerfile: this
// repository has no cmd/ binary for gateway/router yet (see
// .github/workflows/ci.yml's container-scan placeholder job, added in
// Phase 084, which notes the same thing), and
// packages/threatmodel/doc/Dockerfile.hardened (Phase 083) already
// supplies the reference hardened template every infra/*/deployment.yaml's
// securityContext follows -- adding a second Dockerfile for a service
// that is not yet a real buildable binary would be speculative, not
// reproducible.
//
// # What is explicitly reused, not duplicated
//
//   - packages/setup's SetupWizard/deployment-profile concept
//     (country/court/language/provider selection) remains the only
//     "what a deployment reasons about" model; DeploymentProfile
//     references it by SetupProfileName only, governing "how it is
//     deployed" instead -- two orthogonal concerns over the same
//     tenant, not a duplicate profile type.
//   - packages/airgapped.Profile (Phase 079) remains the only
//     air-gapped zero-egress conformance model (NetworkPolicy,
//     GuardRegister, Conformance); DeploymentProfile.AirgapConformanceRef
//     and infra/airgapped/profile-composition.yaml both reference it
//     by DeploymentID, never re-implementing its checks.
//   - packages/dataresidency's ResidencyPolicy/RegionPin/AirGappedPreset
//     remain the only residency enforcement; DeploymentProfile.Region
//     names the same region code by convention, without importing that
//     package to re-validate it.
//   - packages/keymanagement.Provider and packages/encryption.KeySource
//     remain the only secret-material storage/rotation machinery;
//     SecretRef.ComposesWith names them by string tag only
//     (packages/compliance.Control.MappedTo's exact convention) -- this
//     package never touches a real key or KMS client.
//   - packages/observability's LivenessHandler/ReadinessHandler,
//     packages/persistence's migration runner, and
//     packages/guardrail's RequireDisclaimer/CheckText remain the only
//     health-check, migration-tracking, and disclaimer-enforcement
//     mechanisms; CheckKindHealthEndpoint/CheckKindMigrationVersion/
//     CheckKindGuardrailSmokeTest name what a real Check.Run composes
//     with, by reference -- this package supplies the checklist/
//     aggregation shape only, never the probe logic itself.
//   - packages/threatmodel/doc/Dockerfile.hardened (Phase 083) remains
//     the only reference hardened container image template; every
//     infra/*/deployment.yaml's securityContext follows its runtime-
//     settings notes rather than inventing a second hardening
//     checklist.
//
// See doc/deployment.md for the full write-up, including the
// composition table and the per-tier manifest rationale in prose.
package iac
