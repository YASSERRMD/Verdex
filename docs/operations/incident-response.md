# Incident response runbook

This runbook covers general platform incidents — a service outage, a
security event, or a data-integrity concern — that are not specifically
a backup/disaster-recovery scenario. For restoring a tenant's data from
backup after a region outage or data-class corruption/deletion, use
[`packages/backupdr`'s own runbook](../../packages/backupdr/doc/dr-runbook.md)
instead; this document mirrors its shape deliberately, so an on-call
engineer moving between the two finds the same structure.

This runbook assumes an incident has already been detected (via
whatever alerting/on-call tooling this deployment uses — see
[`docs/operations/runbooks.md`](runbooks.md) for the reliability signals
that typically trigger detection) and someone is now executing the
response.

## Scope

This runbook covers scenarios such as:

- A service or dependency outage surfaced by a tripped
  [`packages/reliability.CircuitBreaker`](../../packages/reliability),
  an exhausted `ErrorBudget`, or a failing `CheckKindHealthEndpoint`
  post-deploy check.
- A suspected security event: an anomalous access pattern flagged by
  [`packages/accessgovernance`](../../packages/accessgovernance) (Phase
  080), a prompt-injection `Finding` from
  [`packages/threatmodel`](../../packages/threatmodel) (Phase 083), or a
  cross-tenant/cross-case access attempt that should have been blocked
  by [`packages/knowledgeisolation`](../../packages/knowledgeisolation)
  (Phase 047) or `packages/tenancy`.
- A suspected data-integrity concern: an
  [`packages/auditlog.VerifyChain`](../../packages/auditlog) failure
  reporting a broken hash chain, or a
  [`packages/treevalidation`](../../packages/treevalidation) (Phase 040)
  integrity check failing against a case's assembled reasoning tree.
- A guardrail failure: a `CheckKindGuardrailSmokeTest` post-deploy check
  failing, indicating the non-binding-analysis guarantee
  ([`packages/guardrail`](../../packages/guardrail), Phase 057) is not
  actually enforcing on a live deployment.

If the incident is instead "we need to restore data from backup,"
declare it here, then hand off to
[`packages/backupdr`'s runbook](../../packages/backupdr/doc/dr-runbook.md)
at its step 2 (scope confirmation) — the two runbooks share the same
incident-declaration step so there is exactly one place that happens.

## Prerequisites

- The on-call engineer has access to this deployment's observability
  stack (`packages/observability`, Phase 003) and, where applicable,
  `packages/reliability`'s `CircuitBreakerRegistry` state.
- The incident commander has confirmed which tenant(s), if any, are
  affected — every investigation step below should stay scoped to the
  affected tenant(s) rather than querying across tenant boundaries.

## Procedure

1. **Declare the incident and page the on-call rotation.**
   Owner: on-call engineer.
   Standard incident-declaration process for this deployment; not
   specific to any particular failure category below.

2. **Classify the incident: outage, security event, or data-integrity
   concern.**
   Owner: incident commander.
   The remaining steps branch by classification — an outage moves
   toward restoring service; a security event moves toward containment
   and forensics; a data-integrity concern moves toward verification and
   scoping any actual corruption. Re-classify at any point if new
   evidence changes the picture — do not stay committed to an initial
   guess.

3. **For an outage: identify the failing dependency and consult its
   circuit-breaker/degradation state.**
   Owner: on-call engineer.
   Check `packages/reliability.CircuitBreakerRegistry` for which named
   dependency (if any) has tripped `Open`, and whether a
   `packages/reliability.Degrader[T]` is already serving a
   `DegradationMode`-marked reduced result rather than failing outright
   — the system may already be correctly protecting itself, in which
   case the priority is restoring the underlying dependency, not
   "fixing" the degradation wrapper itself. See
   [`docs/operations/runbooks.md`](runbooks.md) for how to read these
   signals.

4. **For a security event: contain first, then investigate.**
   Owner: on-call engineer, with the tenant administrator liaison
   looped in immediately if tenant data may be affected.
   Containment depends on the event: revoke a compromised credential
   via `packages/keymanagement`'s break-glass path (Phase 076,
   composing with `packages/accessgovernance`'s certification model,
   Phase 080), tighten a `packages/airgapped.NetworkPolicy`
   allow-list if an air-gapped deployment's zero-egress guarantee is in
   question (see
   [`docs/deployment/airgapped.md`](../deployment/airgapped.md)), or
   disable an affected integration via `packages/integration` (Phase
   087). Do not wait for a full root-cause understanding before
   containing.

5. **For a data-integrity concern: verify before assuming corruption.**
   Owner: on-call engineer.
   Run `packages/auditlog.VerifyChain` against the affected event range
   to confirm (or rule out) a broken hash chain, and/or
   `packages/treevalidation`'s integrity checks against the affected
   case's assembled reasoning tree. Treat a clean verification result as
   ruling out this incident's classification — re-classify per step 2
   rather than continuing to search for corruption that the check did
   not find.

6. **Verify the non-binding guardrail is still enforcing.**
   Owner: on-call engineer.
   Regardless of classification, if there is any chance a deployed
   change is implicated, run the same guardrail smoke test
   `packages/iac`'s `CheckKindGuardrailSmokeTest` runs post-deploy (see
   [`docs/operations/runbooks.md`](runbooks.md)) against the live,
   currently-running instance. A guardrail failure during an unrelated
   incident is itself a second, higher-priority incident — escalate it
   as such rather than treating it as a side note.

7. **Restore service or close containment, and confirm recovery.**
   Owner: on-call engineer.
   For an outage: confirm the dependency is healthy again and the
   circuit breaker has returned to `Closed` (or `HalfOpen` and
   recovering). For a security event: confirm the contained vector is
   actually closed (e.g. the revoked credential no longer authenticates,
   the tightened network policy actually blocks the previously-allowed
   target). For a data-integrity concern: confirm the affected range
   re-verifies clean, or scope exactly what is corrupted and hand off to
   [`packages/backupdr`'s runbook](../../packages/backupdr/doc/dr-runbook.md)
   for restoration.

8. **Notify affected tenant administrators and, where applicable,
   record a compliance/breach-notification event.**
   Owner: tenant administrator liaison.
   If the incident involved actual data exposure, loss, or a security
   compromise, this is coordinated through
   `packages/compliance`'s (Phase 082) `CategoryBreachNotification`
   controls and `packages/privacy`'s (Phase 081) existing
   subject-notification machinery — this runbook does not reinvent
   either. See
   [`docs/security-compliance/overview.md`](../security-compliance/overview.md).

9. **Record the incident.**
   Owner: incident commander.
   Capture classification, affected tenant(s), detection time,
   containment/restoration time, and root cause (once known) for the
   post-incident review. If the incident also touched backup/DR
   territory, cross-reference the `RestoreDrill`-shaped record
   `packages/backupdr`'s own runbook produces.

10. **Hold a post-incident review and file follow-up actions.**
    Owner: incident commander.
    Standard blameless postmortem process for this deployment. Any
    reliability signal that should have caught this sooner (a missing
    `SLO`, a circuit breaker that should have tripped earlier, a
    guardrail smoke test that should have caught a regression before it
    reached production) becomes a tracked follow-up, not a one-off
    lesson that is forgotten once the incident channel goes quiet.

## Security-specific validation

Where a security event's containment needs adversarial confirmation
that a defense actually holds (rather than merely appears configured),
[`packages/securitytesting`](../../packages/securitytesting) (Phase
086) is the adversarial validation harness this platform already
ships — run the relevant `Scenario` against the now-contained
environment rather than asserting from configuration alone that the
defense holds.

## Related runbooks

- [`docs/operations/runbooks.md`](runbooks.md) — routine reliability
  posture and post-deploy verification (not incident-specific).
- [`packages/backupdr/doc/dr-runbook.md`](../../packages/backupdr/doc/dr-runbook.md)
  — restoring tenant data from backup (Phase 085).
