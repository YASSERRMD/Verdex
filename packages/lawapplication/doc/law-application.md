# Law-application module (`packages/lawapplication`)

Phase 054 sits between the evidence-weighing module
(`packages/evidenceweighing`, Phase 053) and the downstream synthesis
stage (`packages/synthesis`-equivalent, Phase 055). Its job is to apply a
case's controlling legal rules to its weighed facts, per issue: which
rules govern which issue, which weighed facts back each rule's
application, how strongly statute versus precedent should count given
the case's legal family, where two rules pull in different directions,
and what citation backs every rule actually applied.

## Design decision: heuristic, not LLM-backed

This package is deliberately **not** an `packages/agentframework` agent,
matching Phase 053's precedent rather than Phases 050-052's. The plan's
language for this phase — "map," "apply," "weight," "handle," "produce,"
"capture," "cite," "output" — matches the deterministic/heuristic
bookkeeping style of `packages/application`'s `WeightByLegalFamily`
(Phase 037) and `packages/evidenceweighing`'s `Weigh` (Phase 053), not
the LLM-agent style of `packages/issueagent`/`packages/firstpartyagent`/
`packages/secondpartyagent`. Every core function in this package —
`MapIssueToControllingRules`, `BuildElementFactMap`,
`DetectConflictingAuthority`, `WeightByOrigin`, `ComputeConfidence` — is
plain Go, unit-testable with in-memory fixtures and no model call. This
package does not import `packages/agentframework`, `packages/router`, or
any provider.

This was a deliberate choice, for the same reasons Phase 053 gave:

- **Determinism and auditability.** Every `IssueApplication` carries an
  explicit `Steps` reasoning trail derived directly from the inputs —
  which rules were found and how, which facts back them, whether a
  conflict was detected, how confidence was blended — with no risk of a
  model inventing a justification after the fact.
- **The inputs are already structured.** By this stage, the available
  signals are rule/issue/fact node IDs, `Argument.SupportingRuleIDs`/
  `SupportingFactIDs` (structured IDs, not prose), and
  `evidenceweighing.FactWeight` values (numbers). Mapping rules to
  issues, aggregating cited facts per rule, and flagging opposing-party
  rule citations are all set operations over IDs already provided by
  upstream agents — there is no unstructured text here that would
  justify a model call for the core bookkeeping.
- **Consistency with sibling packages.** `packages/application` and
  `packages/evidenceweighing` both score/aggregate this same
  deterministic way; an LLM-backed law-application stage would be
  inconsistent with the established pattern for every other
  bookkeeping/scoring module in the tree-reasoning pipeline.

As with Phase 053, the plan permits an optional narrow LLM sub-task, and
this package does not add one: element-to-fact "application" here means
structuring which weighed facts back which rule, not synthesizing prose
about *how* a fact satisfies a legal element. That natural-language
synthesis is deliberately left to Phase 055's synthesis agent, which has
the fuller context (both `ArgumentSet`s' `Claim` text, this package's
`IssueApplication`s, and `evidenceweighing.Result`) to write it well.

## Composes with, does not duplicate

| Package | Owns | This package's relationship |
|---|---|---|
| `packages/irac` | `RuleNode`, `IssueNode`, `EdgeGoverns`. | Read-only. This package never constructs, mutates, or persists a tree node — its `RuleRef` input type is a narrow, caller-supplied projection of a `RuleNode`'s ID/Text/LegalFamily, not a dependency on the full type. |
| `packages/knowledgeapi` | `GetTree`, `NodeDTO`, `ResolveCitation`, `CitationDTO`. | The intended way a caller obtains `[]RuleRef`, each issue's `GoverningRuleIDs` (from `Rule--governs-->Issue` edges), and citation resolution — this package itself never calls `knowledgeapi` directly. `CitationLookupFunc` is this package's own narrow function type a caller adapts `KnowledgeAPI.ResolveCitation` into (see `knowledgeapi_test.go`), keeping this package's core logic free of I/O and fully unit-testable, mirroring `evidenceweighing`'s "callers fetch, this package computes" boundary. |
| `packages/application` (Phase 037) | `Origin`/`OriginatedRule`, `WeightByLegalFamily` — a **tree-assembly-time** rule/origin/legal-family model. | **Not imported.** By the time this package's reasoning stage runs (post-argument-construction), no `application.OriginatedRule` wrapper survives into the tree — a caller only has `irac.RuleNode`/`knowledgeapi.NodeDTO`, which carries no `Origin` field. This package reimplements the *same weighting concept* locally (`Origin`, `WeightByOrigin`, `OriginProfile`/`ProfileForFamily`) against its own `RuleRef` type and its own origin-inference heuristic, rather than reconstructing an `application.Origin` guess just to call `application.WeightByLegalFamily` — see "Origin-inference limitation" below. |
| `packages/evidenceweighing` (Phase 053) | `Result`, `FactWeight` — reasoning-stage evidentiary weights. | **Primary input.** This package imports it only for `Result`/`FactWeight` (never its internal scoring logic) and reads `FactWeight.Weight`/`Contradicted` directly into `ElementFactEntry`, per Phase 053's own documented expectation that Phase 054 "read `FactWeight.Weight` ... rather than raw `FactNode.Confidence`." |
| `packages/firstpartyagent` (Phase 051) / `packages/secondpartyagent` (Phase 052) | `ArgumentSet`/`Argument`, including `SupportingRuleIDs`. | **Primary input**, converted into this package's own party-agnostic `ArgumentRef` view (mirroring `evidenceweighing.CitingArgument`'s identical conversion), so `Apply` never has to special-case which of the two Argument shapes it is reading. `SupportingRuleIDs` is the signal `MapIssueToControllingRules` unions with governs-edge rules. |
| `packages/issueagent` (Phase 050) | `FramedIssue`, `GoverningQuestions`. | Read-only context. `IssueInput.Issue` carries the `FramedIssue` for its `SourceIssueNodeID` and `GoverningQuestions` alongside the analysis, but this package's own mapping/aggregation logic keys only off `SourceIssueNodeID` — it does not parse or reason over `GoverningQuestions` text. |
| `packages/citation` | `Origin` (`OriginUnknown`/`OriginStatute`/`OriginPrecedent`), `Resolver`. | Not imported directly, but `Origin`'s three values are deliberately identical to `citation.Origin`'s, so a `CitationLookupFunc` implementation can pass a resolved `CitationDTO.Origin` string straight through as a `lawapplication.Origin` (see `RuleRef.OriginHint` and `AttachCitations`). |
| `packages/jurisdiction` | The richer jurisdiction/legal-family domain model. | **Not imported.** `LegalFamily` is this package's own opaque `string` type, mirroring `evidenceweighing.LegalFamily`'s and `application`'s identical decoupling convention. |
| `packages/evidenceweighing`'s `store.go` convention | `Repository`/`InMemoryRepository` per-case persistence. | Not imported, but `store.go` here deliberately mirrors its shape exactly, so the in-memory-store convention stays consistent across the monorepo. |

## Origin-inference limitation (known tradeoff)

`knowledgeapi.NodeDTO` does not carry `irac.RuleNode.LegalFamily` or any
`Origin` field — a rule read via `GetTree` is just an ID/Text/Confidence
projection. This package resolves a rule's `Origin` in priority order:

1. **`RuleRef.OriginHint`**, an explicit caller override. A caller that
   has already resolved a rule's citation via `knowledgeapi.ResolveCitation`
   should populate this from `CitationDTO.Origin` (see
   `knowledgeapi_test.go`'s `citationLookupFromKnowledgeAPI` for the
   composition pattern) — `packages/citation`'s own `Origin` classification,
   made at citation-resolution time from the underlying `CitedUnit`, is a
   strictly better signal than a text heuristic, since citation
   resolution may already know a rule's origin from a structured
   citation record (`packages/statute`/`packages/precedent` output).
2. **A lexical keyword heuristic** (`InferOrigin`, `origin.go`) over the
   rule's own `Text`: statute-shaped fragments (`§`, "U.S.C.", "C.F.R.",
   "enacted", "codified at", ...) versus case-shaped fragments (" v. ",
   reporter abbreviations like "F.3d", "the court held", ...). Statute
   wins on overlap, mirroring `evidenceweighing.ClassifyEvidenceKind`'s
   "more conservative/verifiable classification wins" convention.
3. **`OriginUnknown`**, if neither signal is present — treated as
   equivalent to `OriginPrecedent` by every `OriginProfile.Multiplier`
   (the more conservative default, matching `evidenceweighing.EvidenceKindUnknown`'s
   identical "default to the less volatile of the two categories"
   choice).

This is intentionally conservative and will misclassify a rule whose
text contains no recognizable keyword and whose caller supplied no hint.
`AttachCitations` also folds in whatever `Origin` a successful
`CitationLookupFunc` call returns (overriding the pre-lookup inference
when non-unknown), so a caller wiring a real `knowledgeapi.ResolveCitation`-backed
lookup gets the better signal automatically once citation resolution
runs — the text heuristic is only ever the fallback when a rule's
citation could not be resolved at all. The correct long-term fix is
surfacing `RuleNode.LegalFamily`/`Origin` (once tracked) through
`knowledgeapi.NodeDTO` directly, so this package can read ground truth
instead of inferring it — tracked here as an extension point, not
solved in this phase, exactly mirroring how Phase 053 tracked its own
testimony/documentary classification limitation.

## Rule-to-issue mapping

`MapIssueToControllingRules` (`map.go`) returns the sorted,
deduplicated union of:

- every `RuleID` linked to the issue via a `Rule--governs-->Issue` edge
  in the case's tree (`IssueInput.GoverningRuleIDs`, typically read from
  `knowledgeapi.GetTree`'s edge list), and
- every `RuleID` either party's arguments cite via `SupportingRuleIDs`
  for that issue.

A rule counts as controlling either because the tree structurally links
it there, or because a party actually invoked it in argument for that
issue even without (or ahead of) a governs edge — treating the union
rather than the intersection as controlling is deliberately
over-inclusive, so an upstream tree-assembly gap (a missing governs
edge) does not silently drop a rule a party is actually relying on.

## Element-to-fact mapping

`BuildElementFactMap` (`elements.go`) is deterministic bookkeeping, not
prose generation: for a given issue and controlling rule, it finds every
argument (from either party) that cites the rule, collects every fact
those arguments cite, and annotates each fact with its
`evidenceweighing.FactWeight` (weight, contradiction) and which parties
cited it. A fact cited without a matching `FactWeight` in the supplied
`evidenceweighing.Result` (e.g. an evidence-weighing run that predates a
later-added argument) is still recorded, with `FactWeight` 0 — a
defensively-handled gap, not a fatal error, mirroring
`evidenceweighing.DetectGaps`'s "surface, don't fail" convention.

## Weighting statute versus precedent by legal family

`WeightByOrigin` (`jurisdiction.go`) mirrors
`packages/application.WeightByLegalFamily`'s weighting concept exactly
(precedent favored under `common_law`, statute favored under
`civil_law`, neutral otherwise), reimplemented against this package's
own `Origin`/`RuleRef` types per the "Origin-inference limitation"
section above:

| `LegalFamily` | `OriginStatute` | `OriginPrecedent` | `OriginUnknown` |
|---|---|---|---|
| `common_law` | 0.8 | 1.0 | 1.0 |
| `civil_law` | 1.0 | 0.8 | 1.0 |
| anything else | 1.0 | 1.0 | 1.0 |

`CommonLawProfile`/`CivilLawProfile`/`NeutralProfile` construct an
`OriginProfile`; `ProfileForFamily` resolves one from a `LegalFamily`
string, exactly mirroring `evidenceweighing.JurisdictionProfile`'s
constructor/resolver pattern.

## Handling conflicting authority

`DetectConflictingAuthority` (`conflict.go`) flags a pair of controlling
rules for the same issue as a `ConflictingAuthority` finding when the
set of parties citing the first rule is non-empty, the set of parties
citing the second rule is non-empty, and the two sets are disjoint (no
party cites both). This package does **not** attempt to resolve which
rule prevails — Phase 055's synthesis agent, with the fuller context of
each argument's `Claim` text, is expected to make that call. This is a
deliberately conservative, party-based proxy for conflict, mirroring
`evidenceweighing.DetectContradictions`'s identical tradeoff: two rules
invoked by opposing parties are not necessarily legally inconsistent (a
party might cite a rule merely for background, or two rules might be
complementary), but opposing-party citation is the only stance proxy
available without semantic comparison of each argument's `Claim` text.

## Citing every applied authority

`AttachCitations` (`citations.go`) produces exactly one `AppliedCitation`
per `ControllingRuleIDs` entry, regardless of lookup outcome. A `nil`
`CitationLookupFunc`, a lookup error, or a lookup that resolves but
fails verification are all recorded (`Resolved`/`Verified` set
accordingly) rather than silently dropped or causing `Apply` to fail —
per the plan's explicit "cite every applied authority... track
unresolved/unverified citations as a quality signal" requirement.
`ComputeConfidence` folds citation health (the fraction resolved *and*
verified) into `IssueApplication.Confidence`, so an issue resting on
unverified authority scores lower without being rejected outright.

## Reasoning steps and confidence

Every `IssueApplication.Steps` entry is a human-readable record of one
reasoning move: how many controlling rules were found and from where,
how large the element-fact map is, whether a conflict was detected, how
many citations are unresolved/unverified, and the exact blend that
produced `Confidence` (`ComputeConfidence`, `confidence.go`). This
mirrors `evidenceweighing.FactWeight.Rationale`'s human-readable,
formula-transparent convention, but as a `[]Step` slice rather than one
string, since law application accumulates several distinct reasoning
moves per issue (mapping, aggregation, conflict detection, weighting)
rather than blending straight to one score.

`Confidence` blends three signals — average `WeightByOrigin` across
controlling rules (35%), citation health (35%), and average
`ElementFactEntry.FactWeight` (30%) — then applies a fractional penalty
(25% per `ConflictingAuthority`, capped at 75%) on top, mirroring
`evidenceweighing.WeightFactors.ContradictionPenalty`'s
penalty-on-top-of-blend convention exactly.

## Output and persistence

`Apply` (`apply.go`) ties rule mapping, element-fact aggregation,
conflict detection, citation attachment, and confidence scoring together
into one `Result` per case: `Result{CaseID, IssueApplications, GeneratedAt}`.
`Repository`/`InMemoryRepository` (`store.go`) persist that result
per-case, mirroring `evidenceweighing.Repository`'s upsert-by-key
convention exactly, so Phase 055 can retrieve a case's law-application
analysis without recomputing it.

## How this feeds Phase 055

Phase 055's synthesis agent is expected to consume, alongside both
parties' `ArgumentSet`s:

- **This package's `Result`** — "what does the law say": which rules
  control each issue, which weighed facts back their application, any
  conflicting authority to resolve, and each issue's law-application
  `Confidence`/`Steps` trail.
- **`packages/evidenceweighing`'s `Result`** — "how strong is the
  evidence": per-fact weights, contradictions, and gaps.

Together these give Phase 055 both halves of "weakest-link reasoning"
per the plan's Phase 055 goal: an issue whose controlling rule rests on
low-weight or contradicted facts (from `evidenceweighing`) *and* whose
own authority is unresolved, unverified, or in conflict (from this
package) should surface as a weaker basis for a tentative conclusion
than one resting on well-cited, uncontested rules applied to
high-weight, corroborated facts.

## What this package deliberately does not do

- It does not call an LLM anywhere in its analysis path (see "Design
  decision" above).
- It does not construct, mutate, or persist any `irac` tree node or
  edge — its `RuleRef`/`IssueInput` inputs are narrow projections, not
  ownership.
- It does not re-run `packages/evidenceweighing`'s scoring pipeline; it
  reads `FactWeight` values as-is.
- It does not resolve or verify citations itself (`packages/citation`'s
  and `packages/knowledgeapi`'s job) — `CitationLookupFunc` is a
  caller-supplied adapter, and this package's core logic never performs
  I/O.
- It does not decide which of two conflicting rules prevails, or predict
  a case outcome — `ConflictingAuthority` and `IssueApplication` are
  inputs to Phase 055's synthesis, not conclusions themselves.
- It does not classify a rule's `Origin` from ground truth when no
  `OriginHint`/citation-resolved origin is available — see
  "Origin-inference limitation" above for the accepted heuristic
  tradeoff.
- It does not semantically compare `Claim` text between opposing
  arguments — conflict detection is ID/party-based, not NLP-based, by
  design (see "Handling conflicting authority").
