# Admin and setup guide

This guide walks a deployment administrator through first-run
provisioning of a new tenant, using
[`packages/setup`](../../packages/setup)'s wizard (Phase 008) and
[`packages/jurisdiction`](../../packages/jurisdiction)'s registry (Phase
007). It complements, and does not duplicate, either package's own
design doc:
[`packages/setup/doc/setup-api-contract.md`](../../packages/setup/doc/setup-api-contract.md)
and
[`packages/jurisdiction/doc/jurisdiction-schema.md`](../../packages/jurisdiction/doc/jurisdiction-schema.md).

Before starting, decide which deployment tier this tenant runs on and
follow the matching guide under [`docs/deployment/`](../deployment/)
(cloud, on-prem, or air-gapped) — the setup wizard configures *what a
deployment reasons about* (jurisdiction, court, language, provider);
`packages/iac`'s `DeploymentProfile` governs *how it is deployed*. The
two compose by name reference only (`DeploymentProfile.SetupProfileName`),
never by import.

## Prerequisites

- A tenant record exists (`packages/tenancy`) and every request below
  carries that tenant's UUID in the request context, injected by
  upstream authentication middleware.
- The administrator's identity carries whatever `packages/identity`
  permission this deployment's gateway configuration requires for
  setup operations.

## The wizard's state machine

A tenant's setup wizard moves through the following states, in order,
enforced by `packages/setup`:

```
pending
  └─ in_progress
       └─ jurisdiction_selected
            └─ court_selected
                 └─ language_selected
                      └─ provider_configured
                           └─ completed
                                └─ locked  (terminal)
```

Every step below returns **422 Unprocessable Entity** if applied out of
sequence — this protects against double-submission or a client retrying
a step it already completed successfully. Once a wizard reaches
`locked`, every modification attempt returns **409 Conflict**: locking
is intentionally a one-way, administrative operation with no wizard-only
"unlock" endpoint.

## Step 1 — Select a jurisdiction

`POST /setup/jurisdiction` with `{ "jurisdiction_id": "<uuid>" }`.

Look up a jurisdiction ID first via `packages/jurisdiction`'s
`LookupService` (`GetByID`, `GetByCountry`). The package ships 13 seed
jurisdictions out of the box (`SeedData()`) spanning common-law,
civil-law, mixed, and Islamic-law legal families — from the UAE Federal
Supreme Court and Abu Dhabi Global Market Courts through the UK and US
supreme courts — see the full seed table in
[`packages/jurisdiction/doc/jurisdiction-schema.md`](../../packages/jurisdiction/doc/jurisdiction-schema.md#seed-jurisdictions).
An administrator may also register a new `Jurisdiction` first if the
tenant's court is not already seeded, subject to that package's
validation rules (a 2-letter uppercase ISO 3166-1 country code, a
recognized `CourtLevel`, a recognized `LegalFamily`, and at least one
2-letter lowercase ISO 639-1 language code).

This step implicitly starts the wizard (`StartSetup`) if it has not
already been started — calling it again once past `pending` is safe and
does not reset prior progress.

## Step 2 — Select a court level

`POST /setup/court` with `{ "court_level": "supreme" }` (or
`appellate`/`high`/`district`/`magistrate`/`special` — see
`packages/jurisdiction`'s `CourtLevel` constants). Court-level
validation is owned by the jurisdiction package, not the setup layer.

## Step 3 — Select reasoning languages

`POST /setup/languages` with `{ "languages": ["en", "ar"] }` — BCP-47
codes. At least one is required; this set governs which
`packages/multilingual` normalization paths and (later)
`packages/localization` (Phase 090) locales this tenant's reasoning
pipeline exercises.

## Step 4 — Configure the inference provider stub

`POST /setup/provider` with `provider_type` (required) plus optional
`endpoint`/`model_id`. This step records a placeholder only — real
provider integration happens through
[`packages/provider`](../../packages/provider)'s `LLMProvider`
abstraction and [`packages/router`](../../packages/router) elsewhere in
the stack, never by hardcoding a vendor here (`CONTRIBUTING.md`'s
"Provider Abstraction" rule). For an air-gapped deployment, this must
be a `local:`-prefixed provider — see
[`docs/deployment/airgapped.md`](../deployment/airgapped.md).

## Step 5 — Complete and lock

`POST /setup/complete` validates that a jurisdiction and at least one
language were recorded, then marks the wizard `completed`. Locking
(`StepLock`) is then performed programmatically by an administrator
confirming the configuration is correct — there is deliberately no REST
endpoint for locking, since it is a one-way administrative decision, not
a routine wizard step.

## Checking status at any point

`GET /setup/status` is always safe to call and returns the wizard's
current state, every field recorded so far, and `completed_at`/
`locked_at` if applicable.

## Error reference

| Error constant | HTTP status | Meaning |
|---|---|---|
| `ErrSetupNotFound` | 404 | No wizard record for the tenant |
| `ErrSetupLocked` | 409 | Wizard is locked — no further changes |
| `ErrSetupAlreadyComplete` | 409 | Wizard already completed |
| `ErrInvalidTransition` | 422 | Step applied out of sequence |
| `ErrMissingJurisdiction` | 400 | Completion attempted without a jurisdiction |
| `ErrMissingLanguages` | 400 | No languages selected |

## After setup completes

- Verify the deployment's compliance profile
  (`packages/compliance.Profile`, Phase 082) selects the frameworks
  applicable to this tenant's jurisdiction — see
  [`docs/security-compliance/overview.md`](../security-compliance/overview.md).
- If this tenant is on the air-gapped tier, run
  `packages/airgapped.VerifyZeroEgress` before go-live — see
  [`docs/deployment/airgapped.md`](../deployment/airgapped.md).
- Point the tenant's practitioners to
  [`docs/user-guide/judges-advocates.md`](../user-guide/judges-advocates.md)
  for the case workspace walkthrough.
