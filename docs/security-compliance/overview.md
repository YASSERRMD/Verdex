# Security and compliance overview

This page indexes Part 7 of the architecture
([`docs/architecture/overview.md`](../architecture/overview.md)) — the
security, privacy, and compliance packages — by phase number, with a
one-line summary of each. It links out to each package's own `doc/*.md`
rather than restating its design.

> **On the non-binding guardrail:** every mention of "guardrail,"
> "non-binding," or "disclaimer" below refers to
> [`packages/guardrail`](../../packages/guardrail) (Phase 057)'s
> code-enforced blocking of verdict/directive language and its
> `SignoffGate` requirement — never an actual verdict, ruling, or legal
> determination. Every package on this page produces or protects *draft
> analysis material*, not a binding output.

| Phase | Package | Summary |
|---|---|---|
| 075 | [`packages/encryption`](../../packages/encryption) | Encryption at rest and in transit for this platform's own stored data. See [`doc/encryption.md`](../../packages/encryption/doc/encryption.md). |
| 076 | [`packages/keymanagement`](../../packages/keymanagement) | Mandatory, centralized secret and key handling, including a break-glass path. See [`doc/key-management.md`](../../packages/keymanagement/doc/key-management.md). |
| 077 | [`packages/auditlog`](../../packages/auditlog) | Verdex's centralized, immutable, hash-chained, queryable audit trail — the durable persisted store every later security package writes to via its own `AuditSink`. |
| 078 | [`packages/dataresidency`](../../packages/dataresidency) | Guarantees a deployment's data stays within its declared jurisdiction boundary. |
| 079 | [`packages/airgapped`](../../packages/airgapped) | Assembles the fully offline deployment tier for sensitive courts — zero network egress beyond an explicit allow-list, enforced in code. See [`doc/airgapped-tier.md`](../../packages/airgapped/doc/airgapped-tier.md) and [`docs/deployment/airgapped.md`](../deployment/airgapped.md). |
| 080 | [`packages/accessgovernance`](../../packages/accessgovernance) | Certification/reporting capstone drawing together RBAC, case lifecycle, key-management break-glass, and the audit trail. |
| 081 | [`packages/privacy`](../../packages/privacy) | Honors data-subject rights (subject access requests, erasure, consent, retention) and enforces data minimization across every tenant's stored personal data. See [`doc/privacy.md`](../../packages/privacy/doc/privacy.md). |
| 082 | [`packages/compliance`](../../packages/compliance) | Maps this platform's controls to the legal/regulatory frameworks a deployment must answer to, with a real per-control gap-analysis evaluation. See [`doc/compliance.md`](../../packages/compliance/doc/compliance.md). |
| 083 | [`packages/threatmodel`](../../packages/threatmodel) | Systematically reduces attack surface by cataloguing, in structured STRIDE-style form, what can go wrong per named platform component; also home to the reference hardened-container template ([`doc/Dockerfile.hardened`](../../packages/threatmodel/doc/Dockerfile.hardened)) every deployment tier's manifests follow. |
| 084 | [`packages/vulnmanagement`](../../packages/vulnmanagement) | Continuous detection, triage, and remediation-SLA tracking of vulnerable components (dependencies/SCA, this platform's own source/SAST, containers), wired into `.github/workflows/ci.yml`'s `sca-scan`/`sast-scan` jobs. |
| 085 | [`packages/backupdr`](../../packages/backupdr) | Resilient backup and disaster recovery for this platform's own data. See [`doc/dr-runbook.md`](../../packages/backupdr/doc/dr-runbook.md) and [`docs/operations/incident-response.md`](../operations/incident-response.md). |
| 086 | [`packages/securitytesting`](../../packages/securitytesting) | An adversarial validation harness proving this platform's defenses actually hold under attack, rather than merely asserting that they exist. |

## Compliance control mapping

`packages/compliance`'s `SeedControls` ships a starter catalogue
covering UAE data-protection-style controls (`UAE-DP-*`), judicial-
records-handling controls (`JRH-*`, including a control mapped
specifically to enforcement of the non-binding disclaimer), and
international data-protection overlay controls (`INTL-DP-*`). Each
`Control.MappedTo` names the platform feature that satisfies it by
reference tag (e.g. `"packages/privacy.SAR"`, `"packages/auditlog"`) —
this is a reference convention, not an import; `packages/compliance`
does not depend on the packages it maps to. `RunGapAnalysis` evaluates
each control against collected `ControlEvidence` to a real
`StatusSatisfied`/`StatusPartiallyMet`/`StatusGap` status — never a
stub that always reports satisfied.

## Audit trail

Every package above that records an event (control registration,
evidence addition, sign-off decision, access-governance certification,
key-management break-glass use) writes to the same
[`packages/auditlog.Store`](../../packages/auditlog) (Phase 077) — a
durable, hash-chained, queryable sink. There is no second audit table
anywhere in this platform. Use `auditlog.VerifyChain` to confirm the
chain's integrity — see
[`docs/operations/incident-response.md`](../operations/incident-response.md)
for when to run it during an incident.

## CI-enforced security gates

[`.github/workflows/ci.yml`](../../.github/workflows/ci.yml) runs, on
every pull request:

- `secrets-scan` (gitleaks) — blocking.
- `sca-scan` (govulncheck, Phase 084) — informational
  (`continue-on-error: true`); see the workflow's own comment for why.
- `sast-scan` (gosec, Phase 084) — informational
  (`continue-on-error: true`) for the same reason.
- `container-scan` — currently a documented placeholder, since this
  repository has no Dockerfile yet (see
  [`packages/threatmodel/doc/Dockerfile.hardened`](../../packages/threatmodel/doc/Dockerfile.hardened)
  for the reference template a future phase would use).
- `branch-policy` and `sign-artifacts` (Phase 095,
  `packages/cicdgate`) — blocking.

See [`docs/operations/runbooks.md`](../operations/runbooks.md) for the
day-to-day operational reliability signals that complement these
CI-time gates, and
[`docs/operations/incident-response.md`](../operations/incident-response.md)
for what to do when one of them catches something in anger.

## Deployment-tier security posture

Every tier under [`docs/deployment/`](../deployment/) inherits the same
container-hardening baseline
([`packages/threatmodel/doc/Dockerfile.hardened`](../../packages/threatmodel/doc/Dockerfile.hardened))
and the same secret-reference discipline (never a literal value in a
committed manifest). The air-gapped tier additionally composes
`packages/dataresidency`'s `AirGappedPreset` and
`packages/airgapped`'s zero-egress `Profile`/`Conformance` model — see
[`docs/deployment/airgapped.md`](../deployment/airgapped.md).
