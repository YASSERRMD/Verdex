# Synthesis agent (`packages/synthesisagent`)

Phase 055 is the capstone reasoning agent of Part 5: it weighs both
parties' constructed arguments (`packages/firstpartyagent`, Phase 051;
`packages/secondpartyagent`, Phase 052), the case's weighed evidence
(`packages/evidenceweighing`, Phase 053), and its applied law
(`packages/lawapplication`, Phase 054), resolved per issue
(`packages/issueagent`, Phase 050), into a single draft, non-binding
reasoned opinion.

## An LLM agent, unlike Phases 053-054

Where `packages/evidenceweighing` and `packages/lawapplication` are
deliberately deterministic, heuristic bookkeeping modules with no model
call, this package **is** an `packages/agentframework` agent, per the
plan's own Phase 055 title ("Synthesis & reasoned-opinion agent") and its
"resolve each issue with reasoning" goal. Synthesizing a coherent, prose
explanation of how competing arguments, evidentiary weight, and applied
law together resolve an issue is exactly the natural-language-generation
task `packages/lawapplication`'s own doc explicitly deferred to this
phase (see its "Design decision" section: "That natural-language
synthesis is deliberately left to Phase 055's synthesis agent, which has
the fuller context ... to write it well").

`Agent` implements `agentframework.Agent` and is driven end-to-end by
`agentframework.Runner` via the `Synthesize` convenience entrypoint,
mirroring `firstpartyagent.Argue`'s and `issueagent.Analyze`'s
single-step, "gather everything up front, one model call, conclude"
design exactly. Every model call is dispatched through
`packages/router` — this package never hardcodes a provider.

## Composes with, does not duplicate

| Package | Owns | This package's relationship |
|---|---|---|
| `packages/issueagent` (Phase 050) | `FramedIssue`, `IssueAnalysisResult`. | **Primary input.** `New`'s `issues []issueagent.FramedIssue` parameter is the exact set of issues this agent resolves, one `TentativeConclusion` per issue. |
| `packages/firstpartyagent` (Phase 051) / `packages/secondpartyagent` (Phase 052) | `ArgumentSet`/`Argument`. | **Primary input**, passed to `New` directly (not converted into a party-agnostic view like `lawapplication.ArgumentRef`/`evidenceweighing.CitingArgument`, since this package needs each `Argument`'s full `Claim` text for prompt rendering, not just its ID/fact/rule references). Either `ArgumentSet` may be its zero value if that party produced no arguments. |
| `packages/evidenceweighing` (Phase 053) | `Result`, `FactWeight`. | **Primary input.** Read-only: `FactWeight.Weight`/`Contradicted` feed both the per-issue prompt section and weakest-link derivation (`weakestlink.go`). This package never recomputes evidence weights. |
| `packages/lawapplication` (Phase 054) | `Result`, `IssueApplication`. | **Primary input.** Read-only: `ControllingRuleIDs`, `Conflicts`, and `Confidence` feed both the prompt and weakest-link derivation. This package never re-derives controlling rules or conflicting authority — it reads `lawapplication`'s findings as given. |
| `packages/knowledgeapi` | `GetTree`, `NodeDTO`. | This package's only access path to the case's actual tree, used to resolve the real fact/rule nodes either party's arguments (or law application's controlling rules) reference, both for prompt rendering (`fetch.go`) and as the ground-truth set the model's proposed citations are checked against (`ground.go`) — mirroring `firstpartyagent`'s/`secondpartyagent`'s identical anti-fabrication convention. |
| `packages/agentframework` (Phase 049) | `Agent`, `Runner`, `Scratchpad`, `Decision`. | This package's `Agent` implements the `agentframework.Agent` interface; `Synthesize` wraps `agentframework.NewRunner`/`Runner.Run`, mirroring `firstpartyagent.Argue` exactly. |
| `packages/prompts` | `PromptTemplate`, `Registry`, `VariantSelector`. | `templates/synthesis.go` registers this package's one template (`synthesisagent.opinion.synthesis`) into `prompts.DefaultRegistry` with `NonBindingLabel: true`, mirroring `firstpartyagent/templates/argument_construction.go`'s registration pattern exactly. |
| `packages/irac` | `ConclusionNode`, `NewConclusionNode`, `ContainsVerdictLanguage`, `DraftAnalysisLabel`. | `provider.go`'s `Provider` is the only place in this package that constructs tree nodes, and it does so exclusively via `irac.NewConclusionNode` — see "The ConclusionProvider wiring" below. |
| `packages/treeassembly` (Phase 039) | `ConclusionProvider`, `ComposeTree`, `AssemblyInput`. | **This is the phase treeassembly's own `doc.go` has been waiting for since Phase 039.** See below. |

## Resolving each issue

For every `FramedIssue` supplied to `New`, `fetchSynthesisInputs`
(`fetch.go`) gathers:

- both parties' `Argument`s whose `IssueNodeID` matches the issue (by
  grouping each `ArgumentSet` on that field — no cross-referencing
  against `packages/lawapplication`'s `ArgumentRef` conversion is needed
  here, since this package keeps the original `Argument` types for their
  `Claim` text);
- the issue's `lawapplication.IssueApplication`, if one exists;
- the union of fact/rule node IDs either party's arguments cited plus
  `IssueApplication.ControllingRuleIDs`, resolved against the case's
  actual tree via one `knowledgeapi.GetTree` call (shared across every
  issue, mirroring `firstpartyagent.fetchIssueEvidence`'s single-tree-
  fetch convention); and
- the `evidenceweighing.FactWeight` for each resolved fact.

`prompt.go`'s `renderIssuesBlock` turns this into one prompt section per
issue: governing question(s), both parties' claims, controlling rules and
any detected conflicting authority, and every citable fact (with its
weight and contradiction status) and rule. The model is instructed to
cite only fact/rule IDs appearing in this list — exactly the same
"exhaustive, explicit allow-list" convention `firstpartyagent`'s and
`secondpartyagent`'s prompts use.

## Tracing every conclusion to fact and rule nodes

`ground.go`'s `groundConclusion` cross-checks every ID the model proposed
in a conclusion's `supporting_fact_ids`/`supporting_rule_ids` against the
exact per-issue allow-list `fetchSynthesisInputs` resolved from the case's
actual tree — the same anti-fabrication pattern `firstpartyagent/ground.go`
and `secondpartyagent/ground.go` established. A fabricated ID is stripped,
not trusted, and recorded in `TentativeConclusion.FabricatedNodeIDs` with
`Grounded` set to `false`. A conclusion left with zero real supporting IDs
after stripping is dropped entirely; its issue is recorded in
`Opinion.SkippedIssueNodeIDs` rather than failing the whole run, mirroring
`firstpartyagent.ArgumentSet.SkippedIssueNodeIDs`'s per-issue, non-fatal
convention exactly.

## Weakest-link reasoning

`weakestlink.go`'s `deriveWeakestLink` surfaces, per conclusion, the
single supporting element most likely to undermine it, in priority order:

1. **A fabricated reference**, if grounding stripped one — the strongest
   possible weak-link signal, since the model cited something that does
   not exist.
2. **The lowest-weight or contradicted fact** among the conclusion's
   surviving `SupportingFactIDs`, per `evidenceweighing.FactWeight`
   (a `Contradicted` fact always outranks a merely low-weight one).
3. **A `lawapplication.ConflictingAuthority`** affecting the issue's
   controlling rules, if evidence-level signals found nothing.
4. **The model's own self-reported `weakest_link` text**, used verbatim
   as a last resort — it is unverified free-text commentary, not a
   fabrication signal in itself, so it is trusted only once the more
   concrete, tree-verifiable signals above have been checked and found
   nothing.

This mirrors Phase 054's own framing of "how this feeds Phase 055":
an issue resting on low-weight/contradicted facts *and* unresolved or
conflicting authority should surface as a weaker basis for a tentative
conclusion than one resting on well-cited, uncontested rules applied to
high-weight, corroborated facts. `deriveWeakestLink` is that promise,
implemented.

## The `ConclusionProvider` wiring into `treeassembly`

`packages/treeassembly` has, since Phase 039, defined a
`ConclusionProvider` interface and shipped `NoOpConclusionProvider` as its
only implementation, with `compose.go` documenting explicitly: "once
Phase 055 lands, it need only implement `ConclusionProvider` and be wired
in here (or by a caller), with no change required to this package's
composition, validation, gap-detection, revision, or persistence logic."

**This package is that Phase 055.** `provider.go`'s `Provider` type wraps
an already-synthesized `Opinion` and implements
`treeassembly.ConclusionProvider`:

```go
opinion, _, err := synthesisagent.Synthesize(ctx, agent, caseID, cfg)
// ...
tree, err := treeassembly.ComposeTree(ctx, assemblyInput, synthesisagent.Provider{Opinion: opinion})
```

`Provider.Provide` converts each `TentativeConclusion` into exactly one
`irac.ConclusionNode`, deriving `Provenance.UpstreamNodeIDs` by matching
the conclusion's `SupportingFactIDs`/`SupportingRuleIDs` against which
`ApplicationNode`s in the assembly input reasoned about those same
fact/rule references — reconstructing the
`Conclusion --concludes_from--> Application` edge
`treeassembly.ComposeTree`'s own `deriveEdges` expects, per
`packages/irac/edge.go`'s legal-triple table. A caller assembling a case's
full tree now passes `synthesisagent.Provider{Opinion: opinion}` in place
of `treeassembly.NoOpConclusionProvider{}`, with no other change to how
`ComposeTree` is called.

## The non-binding guardrail, enforced at the output boundary

Every `irac.ConclusionNode` `Provider` emits is built exclusively via
`irac.NewConclusionNode`, which unconditionally attaches the mandatory
`draft_analysis` label (`irac.DraftAnalysisLabel`) — there is no other
constructor path, so it is not possible for this package to emit a
`ConclusionNode` missing the guardrail label.

On top of that structural guarantee, `Provider.Provide` additionally
checks every `TentativeConclusion.Text` against
`irac.ContainsVerdictLanguage` before it ever reaches
`irac.NewConclusionNode`. A conclusion whose text contains verdict or
directive language (e.g. "guilty", "is ordered", "shall pay") is
**rejected**: it is logged and excluded from the returned node slice
rather than becoming a `ConclusionNode` at all. `synthesisagent.templates`
also registers its one template with `NonBindingLabel: true`, so
`prompts.Render` appends the standard AI-generated-analysis disclaimer to
every rendered prompt, and the prompt body itself instructs the model to
use hedged, analytical language and never verdict/directive phrasing.

This is deliberately narrow: full guardrail **policy** enforcement —
blocking an entire run because one conclusion tripped the check, gating
human review, or auditing rejected conclusions — is Phase 057's job. This
package's job is narrower and non-negotiable: it must never itself
construct a `ConclusionNode` carrying verdict-flavored text or missing the
`draft_analysis` label, and it meets that bar at the exact boundary where
an `Opinion` becomes tree nodes.

## Output shape

`Opinion` (not `SynthesisResult` — avoiding the stutter against this
package's own name `synthesisagent`, mirroring how `lawapplication.Result`
and `evidenceweighing.Result` avoid stuttering against theirs) is the
top-level structured output of one synthesis run:

```go
type Opinion struct {
    CaseID              string
    Conclusions         []TentativeConclusion
    SkippedIssueNodeIDs []string
    GeneratedAt         time.Time
}

type TentativeConclusion struct {
    IssueNodeID       string
    Text              string
    FavoredParty      string // empty when genuinely unresolved
    Confidence        float64
    WeakestLink       string
    SupportingFactIDs []string
    SupportingRuleIDs []string
    Grounded          bool
    FabricatedNodeIDs []string
}
```

`FavoredParty` being empty is a legitimate, honestly-reported outcome —
"the record does not yet clearly favor either party" — not an error or a
sign of a failed run.

## What this package deliberately does not do

- It does not itself decide a case's outcome or issue a verdict —
  `TentativeConclusion` is a draft analysis, and `FavoredParty` names
  which party's position the current record favors, not a binding
  determination.
- It does not re-derive controlling rules, conflicting authority, or fact
  weights — it reads `lawapplication.Result` and `evidenceweighing.Result`
  as given, exactly as `lawapplication` itself reads
  `evidenceweighing.Result` without recomputing it.
- It does not enforce guardrail *policy* (blocking runs, human-review
  gating, audit trails for rejected conclusions) — only the narrow,
  structural non-binding guarantee described above. Full policy
  enforcement is Phase 057's job.
- It does not construct, mutate, or persist any `irac` tree node itself
  outside of `Provider.Provide`'s `ConclusionNode` conversion — it never
  touches `packages/graph` or `packages/treeindex` directly, only
  `packages/knowledgeapi`.
- It does not resolve or verify citations itself — `SupportingRuleIDs`
  trace back to the case's actual rule nodes, but citation
  resolution/verification remains `packages/citation`'s and
  `packages/knowledgeapi`'s job, already performed upstream by
  `lawapplication.AttachCitations`.
- It does not persist `Opinion` itself — mirroring
  `firstpartyagent`/`secondpartyagent`'s convention of leaving
  persistence to a caller (or a future orchestration phase), this package
  only produces the typed result and the `ConclusionProvider` adapter.
