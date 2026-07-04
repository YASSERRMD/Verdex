# Cloud deployment guide

This guide covers deploying Verdex on a managed cloud (`Tier = TierCloud`
in [`packages/iac`](../../packages/iac), Phase 094). It complements, and
does not duplicate, [`packages/iac`'s own design doc](../../packages/iac/doc/deployment.md);
see that document for the full `DeploymentProfile`/`ValidateManifest`
model this guide operationalizes.

## Prerequisites

- A managed PostgreSQL instance reachable from the cluster (the
  `postgres` service in `infra/cloud/docker-compose.yml` is a **local
  smoke-test stand-in only** — see that file's own header comment — the
  production topology assumes a managed database downstream).
- A Kubernetes cluster with an external load balancer or ingress
  controller in front of it (`infra/cloud/service.yaml` is a
  `ClusterIP` `Service`; it does not provision one itself).
- A `packages/dataresidency` region code selected for this deployment
  (required for this tier — see below).
- A KMS reachable for secret injection (see "Secrets" below).

## Manifests

Real, committed manifests live under [`infra/cloud/`](../../infra/cloud):

| File | Purpose |
|---|---|
| `configmap.yaml` | Non-secret environment configuration, including `VERDEX_DATARESIDENCY_REGION` (cloud-tier only). |
| `deployment.yaml` | The `gateway` service `Deployment`: 3 replicas, non-root (`runAsUser: 65532`), read-only root filesystem, all capabilities dropped, `/healthz`/`/readyz` liveness and readiness probes. |
| `service.yaml` | A `ClusterIP` `Service` — an external load balancer/ingress is expected in front of it. |
| `docker-compose.yml` | Local smoke-testing of this tier's shape only; not the production topology. |

Container hardening in `deployment.yaml`'s `securityContext` follows
[`packages/threatmodel/doc/Dockerfile.hardened`](../../packages/threatmodel/doc/Dockerfile.hardened)'s
runtime-settings notes verbatim (non-root, read-only root filesystem,
dropped capabilities) — there is no second, competing hardening
checklist to reconcile against.

## Deployment profile

Register this deployment as a `packages/iac.DeploymentProfile` with
`Tier: TierCloud` and a required `Region` matching the
`packages/dataresidency` region code this deployment is pinned to
(`Region` is rejected for any other tier — see
[`packages/iac`'s design doc](../../packages/iac/doc/deployment.md)). Optionally
set `SetupProfileName` to the name of the completed
[`packages/setup`](../../packages/setup) wizard (see
[`docs/admin/setup-guide.md`](../admin/setup-guide.md)) this deployment
was provisioned through — this is a reference by name only, not an
import. See [`packages/iac`'s design doc](../../packages/iac/doc/deployment.md)
for the full field-by-field validation rules.

## Secrets

Every environment key that looks secret-shaped (`_PASSWORD`, `_DSN`,
`_KEY`, `_SECRET`, `_TOKEN`) must resolve through a
`packages/config` secret reference (`env://`, `vault://`) or a
Kubernetes `Secret` populated from one — never a literal value in a
committed manifest. `deployment.yaml`'s `VERDEX_DATABASE_DSN` and
`VERDEX_ENCRYPTION_KEY` are sourced from a `verdex-cloud-secrets`
Kubernetes `Secret` this repository never populates directly; a real
deployment resolves that `Secret`'s values via a KMS reference composing
with [`packages/keymanagement`](../../packages/keymanagement) and
[`packages/encryption`](../../packages/encryption) (Phases 076/075), per
`packages/iac.SecretInjectionPlan`.

## Validating manifests before applying

Before applying any edited cloud-tier manifest, validate it structurally
with `packages/iac.ValidateManifest(iac.TierCloud, path)` — this checks
container hardening, that secret-shaped keys are references not
literals, and that the region key is present. CI does not currently run
this automatically against `infra/`; treat it as a pre-apply operator
step until a dedicated CI job is added.

## Promotion

A cloud deployment advances through `packages/iac.PromotionPipeline`'s
gated Dev → Staging → Prod state machine (mirroring
`packages/setup.SetupWizard`'s own transition convention). `Promote`
refuses to advance a stage without a passing
`DeploymentVerificationReport` recorded for the current stage — there is
no bypass. See [`docs/operations/runbooks.md`](../operations/runbooks.md)
for what a verification pass checks.

## Rollout strategy

`packages/iac.RolloutStrategy` supports blue-green, canary, and direct
rollout for this tier. See `packages/iac`'s own doc for the
traffic-percentage-at-step calculator canary rollouts use.

## Post-deploy verification

Run the deployment-verification checklist from
[`docs/operations/runbooks.md`](../operations/runbooks.md) after every
cloud deploy: liveness/readiness endpoints (Phase 003), migration
version (`packages/persistence`), and a guardrail smoke test
(`packages/guardrail`) all resolve before traffic is considered healthy.
