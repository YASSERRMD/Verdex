# Air-gapped deployment guide

This guide covers deploying Verdex fully offline for sensitive courts
(`Tier = TierAirgapped` in [`packages/iac`](../../packages/iac), Phase
094, composing with [`packages/airgapped`](../../packages/airgapped)'s
Phase 079 `Profile`). It complements, and does not duplicate, either
package's own design doc:
[`packages/iac/doc/deployment.md`](../../packages/iac/doc/deployment.md)
and [`packages/airgapped/doc/airgapped-tier.md`](../../packages/airgapped/doc/airgapped-tier.md).

## What "air-gapped" means here

Zero network egress beyond an explicit local allow-list, enforced in
code, not merely by network topology:

- Every LLM provider registered under an air-gapped
  `packages/airgapped.Profile` must be `local:`-prefixed
  (`GuardRegister` rejects anything else) — see
  [`packages/adapters`](../../packages/adapters)'s bundled local model
  runtime.
- Outbound dials are checked against `Profile.AllowedNetworkTargets`
  by `packages/airgapped.NetworkPolicy`/`GuardedDialContext`; anything
  that is neither loopback nor explicitly allow-listed is blocked.
- The key source is mandated to be a
  `packages/keymanagement.FileProvider` — never a reachable KMS
  (`InjectionMechanismKMSReference` is rejected outright for this tier
  by `packages/iac.SecretInjectionPlan`, since there is no reachable
  KMS under this guarantee).
- Statute/precedent corpus data loads from a local bundle directory
  (`packages/airgapped.ProvisionCorpus`), never a network fetch.
- Platform updates and license activation are signed, checksum-verified
  local bundles with no network callout
  (`packages/airgapped.ApplyUpdateBundle`, `Activate`).

## Prerequisites

- A single-node (or small, no-external-control-plane) local Kubernetes
  distribution, or Docker Compose for a lighter footprint.
- A local offline container registry mirror (`verdex-registry.local`)
  loaded from a signed bundle transferred by physical media.
- A completed `packages/airgapped.Profile`, `Conformance`-checked
  (`ConformanceReport`) before this deployment is relied upon.

## Manifests

Real, committed manifests live under
[`infra/airgapped/`](../../infra/airgapped):

| File | Purpose |
|---|---|
| `configmap.yaml` | Non-secret configuration. No `VERDEX_DATARESIDENCY_REGION` (same reasoning as the on-prem tier), plus an explicit `VERDEX_KEY_PROVIDER=file` and `VERDEX_PROVIDER_ALLOWLIST_PREFIX=local:`. |
| `deployment.yaml` | The `gateway` service `Deployment`: 1 replica. Every image reference is `verdex-registry.local/<name>@sha256:<64-hex-digest>` — pinned by digest, never a public tag. `imagePullPolicy: Never` is set explicitly, since this cluster has no route to any registry outside the offline mirror. |
| `service.yaml` | Kubernetes service definition for the single-node deployment. |
| `docker-compose.yml` | Every published port binds `127.0.0.1` only. |
| `profile-composition.yaml` | **Not** a Kubernetes/compose manifest — a structurally validated cross-reference recording which `packages/airgapped.Profile` fields (`config_profile_name_ref`, `residency_preset_ref`, `routing_air_gapped_only_ref`, `key_provider_ref`, `profile_deployment_id_ref`, `offline_registry_host`) this deployment composes with. `packages/iac.ValidateManifest`'s air-gapped rules parse and check it exactly like the other manifests in this directory. |

Container hardening follows
[`packages/threatmodel/doc/Dockerfile.hardened`](../../packages/threatmodel/doc/Dockerfile.hardened)'s
runtime-settings notes, same as the other two tiers.

## Deployment profile

Register this deployment as a `packages/iac.DeploymentProfile` with
`Tier: TierAirgapped` and a required `AirgapConformanceRef` — the
`packages/airgapped.Profile.DeploymentID` this deployment's zero-egress
conformance was certified against (`Region` is forbidden for this
tier, same as on-prem). `profile-composition.yaml`'s
`profile_deployment_id_ref` must match the same `DeploymentID` by
convention.

Before relying on this deployment, populate
`profile-composition.yaml`'s `*_ref` fields with the real values from
the constructed `packages/airgapped.Profile` — the committed file ships
with a placeholder zero UUID for `profile_deployment_id_ref` that an
operator must replace.

## Secrets

The only permitted `InjectionMechanism` for this tier is a mounted
file or environment variable sourced from the mandated
`keymanagement.FileProvider` — a KMS reference is rejected outright
by `packages/iac.SecretInjectionPlan` (there is no reachable KMS under
this tier's zero-egress guarantee).

## Provisioning the corpus and applying updates

- Load statute/precedent data from a local bundle directory via
  `packages/airgapped.ProvisionCorpus` (`FileBundleStatuteLoader`,
  `FileBundlePrecedentLoader`) — never a network fetch.
- Apply platform updates only via
  `packages/airgapped.ApplyUpdateBundle` against a signed,
  checksum-verified local bundle (`UpdateManifest`/`SignManifest`).
- Activate a license offline via `packages/airgapped.Activate` against
  a locally signed `LicenseKey` — no license-server callout.

## Validating manifests before applying

Validate an edited air-gapped manifest structurally with
`packages/iac.ValidateManifest(iac.TierAirgapped, path)` before
applying it. This checks container hardening, that every image
reference is digest-pinned against the declared offline registry host,
and that `profile-composition.yaml`'s cross-reference fields are
present.

## Before go-live: verify zero egress

Run `packages/airgapped.VerifyZeroEgress` against the deployment's
configured provider/endpoint list before go-live, and again on every
change to `AllowedNetworkTargets`. This is the same check
`Conformance`/`ConformanceReport` rolls up alongside profile validity
and the provider allow-list — treat a failing `ConformanceReport` as a
go/no-go gate, not an informational warning.

## Post-deploy verification

Run the deployment-verification checklist from
[`docs/operations/runbooks.md`](../operations/runbooks.md), same as
every other tier: liveness/readiness endpoints, migration version, and
a guardrail smoke test — plus this tier's own zero-egress conformance
check above.
