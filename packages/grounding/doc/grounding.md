# Grounding checks (`packages/grounding`)

Phase 061's goal, verbatim from the implementation plan:

> Verify every assertion is grounded in case or law.

By Phase 055, `packages/synthesisagent` could produce an `Opinion`: one
`TentativeConclusion` per issue, each already grounded at the *individual
conclusion* level — `groundConclusion` (see `packages/synthesisagent/
ground.go`) strips any `SupportingFactIDs`/`SupportingRuleIDs` the model
proposed that don't actually exist in the case's tree, before the
conclusion is ever added to the `Opinion`. `packages/firstpartyagent` and
`packages/secondpartyagent` do the same for their own per-issue arguments.
What none of those packages do is re-check the *fully assembled* opinion
once it has been composed, exported, or otherwise handled downstream —
and none of them check whether the opinion's own **prose** (the actual
sentence text a human reviewer reads) still agrees with the facts it
claims to rely on. This package is that final pass.

## What "grounded" means here

A `Claim` extracted from a `TentativeConclusion` is **grounded** when it
is independently verifiable against the case's own record, along three
axes:

1. **Reference grounding** (`reference.go`) — every `SupportingFactIDs`/
   `SupportingRuleIDs` entry the conclusion claims to rely on must exist
   as a real `irac.Node` in the case's tree. This re-runs the same check
   `synthesisagent.groundConclusion` already performed, but as an
   independent second pass over the *opinion as it stands now* — not to
   distrust `synthesisagent`, but because nothing prevents an `Opinion`
   from being serialized, passed through other code, or reconstructed by
   a caller between synthesis and finalization, and this package's whole
   purpose is to not simply trust that the original grounding is still
   valid.
2. **Citation grounding** (`citations.go`) — every controlling rule a
   conclusion cites must independently verify against
   `packages/citation`: not hallucinated, not attributed to the wrong
   case. This package does not reimplement citation verification; it
   calls `citation.Verify` and translates the result with
   `citation.FindingsFromVerification`, exactly as any other caller of
   `packages/citation` would.
3. **Numeric/date consistency** (`consistency.go`) — every concrete
   figure (an amount, a percentage, a count) or calendar date mentioned in
   a conclusion's `Text` must appear, verbatim, in the text of that same
   conclusion's own supporting fact nodes. A conclusion is free to *reason
   about* its facts however it likes, but a bare number or date stated as
   if it were itself a fact must be traceable to one — this is the
   package's answer to "did the model just make up a number that sounds
   plausible."

A `Claim` that cannot be checked at all — e.g. a numeric/date claim on a
conclusion with zero `SupportingFactIDs` to check against — is
**unverifiable**, not ungrounded: `OutcomeUnverifiable` is a coverage gap
(surfaced as a `SeverityWarning` `CodeUnverifiableClaim` `Finding`), not a
confirmed fabrication. This distinction matters for `CanFinalize`: it
never blocks on `OutcomeUnverifiable`, only on `SeverityCritical`
findings — an opinion that says too little to check is not, on its own,
untrustworthy in the way a *demonstrably false* claim is.

## What this package deliberately does not do

- **It does not re-run synthesis.** `Check` never mutates or regenerates
  an `Opinion`; it reads one and reports on it. A `Report` finding a
  problem does not fix the problem — a caller decides what to do about a
  flagged `Opinion` (regenerate it, hand it to a human reviewer, etc.).
- **It does not check tree structure.** Orphaned nodes, cycles, and
  traceability gaps in the tree itself are `packages/treevalidation`'s
  job (see below) — this package assumes the tree it reads via
  `graph.GraphStore.Traverse` is whatever it is, and only asks "does the
  opinion's content agree with it."
- **It does not gate on human sign-off.** That is `packages/guardrail`'s
  job (see below) — this package has no concept of a reviewer's approval.
- **It does not resolve or format citation text.** It calls
  `citation.Verify` (existence only), not `citation.Resolve`
  (formatting) — this package only needs to know whether a cited node is
  real, not what its citation string should look like.
- **It uses no ML models.** Numeric and date extraction (`numeric.go`)
  are deterministic regex patterns, mirroring
  `packages/timeline/event.go`'s and `packages/evidence`'s "no ML
  models, rule based" convention exactly. Reused independently rather
  than importing `packages/timeline` (which is not otherwise a
  dependency of the reasoning pipeline packages this one sits next to),
  since three regex patterns are cheaper to duplicate than a new package
  dependency.

## Relationship to `packages/citation`

`packages/citation` is imported, not reimplemented. `verifyCitations`
(`citations.go`) builds a minimal `citation.CitedUnit{NodeID, CaseID}` per
controlling rule ID, calls `citation.Verify`, and translates every
non-`StatusVerified` result into a `citation.Finding` via
`citation.FindingsFromVerification`. `ConclusionResult.CitationFindings`
keeps these as native `citation.Finding` values — deliberately *not*
converted into this package's own `Finding` type — so a caller inspecting
a `Report` can tell at a glance which findings came from this package's
own claim-verification logic and which came from `packages/citation`'s
independent check, while `Report.Summary`/`HasCritical`/`CanFinalize`
still fold both into one picture (`Report.AllCitationFindings` flattens
them for exactly that purpose).

## Relationship to `packages/treevalidation`

`packages/treevalidation` answers "is this reasoning tree
*structurally* sound" — no orphaned nodes, no cycles, every conclusion
traceable to a rule and a fact, confidence propagated consistently. This
package answers a different question: "does this *specific opinion's
prose* actually say what its cited facts and rules support." A tree can
pass every `treevalidation` check and still have an `Opinion` synthesized
over it whose sentence states a date or dollar figure that doesn't
actually appear anywhere in the fact it cites — `treevalidation` has no
way to catch that, because it never looks at conclusion prose at the
sentence level, and this package never looks at tree structure (no cycle
detection, no orphan detection — `Check` calls
`graph.GraphStore.Traverse` purely to build a node-ID lookup table, not to
re-validate tree shape). Neither package imports the other.

## Relationship to `packages/guardrail`

`packages/guardrail.CanFinalize` blocks on human sign-off **state**: has
a judge or reviewer actually approved this case's output. This package's
own `CanFinalize` blocks on grounding **content**: does the opinion's own
text hold up against the record, independent of whether anyone has
reviewed it yet. The two gates are complementary and orthogonal, mirroring
exactly how `packages/guardrail/signoff.go` documents its own
relationship to `packages/treevalidation.CanFinalize` — a caller wanting
every finalization guarantee this codebase can offer calls all three
gates (`treevalidation.CanFinalize`, `grounding.CanFinalize`,
`guardrail.CanFinalize`) before treating a case's output as ready. No two
of these three packages import each other.

## The `Report` shape

`Check(ctx, caseID, store, opinion)`:

1. Authorizes the caller via `RequireCheckPermission` (gated on
   `identity.PermViewCase`, mirroring `packages/reasoningtrace` and
   `packages/knowledgeapi`'s authorize-then-proceed pattern exactly) —
   before a single store read.
2. Loads the case's entire tree once via one
   `graph.GraphStore.Traverse(ctx, TraversalQuery{CaseID: caseID})` call,
   indexed by node ID (`nodesByID`) — cheaper than one `GetNode` call per
   referenced ID, since a typical opinion references a large fraction of
   the case's own nodes anyway.
3. For every `TentativeConclusion`: extracts its `Claim`s (`extract.go`),
   verifies each one (`reference.go`/`consistency.go`), independently
   re-verifies its cited rules via `packages/citation`
   (`citations.go`), and computes a per-conclusion `ConfidenceScore`
   (`confidence.go`).
4. Assembles every `ConclusionResult` into a `Report`, with a flattened
   `Findings` list and an overall `OpinionScore` — the unweighted mean of
   every conclusion's own score.

`CanFinalize(report)` (`gate.go`) is the hard gate: `false` plus a
wrapped `ErrCriticalFindings` whenever `report.HasCritical()` is true
(across both `Findings` and every `ConclusionResult.CitationFindings`),
`true` otherwise — including when a `Report` has only
`SeverityWarning`/`SeverityInfo` findings, or none at all.

## Confidence scoring

`scoreConclusion` (`confidence.go`) is the fraction of *checked* claims
that came back grounded: `OutcomeUnverifiable` claims are excluded from
both the numerator and denominator (an unchecked claim should not count
for or against the score — the coverage gap is already surfaced as its
own `Finding`), and every `SeverityCritical` `citation.Finding` counts as
one additional ungrounded check. A conclusion with nothing checkable at
all scores `1.0` — "no checkable assertions" is not evidence of
fabrication, mirroring `packages/citation.ScoreConfidence`'s own stance
on absence of evidence. `scoreOpinion` is the unweighted mean of every
conclusion's score.

## What a future phase could add

- This package only ever reads `graph.GraphStore` directly, mirroring
  `packages/citation`'s own boundary rather than depending on
  `packages/knowledgeapi`'s HTTP-shaped service layer — a future HTTP
  surface for triggering a grounding check (analogous to
  `knowledgeapi`'s `ValidationStatus` endpoint) is a thin wrapper this
  package does not itself need to provide.
- Numeric/date consistency checking is a substring match today
  (`strings.Contains`), deliberately simple rather than attempting
  unit-normalization (e.g. treating "$4,500" and "$4,500.00" as
  equivalent) or fuzzy date-format reconciliation — a future phase could
  extend `consistency.go` with normalized comparison without changing
  this package's public shape.
