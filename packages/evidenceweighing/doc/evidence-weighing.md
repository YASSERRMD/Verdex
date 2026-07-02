# Evidence-weighing module (`packages/evidenceweighing`)

Phase 053 sits between the adversarial argument agents
(`packages/firstpartyagent`, Phase 051; `packages/secondpartyagent`,
Phase 052) and the downstream law-application and synthesis stages
(Phase 054, Phase 055). Its job is to assess the reliability and relative
weight of competing evidence in a case: which facts are corroborated,
which are contradicted across the two parties' arguments, how strongly
each fact should count given the case's jurisdiction, and where the
evidentiary record has gaps.

## Design decision: heuristic, not LLM-backed

This package is deliberately **not** an `packages/agentframework` agent,
unlike Phases 050–052. The plan's language for this phase — "define
rubric," "score," "flag," "weight," "surface gaps," "output," "persist" —
matches the deterministic/heuristic scoring style of earlier packages:
`packages/fact`'s `ReliabilityScore` (Phase 034), `packages/application`'s
`WeightByLegalFamily` (Phase 037), and `packages/precedent`'s
`AuthorityScore` (Phase 037). All of this package's core logic —
`ScoreFact`, `DetectContradictions`, `DetectGaps`, `ClassifyEvidenceKind`
— is plain Go, unit-testable with in-memory fixtures and no model call.
This package does not import `packages/agentframework`, `packages/router`,
or any provider.

This was a deliberate choice over building it as an LLM agent:

- **Determinism and auditability.** A rubric-based score is reproducible
  and explainable by construction — every `FactWeight` carries a
  `Rationale` string derived directly from the coefficients applied, with
  no risk of a model inventing a justification after the fact.
- **The inputs are already structured.** By this stage, the only signals
  available are `irac.FactNode.Confidence` (a number) and how each
  party's `Argument.SupportingFactIDs` cites a fact (structured IDs, not
  prose). There is no unstructured text to interpret that would justify
  a model call — corroboration counting, contradiction flagging by
  shared fact ID + opposing party, and gap detection are all simple set
  operations over IDs already provided by upstream agents.
- **Consistency with sibling packages.** `packages/fact`,
  `packages/application`, and `packages/precedent` all score evidentiary/
  authority signals this same deterministic way; an LLM-backed
  evidence-weighing stage would be inconsistent with the pattern
  established for every other scoring module in the tree-reasoning
  pipeline.

The plan explicitly permits an optional narrow LLM sub-task (e.g.
semantic contradiction detection between how each party's arguments
characterize the same fact in prose). This package does not use one: its
`DetectContradictions` intentionally uses the coarser, fully-deterministic
proxy of "same fact ID + same issue + opposing party" rather than
semantic claim comparison — see "Contradiction detection" below for why
that tradeoff was accepted. A future phase could add a semantic
contradiction pass as an additive, optional LLM-backed enrichment without
changing this package's core scoring contract.

## Composes with, does not duplicate

| Package | Owns | This package's relationship |
|---|---|---|
| `packages/irac` | `FactNode`, `Node.Confidence`. | Read-only. This package never constructs, mutates, or persists a tree node — its `FactRef` input type is a narrow, caller-supplied projection of a `FactNode`'s ID/Text/Confidence, not a dependency on the full type. |
| `packages/knowledgeapi` | `GetTree`, `NodeDTO`. | The intended way a caller obtains the `[]FactRef` and issue node ID list passed into `Weigh` — this package itself never calls `knowledgeapi` directly, keeping its core logic free of I/O and fully unit-testable. |
| `packages/fact` (Phase 034) | `ReliabilityScore`, `DetermineDisputeStatus`, `CorroborationLink`/`DetectCorroboration` — computed at **ingestion time** from raw `evidence.Classification`/segment inputs. | **Not re-run.** Those inputs are not available at this reasoning stage; `packages/fact`'s ingestion-time reliability signal is only indirectly reflected via a `FactNode`'s `Confidence` field, which this package's `WeightFactors.BaseConfidenceWeight` folds in as one of three blended signals. This package computes its own, independent **reasoning-stage** corroboration/contradiction signal from how the two parties' `Argument`s cite facts — a different question from `packages/fact`'s ingestion-time cross-document corroboration. |
| `packages/firstpartyagent` (Phase 051) / `packages/secondpartyagent` (Phase 052) | `ArgumentSet` / `Argument` — each party's constructed case, including `SupportingFactIDs` and `Strength`. | **Primary input.** This package imports both only for their exported `ArgumentSet`/`Argument` types (never their internal agent/grounding logic) and converts each into its own party-agnostic `CitingArgument` view (see `types.go`), so `Weigh` never has to special-case which of the two (structurally near-identical, independently-typed) `Argument` shapes it is reading. |
| `packages/application` (Phase 037) | `WeightByLegalFamily` — an `OriginatedRule`-keyed legal-family multiplier. | Not imported. `jurisdiction.go`'s `JurisdictionProfile`/`ProfileForFamily` is a structurally parallel, independently-implemented equivalent for evidence (testimony vs documentary) rather than rules (statute vs precedent) — mirroring the same neutral-default-for-unrecognized-family convention rather than sharing code, since the two packages weight different axes. |
| `packages/jurisdiction` | The richer jurisdiction/legal-family domain model. | **Not imported.** `LegalFamily` is this package's own opaque `string` type, exactly mirroring `irac.RuleNode.LegalFamily`'s and `packages/application`'s own decoupling convention — a caller maps a richer jurisdiction concept onto `LegalFamily` at its own boundary. |
| `packages/evidence` (Phase 026) | `EvidenceType` — testimony/documentary/etc. classification at ingestion time. | **Not surfaced through `knowledgeapi` today** (see "Testimony vs documentary evidence" below) — this is a known limitation this package works around with a lexical heuristic rather than depending on. |
| `packages/citation` | `Repository`/`InMemoryRepository` per-case persistence convention. | Not imported, but `store.go`'s `Repository`/`InMemoryRepository` deliberately mirrors its shape (and `graph.InMemoryGraphStore`'s/`vectorindex.InMemoryVectorStore`'s) so the in-memory-store convention stays consistent across the monorepo. |

## The rubric

`WeightFactors` (see `rubric.go`) bundles four tunable coefficients,
defaulted by `DefaultWeightFactors()`:

| Coefficient | Default | Role |
|---|---|---|
| `BaseConfidenceWeight` | 0.45 | Weight on the fact's raw `Confidence` — the only signal fixed independently of which arguments cite it. |
| `CorroborationWeight` | 0.30 | Weight on how many distinct arguments (across both parties) cite the fact, normalized by `MaxCorroborationForScoring` (default 3). |
| `CitationStrengthWeight` | 0.25 | Weight on the average `Argument.Strength` across every argument citing the fact. |
| `ContradictionPenalty` | 0.5 | A *fractional* penalty (not a fourth blend weight) applied on top of the blended score when the fact is `Contradicted`. |

`ScoreFact` (see `score.go`) computes:

```
base   = BaseConfidenceWeight*confidence
       + CorroborationWeight*min(corroborationCount/MaxCorroborationForScoring, 1)
       + CitationStrengthWeight*avgCitingStrength

if contradicted:
    base = base * (1 - ContradictionPenalty)

weight = clamp01(base * jurisdictionProfile.Multiplier(evidenceKind))
```

The score is monotonic: for a fixed contradiction/jurisdiction state, more
corroboration or higher citing-argument strength never lowers the score
(see `score_test.go`'s `TestScoreFact_MonotonicInCorroborationAndStrength`).

A `Rubric` bundles `WeightFactors` with a `JurisdictionProfile`;
`DefaultRubric()` returns the default factors with the neutral profile.

## Corroboration and contradiction

- **Corroboration** (`CorroborationCounts`, `contradiction.go`): counts
  how many distinct arguments — from either or both parties — cite a
  fact in `SupportingFactIDs`. A fact cited more (even by the same party
  across multiple arguments, or by both parties toward compatible
  claims) is treated as more evidentially anchored, not resting on a
  single isolated assertion.
- **Contradiction detection** (`DetectContradictions`, `contradiction.go`):
  flags a fact as contradicted when two arguments share an
  `IssueNodeID`, come from different non-empty `PartyID`s, and both cite
  the same fact ID. This is a deliberately **conservative, over-inclusive**
  heuristic: `PartyID` (opposing sides on the same issue) is the only
  stance proxy available without semantic claim comparison, so it will
  flag some pairs that are not, on a full legal reading, actually in
  tension (e.g. both parties citing an undisputed fact but drawing
  different legal conclusions from it). Phase 055's synthesis agent is
  expected to review flagged `Contradiction`s with the fuller context of
  each argument's `Claim` text, not treat every flagged pair as
  necessarily a genuine factual dispute. A semantic (LLM-backed)
  contradiction pass, comparing how each side's `Claim` actually
  characterizes the fact, is the natural future extension point — see
  "Design decision" above.

## Jurisdiction weighting profiles

`JurisdictionProfile` (see `jurisdiction.go`) applies a per-`EvidenceKind`
multiplier on top of the blended base score:

| `LegalFamily` | Testimony | Documentary |
|---|---|---|
| `common_law` | 1.0 | 0.9 |
| `civil_law` | 0.8 | 1.0 |
| anything else | 1.0 | 1.0 |

`common_law` jurisdictions favor live testimony and cross-examination;
`civil_law` jurisdictions favor a documentary/written-record tradition.
Any unrecognized `LegalFamily` (including empty) is treated as neutral,
mirroring `packages/application`'s `WeightByLegalFamily` convention. A
zero-value `JurisdictionProfile{}` literal is also treated as neutral by
`Multiplier`, defensively, in case a caller constructs one without going
through `CommonLawProfile`/`CivilLawProfile`/`NeutralProfile`.

## Testimony vs documentary evidence (known limitation)

`packages/evidence`'s `EvidenceType` classification (Phase 026) — the
authoritative testimony/documentary/etc. distinction made at ingestion
time — is not surfaced through `knowledgeapi.NodeDTO` today; a `FactRef`
carries only `ID`, `Text`, and `Confidence`. `ClassifyEvidenceKind`
(`classify.go`) works around this with a lexical keyword heuristic over
the fact's own text (e.g. "testified," "witness" → testimony; "contract,"
"invoice," "exhibit" → documentary; documentary wins on overlap; neither
→ `EvidenceKindUnknown`, treated as documentary for weighting purposes).
This is intentionally conservative and will misclassify facts whose text
doesn't contain a recognizable keyword. The correct long-term fix is
threading `evidence.Classification`'s `EvidenceType` through
`knowledgeapi.NodeDTO` (or an equivalent) so this package (and any
future consumer) can classify from ground truth instead of a heuristic —
tracked here as an extension point, not solved in this phase.

## Gap surfacing

`DetectGaps` (`gaps.go`) flags two kinds of evidentiary-record defects:

- `GapKindMissingFact`: an argument's `SupportingFactIDs` references a
  fact ID that does not resolve to any `FactRef` supplied to `Weigh`.
  Each argument agent's own anti-fabrication grounding step
  (`firstpartyagent/ground.go`, `secondpartyagent/ground.go`) should
  already prevent this from occurring, but this package checks it
  defensively rather than trusting that invariant holds across every
  caller and code path.
- `GapKindUncitedIssue`: an issue (from the caller-supplied
  `IssueNodeIDs` list) that no argument, from either party, cites even
  one fact for — including an issue that received an argument with zero
  `SupportingFactIDs`.

## Output and persistence

`ScoreFacts` produces one `FactWeight` per fact:

```go
type FactWeight struct {
    FactNodeID         string
    Weight             float64
    Kind               EvidenceKind
    Contradicted       bool
    CorroborationCount int
    Rationale          string
}
```

`Rationale` is a human-readable string (not a structured audit log,
consistent with `packages/precedent`'s authority-scoring convention of a
computed value plus a documented formula) recording exactly which
signals and coefficients produced the score — e.g.:

```
base score 0.612 (confidence=0.80*w0.45 + corroboration=0.67*w0.30 + citation_strength=0.60*w0.25); contradiction penalty 50% applied -> 0.306; jurisdiction profile "common_law" applies 0.90x multiplier for documentary evidence -> 0.275
```

`Weigh` (`weigh.go`) ties fact scoring, contradiction detection, and gap
surfacing together into one `EvidenceWeighingResult` per case.
`Repository`/`InMemoryRepository` (`store.go`) persist that result
per-case, mirroring `citation.Repository`'s upsert-by-key convention, so
Phase 054 and Phase 055 can retrieve a case's evidence weights without
recomputing them.

## How this feeds Phase 054 and Phase 055

- **Phase 054 (law-application module)** applies governing rules to
  *weighed* facts per issue. It is expected to read `FactWeight.Weight`
  (and `Contradicted`/`Rationale`) via this package's `Repository` rather
  than raw `FactNode.Confidence`, so a fact's evidentiary standing —
  corroborated, contradicted, jurisdiction-adjusted — feeds directly into
  how strongly a legal test is treated as satisfied.
- **Phase 055 (synthesis agent)** is expected to consume the full
  `EvidenceWeighingResult` — `FactWeights`, `Contradictions`, and `Gaps`
  — alongside both parties' `ArgumentSet`s and Phase 054's law-application
  output, to produce "weakest-link reasoning" (per the plan's Phase 055
  goal): an issue resting on a low-weight, contradicted, or gapped fact
  should surface as a weaker basis for a tentative conclusion than one
  resting on high-weight, corroborated, undisputed facts.

## What this package deliberately does not do

- It does not call an LLM anywhere in its scoring path (see "Design
  decision" above).
- It does not construct, mutate, or persist any `irac` tree node or
  edge — its `FactRef` input is a narrow projection, not ownership.
- It does not re-run `packages/fact`'s ingestion-time reliability/
  corroboration/dispute pipeline; it computes an independent,
  reasoning-stage signal from how arguments cite facts.
- It does not resolve or verify citations (`packages/citation`'s job);
  `CitingArgument.Strength` is read as-is from each `Argument`, not
  recomputed here.
- It does not semantically compare `Claim` text between opposing
  arguments — contradiction detection is ID/party-based, not NLP-based,
  by design (see "Corroboration and contradiction").
- It does not call `knowledgeapi` or any other I/O boundary itself —
  callers are expected to fetch `FactRef`s and `IssueNodeIDs` via
  `knowledgeapi.GetTree` and pass them in, keeping this package's core
  logic pure and easy to unit test.
- It does not decide case outcomes or produce verdict/directive
  language — `FactWeight` and `Contradiction` are inputs to Phase 054/055's
  reasoning, not conclusions themselves.
