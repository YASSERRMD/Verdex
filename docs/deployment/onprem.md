# On-premises deployment guide

This guide covers deploying Verdex on customer-owned infrastructure
(`Tier = TierOnPrem` in [`packages/iac`](../../packages/iac), Phase 094).
It complements, and does not duplicate,
[`packages/iac`'s own design doc](../../packages/iac/doc/deployment.md);
see that document for the full `DeploymentProfile`/`ValidateManifest`
model this guide operationalizes.

## Prerequisites

- A local Kubernetes distribution (k3s, RKE2, MicroK8s) or, for a
  smaller footprint, plain Docker Compose — both forms are provided
  under [`infra/onprem/`](../../infra/onprem).
- Local persistent storage: this tier has **no managed-database
  fallback**. `postgres` is first-class in both the compose form (a
  named local volume) and the Kubernetes form
  (`postgres-pvc.yaml`, a `local-path`-class `PersistentVolumeClaim`).

## Manifests

Real, committed manifests live under [`infra/onprem/`](../../infra/onprem):

| File | Purpose |
|---|---|
| `configmap.yaml` | Non-secret environment configuration. Deliberately carries **no** `VERDEX_DATARESIDENCY_REGION` key — an on-prem deployment's data resides on the customer's own hardware, not a selected cloud region (`ValidateManifest` enforces this absence for this tier). |
| `deployment.yaml` | The `gateway` service `Deployment`: 2 replicas (a single-site installation rarely needs the 3-way redundancy the cloud tier's multi-AZ topology does), same container hardening as the cloud tier. |
| `service.yaml` | A `NodePort` `Service` — there is no external cloud load balancer to defer to on this tier. |
| `postgres-pvc.yaml` | A `local-path`-class `PersistentVolumeClaim` for the first-class local Postgres instance. |
| `docker-compose.yml` | A lighter-weight alternative to the Kubernetes manifests above, for smaller on-prem footprints. |

Container hardening does not relax for this tier: `deployment.yaml`'s
`securityContext` follows
[`packages/threatmodel/doc/Dockerfile.hardened`](../../packages/threatmodel/doc/Dockerfile.hardened)'s
runtime-settings notes exactly as the cloud tier does.

## Deployment profile

Register this deployment as a `packages/iac.DeploymentProfile` with
`Tier: TierOnPrem`. Unlike the cloud tier, `Region` must be **absent** —
`Validate` rejects a `Region` set on any non-cloud tier, since an
on-prem deployment is not pinned to a cloud region at all.
`SetupProfileName` remains optional, naming the completed
[`packages/setup`](../../packages/setup) wizard by reference (see
[`docs/admin/setup-guide.md`](../admin/setup-guide.md)).

## Secrets

The same secret-reference discipline applies as the cloud tier: any
`_PASSWORD`/`_DSN`/`_KEY`/`_SECRET`/`_TOKEN`-shaped environment key must
resolve through a `packages/config` reference, never a literal. On-prem
customers typically resolve these through a locally-run secrets manager
or a `vault://` reference, composing with
[`packages/keymanagement`](../../packages/keymanagement) exactly as the
cloud tier does — there is no separate on-prem secret mechanism to
learn.

## Validating manifests before applying

Validate an edited on-prem manifest structurally with
`packages/iac.ValidateManifest(iac.TierOnPrem, path)` before applying it
— this checks container hardening, secret-reference discipline, and
(specific to this tier) that no region key has crept back in.

## Promotion and rollout

The same `packages/iac.PromotionPipeline` gate and
`packages/iac.RolloutStrategy` options (blue-green, canary, direct)
described in [`docs/deployment/cloud.md`](cloud.md) apply to this tier —
promotion still refuses to advance without a passing
`DeploymentVerificationReport` for the current stage. A single-site
on-prem installation with only 2 replicas may reasonably choose direct
rollout over canary, given the smaller pool to stage a canary against.

## Post-deploy verification

Run the same deployment-verification checklist from
[`docs/operations/runbooks.md`](../operations/runbooks.md) used for
every tier: liveness/readiness endpoints, migration version, and a
guardrail smoke test all resolve before traffic is considered healthy.
