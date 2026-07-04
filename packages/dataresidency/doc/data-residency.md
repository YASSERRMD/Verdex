# Data residency & sovereignty (Phase 078)

## Goal

Guarantee that a Verdex deployment's data stays within its declared
jurisdiction boundaries: which regions may store/process a
deployment's data, whether the deployment's actual database lives
where it claims to, whether providers currently in use are within the
allowed regions, and whether a fully air-gapped deployment's policy is
consistently paired with the router's local-only enforcement.

Jurisdiction parameterization (Phase 58, "one engine, reweighted by
legal family per deployment") is the reasoning-side differentiator.
This phase is the *enforcement* side: it does not decide legal
outcomes, it decides where data is allowed to physically live and
move.

## What this package composes from, versus what is new

`packages/dataresidency` is deliberately **not** a new data plane. It
is a policy/enforcement layer over four existing pieces:

| Existing piece | What it already provides | What this phase adds |
|---|---|---|
| `packages/router` (Phase 012) | `RoutingPolicy.AirGappedOnly` + `LocalOnlyEnforcer`: filters provider selection to `local:`-prefixed providers only | `ComposeWithRouterAirGap` checks the two are paired consistently; **no change to `packages/router` itself** |
| `packages/provider` (Phase 011) | `Capability` descriptor per provider/model | One additive field: `Capability.Region` (see below) |
| `packages/tenancy` / `packages/persistence` | `Deployment` (ID, TenantID, Profile, Status) | `ResidencyPolicy` and `RegionPin`, both keyed by `DeploymentID`, composing alongside `Deployment` rather than embedding into it |
| `packages/auditlog` (Phase 077) | Centralized, hash-chained, queryable audit trail; `KindSystem` taxonomy entry | `AuditSink` projects every transfer-check and `Verify` report into `auditlog.Event` |
| `packages/accounting` (`AlertSink` idiom) | `AlertSink`/`LoggingAlertSink`/`MultiAlertSink` shape | `dataresidency.AlertSink` mirrors the same shape for `ViolationEvent` |
| `packages/keymanagement` / `packages/encryption` | Key material locality (a residency concern in its own right) | Referenced in this doc as a related-but-separate concern; not duplicated here |

### The one additive change: `provider.Capability.Region`

`packages/provider/types.go`'s `Capability` struct gained a `Region
string` field. This was a deliberate, minimal, additive change (see
that field's doc comment) rather than inventing a parallel
provider-locality registry, because:

- `Capability` is already the per-provider/model descriptor every
  adapter (`packages/adapters/{anthropic,openai,gemini,local}`)
  constructs via a keyed struct literal, so adding a field cannot
  break any existing caller.
- An empty `Region` is treated as "undeclared" and always fails a
  residency check (`CheckProviderLocality` rejects it) rather than
  being silently treated as a wildcard -- fail closed, not fail open.

### `packages/router` is unmodified

Task 4 explicitly allows demonstrating provider-locality enforcement
"as a composable guard function" without touching `router` itself, as
long as there is a real test proving it rejects a disallowed-region
provider selection. This package does exactly that:

- `CheckProviderLocality(ctx, capability, policy) error` — checks one
  candidate.
- `FilterProvidersByLocality(ctx, capabilities, policy)
  []provider.Capability` — narrows an already-resolved candidate list.

A caller sits between router's existing selection (`ProviderSelector`,
`LocalOnlyEnforcer`) and the actual dispatch, and calls
`FilterProvidersByLocality` on the resolved candidate chain before
picking one. See `integration_test.go`'s
`TestProviderSelectionComposesWithResidencyGuard` for the worked
example: a two-provider candidate chain (`us` then `eu`) with a
policy restricted to `eu` correctly rejects the first candidate and
narrows the chain to the second.

## The pieces

### `ResidencyPolicy`

```go
type ResidencyPolicy struct {
    DeploymentID   uuid.UUID
    AllowedRegions []string        // fail-closed: empty means no region is allowed
    DataClassRules []DataClassRule // per-data-class narrowing, e.g. "pii" -> ["eu"]
    StrictMode     bool            // forbid ANY cross-region move, even within AllowedRegions
}
```

`AllowedRegions` says *which regions are acceptable places for data to
live*. `StrictMode` says *data must never move between regions at
all, period* — stricter than a plain allow-list, and the shape used by
the air-gapped preset (`AirGappedPreset`): `StrictMode=true`,
`AllowedRegions=[]`.

### `RegionPin` and the storage-locality check

`RegionPin` pins a deployment's expected database region to a list of
host-substring patterns. `RegionPin.ValidateDSN(dsn)` parses the DSN
with `pgxpool.ParseConfig` — the exact same parser
`packages/persistence.Open` uses to build its connection pool — and
asserts the resulting host matches one of the pinned patterns. This is
a real, testable assertion against the live configuration, not a
comment: `region_pin_test.go` proves both the matching and
mismatching cases.

### `CheckTransfer` — blocking cross-border movement

```go
func CheckTransfer(ctx context.Context, sourceRegion, destRegion string, policy *ResidencyPolicy) error
```

A composable guard, callable before any cross-region operation:
routing to a provider in another region, replicating storage,
exporting a report across a border. `CheckTransferForDataClass` is the
same guard narrowed to a specific `DataClassRule`.

### `Verifier.Verify` — startup/periodic conformance check

```go
func (v *Verifier) Verify(ctx context.Context, deploymentID uuid.UUID) (Report, error)
```

Loads the deployment's `ResidencyPolicy`/`RegionPin` via a
`PolicySource`, then runs three checks against `LiveConfig` (live DB
DSN, live provider regions in use, and the router's actual
`RoutingPolicy`):

1. `CheckStorageRegion` — does the live DB DSN's host match the pin?
2. `CheckProviderRegions` — is every provider region currently in use
   allowed by the policy?
3. `CheckAirGapComposition` — if the policy is air-gapped, is
   `router.RoutingPolicy.AirGappedOnly` actually set?

`Verify` never returns an error just because a check failed — failures
are represented in the returned `Report` (`Report.Passed()` /
`Report.Failures()`) so a caller can inspect exactly what is wrong; it
only errors on a structural problem (no policy configured, a
dependency failure). `verify_test.go`'s
`TestVerify_CatchesLiveStorageRegionMisconfiguration` proves this
catches a real DB-host/region mismatch, not just a policy-shape check.

### Air-gapped preset (this phase's slice of it)

```go
func AirGappedPreset(deploymentID uuid.UUID) ResidencyPolicy // StrictMode=true, AllowedRegions=[]
func ComposeWithRouterAirGap(policy *ResidencyPolicy, routingPolicy router.RoutingPolicy) error
```

Phase 078's scope stops here: the **policy-side preset** and a check
that it is paired with router's existing `AirGappedOnly` flag rather
than applied independently. `ComposeWithRouterAirGap` returns
`ErrAirGappedRequiresLocalFlag` if a policy claims air-gapped
semantics but the router flag is off — this is deliberately a
composition *check*, not a mechanism that flips the router flag itself
(this package never constructs or mutates a `router.RoutingPolicy`).

**Phase 079** builds the full offline deployment tier (bundled model
runtime, offline corpus provisioning, zero-egress verification, etc.)
around this preset.

### Audit and alerting

`AuditSink` (task 7) records every `CheckTransfer` outcome and every
`Verify` report via `packages/auditlog.Store`, using the existing
`KindSystem` taxonomy entry — no new logging channel. `Guard` (in
`guard.go`) composes `CheckTransfer` + `AuditSink` + `AlertSink` into
one call so a caller gets audit-and-alert for free instead of wiring
both by hand at every call site.

`AlertSink` (task 8) mirrors `packages/accounting.AlertSink`'s shape
exactly (`Send(ctx, event) error`, plus `LoggingAlertSink` /
`NoOpAlertSink` / `MultiAlertSink`), so operators already familiar with
`packages/accounting`'s or `packages/signoff`'s alerting recognize this
one immediately. `LoggingAlertSink` is a real, working implementation,
not just an interface.

### Access control

`Manager.SetPolicy` / `Manager.SetRegionPin` require the authenticated
actor (`identity.UserFromContext`) to hold `identity.PermManageSettings`
— reused, not a new permission, since residency policy is
deployment/tenant configuration, the same category
`packages/keymanagement` and `packages/config` already gate on that
permission. Reading a policy (`Manager.Policy` / `Manager.RegionPin`)
is ungated, since `CheckTransfer`/`Verify`/`Guard` all need to consult
it freely.

## What is explicitly out of scope here

- **Key material locality** remains `packages/keymanagement` /
  `packages/encryption`'s concern. This package references it as a
  related residency dimension but does not duplicate key-residency
  logic.
- **The full air-gapped deployment tier** (bundled local model
  runtime, offline corpus/update/license mechanisms, zero-egress
  verification) is Phase 079, built around the `AirGappedPreset` this
  phase provides.
- **Mutating `packages/router`'s selection internals** — this phase
  demonstrates the provider-locality guard as a composable function a
  caller (or a future router change) invokes; `packages/router` itself
  is unmodified.
