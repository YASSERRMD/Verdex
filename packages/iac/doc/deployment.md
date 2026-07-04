# Deployment & infrastructure as code (Phase 094)

This phase draws together several earlier threads -- the first-run
setup wizard's deployment-profile concept added in Phase 008
(`packages/setup`), the fully offline deployment tier assembled in
Phase 079 (`packages/airgapped`), the region-pinning/residency policy
added around Phase 078 (`packages/dataresidency`), the key-management
and encryption-at-rest machinery added in Phases 076/075
(`packages/keymanagement`, `packages/encryption`), the health-endpoint
conventions from Phase 003 (`packages/observability`), the migration
runner in `packages/persistence`, and the non-binding guardrail from
Phase 057 (`packages/guardrail`) -- into one tier-aware deployment
model: `packages/iac`, plus real infrastructure-as-code artifacts
under `infra/` at the repository root.

## Goal

Reproducible deployments across three tiers -- cloud, on-prem, and
air-gapped -- with a real environment-promotion gate, a documented
secret-injection plan, blue-green/canary rollout support, and a
post-deploy verification checklist, so "deploy this platform" means
the same repeatable thing regardless of which tier a customer runs.

## What this package composes from, versus what is new

| Existing piece | What it already provides | What this phase adds |
|---|---|---|
| `packages/setup` (Phase 008) | `SetupWizard`'s deployment-profile concept: jurisdiction/court/language/provider selection, gated state machine | `DeploymentProfile.SetupProfileName` references the completed wizard by name only -- "what a deployment reasons about" stays in `packages/setup`; this phase governs "how it is deployed" instead |
| `packages/airgapped` (Phase 079) | `Profile`: composed residency/routing/key-provider declaration, `NetworkPolicy`, `Conformance`/`ConformanceReport` | `DeploymentProfile.AirgapConformanceRef` and `infra/airgapped/profile-composition.yaml` both reference the same `Profile.DeploymentID` by convention, never re-implementing its checks |
| `packages/dataresidency` (~Phase 078) | `ResidencyPolicy`, `RegionPin`, `AirGappedPreset` | `DeploymentProfile.Region` names the same region code a cloud deployment's residency policy already governs, without importing the package to re-validate it |
| `packages/keymanagement` (Phase 076) / `packages/encryption` (Phase 075) | `Provider`/`KeySource`: the only secret-material storage/rotation machinery | `SecretRef.ComposesWith` names them by string tag only (`packages/compliance.Control.MappedTo`'s exact convention) -- this package never touches a real key or KMS client |
| `packages/observability` (Phase 003) | `LivenessHandler`/`ReadinessHandler` (`/healthz`/`/readyz`) | `CheckKindHealthEndpoint` names what a real `Check.Run` dials, by reference |
| `packages/persistence` | Migration runner, applied-version tracking | `CheckKindMigrationVersion` names what a real `Check.Run` queries, by reference |
| `packages/guardrail` (Phase 057) | `RequireDisclaimer`/`CheckText`: the only non-binding-disclaimer enforcement | `CheckKindGuardrailSmokeTest` names what a real `Check.Run` exercises post-deploy, by reference |
| `packages/threatmodel` (Phase 083) | `doc/Dockerfile.hardened`: the reference hardened container image template | Every `infra/*/deployment.yaml`'s `securityContext` follows its runtime-settings notes verbatim, rather than a second hardening checklist |
| `packages/config` | `VERDEX_`-prefixed env var convention (`EnvPrefix`), `env://VAR_NAME`/`vault://path#key` secret-reference schemes | Every `infra/` manifest and `SecretInjectionPlan` uses these exact conventions, not a new one |

## Tier and DeploymentProfile (types.go)

`Tier` is a closed enum -- `TierCloud`, `TierOnPrem`, `TierAirgapped`
-- unlike `packages/compliance.Framework`'s deliberately open string
type: a tier determines which real `infra/` manifests and composition
rules apply, so adding a fourth tier is a genuinely new capability
warranting a new phase, not a per-customer configuration value a
caller should be able to invent at runtime.

`DeploymentProfile` is the per-tier deployment declaration: a
`DeploymentID`/`TenantID` pair matching
`packages/persistence.Deployment`'s identity fields, a `Tier`, and
three tier-conditional fields `Validate` enforces:

- `SetupProfileName` (optional, any tier): names the
  `packages/setup.SetupWizard` this deployment was provisioned
  through, by convention only.
- `Region` (required for `TierCloud`, forbidden otherwise): the
  `packages/dataresidency` region code this deployment is pinned to.
- `AirgapConformanceRef` (required for `TierAirgapped`, forbidden
  otherwise): the `packages/airgapped.Profile.DeploymentID` this
  deployment's zero-egress conformance was certified against.

## What is under `infra/`, and why

Real IaC artifacts live at the repository root under `infra/cloud/`,
`infra/onprem/`, and `infra/airgapped/` -- not under `packages/iac/`,
matching the brief's design guidance that `infra/` is where "real IaC
artifacts belong." Each tier's directory holds a `docker-compose.yml`
(for local smoke-testing that tier's shape) plus a minimal Kubernetes
manifest set -- a `ConfigMap`, a `Deployment`, and a `Service` (and, for
on-prem, a `PersistentVolumeClaim`) -- for the `gateway` and `router`
services this monorepo would run in production. All three tiers
deliberately share the same service names and the same `VERDEX_`-
prefixed environment-variable convention (`packages/config/env.go`'s
`EnvPrefix`) and `env://VAR_NAME` secret-reference scheme
(`packages/config/secrets.go`), so the tiers read as one consistent
deployment model with three different topologies, not three unrelated
designs. This is a handful of well-formed manifests per tier by
design, not an exhaustive production-grade Helm chart.

### `infra/cloud/`

Assumes a managed database is available downstream; the committed
`postgres` service in `docker-compose.yml` is a local stand-in for
smoke-testing the tier's shape only, explicitly not the production
topology (see that file's own header comment). Declares
`VERDEX_DATARESIDENCY_REGION` in `configmap.yaml`. `service.yaml` is a
`ClusterIP` `Service` -- an external cloud load balancer/ingress
controller is expected to sit in front of it, out of scope for this
manifest set. `deployment.yaml` runs 3 replicas.

### `infra/onprem/`

`postgres` is first-class in both `docker-compose.yml` (a local named
volume) and the Kubernetes form (`postgres-pvc.yaml`, a
`local-path`-class `PersistentVolumeClaim`) -- there is no managed-
database fallback for this tier, and both files say so explicitly.
`configmap.yaml` carries no `VERDEX_DATARESIDENCY_REGION` key: an
on-prem deployment's data resides on the customer's own hardware, not
a selected cloud region (`ValidateManifest` enforces this absence for
this tier, mirroring `DeploymentProfile.Validate`'s identical rule).
`service.yaml` is a `NodePort` `Service` -- no external cloud load
balancer to defer to. `deployment.yaml` runs 2 replicas.

### `infra/airgapped/`

Composes with `packages/airgapped.Profile` (Phase 079) rather than
re-deriving its zero-egress guarantee. Every image reference in
`docker-compose.yml`/`deployment.yaml` is
`verdex-registry.local/<name>@sha256:<64-hex-digest>` -- pinned by
digest against an offline registry mirror an operator loads from a
signed bundle transferred by physical media, never a public tag or a
live registry pull. `deployment.yaml` sets `imagePullPolicy: Never`
explicitly, since this cluster has no route to any registry outside
the offline mirror. Every published port in `docker-compose.yml` binds
`127.0.0.1` only. `profile-composition.yaml` is a non-Kubernetes,
non-compose file -- a structurally validated cross-reference (parsed
and checked by `ValidateManifest` exactly like the other manifests)
recording which `packages/airgapped.Profile` fields (config profile
name, residency preset, `Routing.AirGappedOnly`, key provider,
`DeploymentID`) this deployment composes with. `deployment.yaml` runs
1 replica.

### Why no new Dockerfile

This repository has no `cmd/` binary for `gateway`/`router` yet, and
`.github/workflows/ci.yml`'s `container-scan` job (added in Phase 084)
already documents the same fact as a placeholder. Container hardening
in every `infra/*/deployment.yaml`'s `securityContext` instead follows
`packages/threatmodel/doc/Dockerfile.hardened`'s runtime-settings notes
(non-root, read-only root filesystem, dropped capabilities) verbatim
-- adding a second Dockerfile for a service that is not yet a real
buildable binary would be speculative, not reproducible, and Phase 083
already supplies the reference template for whichever future phase
adds the first real one.

## `ValidateManifest` (validate.go, validate_rules.go)

`ValidateManifest(tier, manifestPath)` does real structural validation
of a committed `infra/<tier>/` file: it parses the file as YAML
(handling a multi-document compose/Kubernetes stream), classifies each
document's `ManifestKind`, and checks the tier-specific structural
rules this phase's own manifests are built to satisfy:

- Every container's `securityContext` disables privilege escalation,
  forces a read-only root filesystem, and drops all capabilities
  (mirroring `packages/threatmodel/doc/Dockerfile.hardened`'s runtime
  notes).
- Every environment value whose key looks secret-shaped (ends in
  `_PASSWORD`, `_DSN`, `_KEY`, `_SECRET`, or `_TOKEN`) must be an
  `env://`/`vault://` reference, never a literal value.
- A cloud-tier `ConfigMap` must declare
  `VERDEX_DATARESIDENCY_REGION`; an on-prem or air-gapped one must
  not.
- Every air-gapped-tier image reference (compose or Kubernetes) must
  match `<registry-host>/<name>@sha256:<64-hex-digest>`, and every
  air-gapped Kubernetes container's `imagePullPolicy` must be `Never`.
- A `PersistentVolumeClaim` is only valid for on-prem/air-gapped tiers
  (cloud defers to a managed database) and must name a non-cloud
  storage class.
- `infra/airgapped/profile-composition.yaml` must carry every field
  its own header comment documents, and only validates against
  `TierAirgapped`.

This is deliberately not a Kubernetes schema validator (no apiserver
dry-run, no CRD awareness) -- it checks the specific structural
invariants this phase's design calls for, nothing more. It is proven
against every manifest this phase actually committed under `infra/`
(the positive case in `validate_test.go`'s
`TestValidateManifest_RealCommittedManifests`) and twelve deliberately-
broken fixtures covering each rule above (the negative case,
`TestValidateManifest_CatchesDeliberatelyBrokenFixtures`) -- a
validator that cannot fail is worthless, the same principle Phase
086's security-testing harness established.

## Environment promotion pipeline (promotion.go)

`Stage` (`StageDev`/`StageStaging`/`StageProd`) and `PromotionPipeline`
implement a real gated state machine, mirroring
`packages/setup.SetupWizard`'s `CanTransitionTo`/`transition`
convention. A pipeline starts at `StageDev`. `Promote()` advances to
the next stage in sequence only if a `DeploymentVerificationReport` has
been recorded for the *current* stage (via `RecordVerification`) and
that report's `Passed()` is true -- there is no bypass path.
`promotion_test.go`'s `TestPromote_ProdBlockedWithoutPassingStagingVerification`
proves the brief's exact scenario: Dev's verification passes and
promotes the pipeline into Staging, but Staging itself has not yet been
verified, so the subsequent `Promote()` call to reach Prod is refused
with `ErrStageNotVerified`.

## Secret injection plan (secretplan.go)

`SecretInjectionPlan` describes which secrets (by `SecretRef.Name`/
`Reference` only, never a value) get injected into a `Tier` via which
`InjectionMechanism` (`env_var`/`mounted_file`/`kms_reference`).
`SecretRef.ComposesWith` names, by string tag, the
`packages/keymanagement`/`packages/encryption` piece that actually
stores/rotates the referenced secret -- this package never touches a
real key or KMS client. `Validate` rejects
`InjectionMechanismKMSReference` outright for `TierAirgapped`: an
air-gapped deployment has no reachable KMS under
`packages/airgapped`'s zero-egress guarantee, so a plan claiming one is
a structural error, not merely an operational risk.
`DefaultPlanForTier` seeds the mechanism this phase's own
`infra/<tier>/` manifests already use for `VERDEX_DATABASE_DSN`/
`VERDEX_ENCRYPTION_KEY` per tier (a documented-future KMS reference for
cloud, `env://` for on-prem, `env://`/a mounted `FileProvider` root for
air-gapped).

## Blue-green/canary rollout (rollout.go)

`RolloutStrategy` (`Direct`/`BlueGreen`/`Canary`) plus `CanaryPlan`, an
ordered sequence of `CanaryStage`s. `TrafficPercentageAt(stepIndex)` is
the real arithmetic task 7 calls for: given a canary plan's stages,
what percentage of traffic should reach the new version at step N. It
is a discrete, stepped lookup -- not a continuous interpolation --
matching how a real traffic-splitting orchestrator (a Kubernetes/Istio
weighted route, an ALB weighted target group) actually applies a
schedule. `DefaultCanaryPlan` seeds a 5% -> 25% -> 50% -> 100% starting
schedule with per-stage bake times.

## Deployment verification (deploymentverification.go)

`Check`/`Checklist`/`RunDeploymentVerification` run an ordered set of
post-deploy checks and aggregate them into a
`DeploymentVerificationReport`, never stopping at the first failure so
a caller sees the full picture. The three named `CheckKind` constants
-- `CheckKindHealthEndpoint`, `CheckKindMigrationVersion`,
`CheckKindGuardrailSmokeTest` -- name what a real `Check.Run` composes
with (`packages/observability`'s readiness handler, `packages/persistence`'s
migration tracking, `packages/guardrail`'s disclaimer enforcement,
respectively), but every `Check.Run` function is caller-supplied: this
package supplies the checklist/aggregation shape only, never the probe
logic for a live deployment.

## Storage

No new `packages/persistence` migration. Deployment-verification run
history follows `packages/perf.Store`/`InMemoryStore`'s precedent
exactly (an in-memory-only `Store` interface with no Postgres-backed
implementation for this phase) rather than adding one: a
`DeploymentVerificationReport` is an operational, point-in-time
artifact produced once per promotion gate check, not a tenant-scoped
business record this platform's compliance/audit surface needs to
query historically the way `packages/auditlog.Event` or
`packages/compliance.ControlEvidence` do. Should a future phase need
durable, queryable verification history, `packages/perf/store.go`'s
`Store` interface (`SaveRun`/`ListRuns`) is the shape to mirror.

## What is explicitly reused, not duplicated

- `packages/setup`'s `SetupWizard`/deployment-profile concept remains
  the only "what a deployment reasons about" model;
  `DeploymentProfile` references it by name only.
- `packages/airgapped.Profile` remains the only air-gapped zero-egress
  conformance model; `DeploymentProfile.AirgapConformanceRef` and
  `infra/airgapped/profile-composition.yaml` reference it by
  `DeploymentID` only.
- `packages/dataresidency`'s `ResidencyPolicy`/`RegionPin`/
  `AirGappedPreset` remain the only residency enforcement;
  `DeploymentProfile.Region` names the same region code by convention.
- `packages/keymanagement.Provider`/`packages/encryption.KeySource`
  remain the only secret-material storage/rotation machinery;
  `SecretRef.ComposesWith` names them by string tag only.
- `packages/observability`'s health handlers, `packages/persistence`'s
  migration runner, and `packages/guardrail`'s disclaimer enforcement
  remain the only mechanisms behind `CheckKindHealthEndpoint`/
  `CheckKindMigrationVersion`/`CheckKindGuardrailSmokeTest` -- this
  package never performs the actual probe.
- `packages/threatmodel/doc/Dockerfile.hardened` remains the only
  reference hardened container template; every `infra/*/deployment.yaml`
  follows its runtime-settings notes rather than inventing a second
  hardening checklist.
- `packages/config`'s `VERDEX_` env prefix and `env://`/`vault://`
  secret-reference schemes remain the only configuration/secret-
  reference conventions; every `infra/` manifest and
  `SecretInjectionPlan` uses them as-is.
