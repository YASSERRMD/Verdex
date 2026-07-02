# Reasoning trace (`packages/reasoningtrace`)

Phase 060's goal, verbatim from the implementation plan:

> Produce a fully auditable reasoning trace.

By Phase 059, `packages/reasoningorchestration` could run a case through
every reasoning stage — issue framing, first- and second-party
arguments, evidence weighing, law application, synthesis, uncertainty
surfacing, and the guardrail check — and checkpoint each stage's typed
output. What it did not do was preserve *how* the four LLM-backed stages
got there: every model call, tool call, and observation an agent made
was computed, held briefly on an `agentframework.Scratchpad`, and then
discarded the moment `issueagent.Analyze` (and its three siblings)
returned. This package is Part 5's final layer: it makes that discarded
detail permanent, and turns it into something a human reviewer — not
just another program — can read.

## Composes with, does not duplicate

`reasoningtrace` never re-derives or second-guesses a conclusion. Every
fact it reports was already computed by an upstream package:

- `packages/reasoningorchestration` — the checkpoint store this package
  reads from, and the four new `Checkpoint` fields (see below) that make
  tracing possible at all.
- `packages/agentframework` — the `Result`/`Scratchpad`/`Step`/
  `Observation` shapes this package flattens into `StageStep` and
  `RetrievalEvent`.
- `packages/synthesisagent` — `Opinion`/`TentativeConclusion`, the
  source of every `AuthorityTrail`'s `IssueNodeID` and
  `SupportingFactIDs`.
- `packages/lawapplication` — `Result`/`IssueApplication`/
  `AppliedCitation`, the source of every `AuthorityTrail`'s controlling
  rules and citation verification status.
- `packages/uncertainty` and `packages/guardrail` — narrated, not
  re-evaluated: the narrative reports what these stages found, it does
  not recompute it.
- `packages/identity` — `RequireViewPermission` mirrors
  `packages/knowledgeapi`'s own authorize-then-proceed pattern exactly,
  gating on the same `identity.PermViewCase` permission rather than
  inventing a new one.

## The Phase 059 gap this phase closes: `Checkpoint`'s new fields

Before this phase, `reasoningorchestration.Checkpoint` stored only each
stage's typed domain result (`IssueAnalysis`, `FirstPartyArguments`,
`SecondPartyArguments`, `Opinion`, ...). The four LLM-backed stages'
entrypoints (`issueagent.Analyze`, `firstpartyagent.Argue`,
`secondpartyagent.Argue`, `synthesisagent.Synthesize`) always returned a
second value — the full `agentframework.Result`, including the
`Scratchpad` of every step, tool call, and observation — but every call
site discarded it with `_`.

Phase 060 added four fields to `Checkpoint` to fix this:

```go
type Checkpoint struct {
    // ... existing typed-result fields unchanged ...
    IssueFramingRun agentframework.Result // populated when Stage == StageIssueFraming
    FirstPartyRun   agentframework.Result // populated when Stage == StageFirstPartyArguments
    SecondPartyRun  agentframework.Result // populated when Stage == StageSecondPartyArguments
    SynthesisRun    agentframework.Result // populated when Stage == StageSynthesis
}
```

This follows the same "one struct, many optional fields" convention the
rest of `Checkpoint` already used: exactly one of the four `*Run` fields
is meaningful for a given `Stage`, the others are zero. `stages.go`'s
four `run*` functions were updated to capture the previously-discarded
second return value and set it on the `Checkpoint` they already build
and save — no new persistence path, no change to `CheckpointStore`,
no change to `Run`/`Resume`'s control flow.

## The `Trace` shape

`Build(ctx, caseID, store)` reads back `reasoningorchestration.RunState`
to discover which stages completed, reads each completed stage's
`Checkpoint`, and assembles:

- **`Steps []StageStep`** — one entry per `agentframework.Step` across
  every LLM-backed stage's `Scratchpad`, tagged with the `Stage` it came
  from. Answers "what did the pipeline actually do."
- **`Retrievals []RetrievalEvent`** — one entry per tool call whose name
  is a `knowledgeapi` read tool (`search_case_knowledge`, `get_node`,
  `lookup_paths`, `resolve_citation`, `validation_status`), extracted
  from the same Scratchpads. Answers "what evidence did it look at."
- **`Narrative string` / `Segments []NarrativeSegment`** — a flat,
  human-readable prose walk through every completed stage. `Build`
  computes one `NarrativeSegment` per completed stage (via the
  unexported `narrateStage`) and joins their `Text` into the flat
  `Narrative` string (via `renderNarrative`); both are populated
  directly on the returned `Trace` rather than requiring a second call.
  Each segment carries the `IssueNodeID`/`SupportingFactIDs`/
  `SupportingRuleIDs` it discusses so a UI can render "explain this
  sentence" links back into the IRAC tree without parsing prose.
- **`AuthorityTrails []AuthorityTrail`** — one per conclusion in the
  synthesized `Opinion`: `IssueNodeID` → `SupportingFactIDs` →
  `CitationTrail` (controlling rule ID, resolved citation text,
  `Resolved`/`Verified` flags). A future UI renders this as a
  collapsible expand/collapse tree per the plan's "expandable
  evidence/authority trail" requirement.

## Export formats

- **`ExportJSON(trace) ([]byte, error)`** — indented JSON, the full
  `Trace` struct. Round-trips via `encoding/json` (see
  `TestExportJSON_RoundTrips`).
- **`ExportMarkdown(trace) (string, error)`** — a human-readable
  document with `## Narrative`, `## Steps`, `## Retrieved nodes and
  citations`, and `## Authority trails` sections, suitable for pasting
  into a case file or a PR description.

## Access control

`RequireViewPermission(ctx)` requires ctx to carry an authenticated
`identity.User` holding `identity.PermViewCase` — the same permission
`packages/knowledgeapi` gates every read method on. `Build` calls this
*first*, before a single `CheckpointStore` read, so an unauthorized
caller never even triggers a store lookup. This is a deliberate
mirror of `knowledgeapi`'s `authorize`-then-proceed pattern, not a new
authorization model: a `Trace` exposes every model call, tool call,
citation, and draft conclusion the pipeline produced, which is at least
as sensitive as the case knowledge `knowledgeapi` already protects.

## Integrity hash: guarantee and limits

`IntegrityHash(trace) string` computes a SHA-256 hash (hex-encoded) over
a canonical JSON view of `trace` — every `error`-typed field is first
rendered as its message string, since `error` has no `MarshalJSON` and
would otherwise marshal unpredictably. `VerifyIntegrity(trace,
expectedHash) bool` recomputes the hash and compares.

This is a **tamper-evidence** mechanism, in the same spirit as
`packages/provenance`'s content-hash concept (see its `doc/
custody-model.md`): it detects that a *stored* `Trace` value was
mutated after the hash was recorded, because any change to any field
changes the hash. It is explicitly **not**:

- a cryptographic signature chain (no private key, no non-repudiation —
  anyone who can compute SHA-256 can recompute a new "valid" hash for a
  tampered `Trace`; this only helps when the *original* hash was
  recorded and preserved somewhere the tamperer cannot also rewrite);
- proof of who produced the original value or when (no timestamp
  authority, no signer identity — contrast with `provenance`'s
  HMAC-SHA256-signed, chain-linked records, which this package
  deliberately does not import or reuse, since a reasoning trace has a
  different threat model: proving a stored explanation was not quietly
  edited after review, not proving custody of an uploaded artifact);
- a substitute for `CheckpointStore`'s own durability guarantees — if
  the underlying checkpoints are lost or corrupted, `Build` simply fails
  or produces an incomplete `Trace`; `IntegrityHash` only ever speaks to
  a `Trace` value already in hand.

## What this package deliberately does not do

- It does not re-run, re-validate, or second-guess any stage's
  conclusion — it is purely an after-the-fact explanation layer over
  data every upstream package already computed and checkpointed.
- It does not persist a `Trace` or its `IntegrityHash` anywhere; `Build`
  is a pure read-and-assemble function, and a caller wanting durable
  storage of a `Trace` (or its hash, for later `VerifyIntegrity` calls)
  is responsible for that itself.
- It does not trace the four deterministic, non-LLM stages (evidence
  weighing, law application, uncertainty surfacing, guardrail check) at
  the step/tool-call level, since none of them make model or tool calls
  — they are narrated in prose from their typed `Checkpoint` output
  instead, exactly like the LLM-backed stages' narrative sentences are.
- It does not implement a UI. `NarrativeSegment.RelatedNodeIDs` and
  `AuthorityTrail`'s nested shape are designed to be consumed by a
  future case-workspace UI's expand/collapse tree (Phases 64-67), but no
  such UI exists in this package.
- It does not add a new authorization model — `RequireViewPermission`
  reuses `identity.PermViewCase` exactly as `knowledgeapi` defines it.

## Closing out Part 5

With this phase, every package in Part 5 (Reasoning & Adversarial
Synthesis, Phases 49-60) is complete: `issueagent` frames the issues,
`firstpartyagent`/`secondpartyagent` argue both sides, `evidenceweighing`
scores the record, `lawapplication` maps controlling authority,
`synthesisagent` drafts tentative conclusions, `uncertainty` surfaces
every reason to doubt them, `guardrail` enforces the non-binding,
sign-off-gated finalization boundary, `reasoningorchestration`
coordinates all of it into one auditable run per case, and
`reasoningtrace` — this package — is the layer that makes the entire
run explainable to a human reviewer after the fact.
