# Runbook: reasoning-quality regression

This is the human-readable counterpart to `Runbook`/`RunbookStep`
(`runbook.go`) and `qualityRegressionRunbook()`'s structured
procedure -- the two are kept in the same order deliberately, so this
document and the data model never drift apart silently. If you change
one, change the other in the same commit.

Attach this runbook to an `AlertRule` by setting
`AlertRule.RunbookName = "quality-regression"`.

## Scope

This runbook covers an `AlertEvent` with
`ConditionKind == ConditionQualityRegression`, produced by
`EvaluateQualityAlert` (`quality_alert.go`) when
`packages/reasoningeval.RegressionDetector.Compare` (Phase 062) flags a
threshold-exceeding drop in average `Overall` quality score between a
baseline and a current run.

**Every signal this runbook responds to is a non-binding monitoring
artifact about the reasoning pipeline's own output quality -- never a
conclusion about any specific case's merits.** This mirrors
`packages/reasoningeval.Alert.Message`'s own non-binding-quality-signal
suffix, which `EvaluateQualityAlert` reuses verbatim in
`AlertEvent.Detail`.

## Prerequisites

- The reviewer has access to `packages/reasoningeval`'s stored
  `QualityScore` history for both the baseline and current run IDs
  (`AlertEvent.Detail` names both).
- The reviewer understands this platform's non-binding guardrail
  (`packages/guardrail`, Phase 057) -- a quality regression never
  changes how that guardrail's disclaimer requirement is enforced; it
  is a separate, orthogonal signal.

## Procedure

1. **Acknowledge the alert and note the `AlertEvent.Detail`'s baseline
   vs current run IDs and average scores.**
   Owner: on-call engineer.
   The detail string already carries
   `run=%s vs baseline avg=%.4f vs current avg=%.4f (drop=%.4f)` --
   read it before pulling anything else.

2. **Pull the full `reasoningeval.RegressionResult.PerDimensionDrop`
   to identify which quality dimension drove the regression.**
   Owner: reasoning-quality reviewer.
   A regression concentrated in one `DimensionName` (e.g. citation
   fidelity vs internal coherence) points to a much narrower root cause
   than an across-the-board drop.

3. **Check whether the current run followed a prompt-template, model,
   or provider change; if so, that is the prime suspect.**
   Owner: reasoning-quality reviewer.
   `packages/prompts` version bumps, `packages/router` provider
   selection changes, and `packages/reasoningprofile` deployment-scope
   changes are the most common correlated causes.

4. **If a recent change is implicated, roll it back or gate it behind a
   flag until the regression is understood.**
   Owner: incident commander.
   Prefer reverting to the last known-good baseline over attempting a
   forward fix under pressure, consistent with this deployment's
   general incident-response posture.

5. **Re-run the comparison against a fresh sample once a fix is in
   place to confirm `Regressed` no longer holds.**
   Owner: reasoning-quality reviewer.
   Call `RegressionDetector.Compare` (or `EvaluateQualityAlert` again)
   against a new current-run sample; do not close the incident on the
   fix alone without re-measuring.

6. **Remember every quality signal here is a non-binding monitoring
   artifact -- never treat it as a conclusion about any specific case's
   merits.**
   Owner: reasoning-quality reviewer.
   This step is not optional process theater: it is the same
   guardrail this codebase enforces everywhere reasoning output
   surfaces, applied to the monitoring layer itself.

## Related seeded `AlertRule`

A tenant seeding a starter reasoning-quality rule typically registers
one `AlertRule` per jurisdiction/legal-family scope it monitors (or one
global rule with `JurisdictionCode` left empty), with
`RunbookName = "quality-regression"` and `Severity = SeverityWarning`
by default -- a quality regression is worth prompt review but is
rarely as urgent as an active SLO breach or a hard cost overage.
