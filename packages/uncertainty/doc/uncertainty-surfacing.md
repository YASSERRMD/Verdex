# Uncertainty surfacing (`packages/uncertainty`)

Phase 056 sits on top of the full tree-reasoning pipeline for a case:
`packages/issueagent` (Phase 050), `packages/evidenceweighing`
(Phase 053), `packages/lawapplication` (Phase 054), and
`packages/synthesisagent` (Phase 055). Its job is to make that combined
output honest about where it is weak — identifying, ranking, and
explaining every reason a reviewer should not take the draft opinion at
face value, and flagging conclusion text that overstates its own
confidence.

## Design decision: heuristic, not LLM-backed

Like `packages/evidenceweighing` and `packages/lawapplication`, this
package is a deterministic, heuristic scoring and text-scanning module —
plain Go, unit-testable with in-memory fixtures, no model call. The
plan's language for this phase — "identify," "rank," "generate,"
"attach," "output" — matches that style, not the LLM-agent style of
Phases 050–052/055. This package does not import
`packages/agentframework`, `packages/router`, or any provider. Everything
it needs (confidence scores, contradiction/gap records, conflicting-
authority records, conclusion text) is already structured data emitted
by upstream packages; there is no unstructured interpretation step that
would justify a model call.

## Composes with, does not duplicate

| Package | Owns | This package's relationship |
|---|---|---|
| `packages/issueagent` | `FramedIssue.Confidence`, `MaterialityRank`. | Read-only. Supplies both a low-confidence signal (`SourceIssueFraming`) and the materiality context every finding's impact is ranked against. |
| `packages/evidenceweighing` | `FactWeight`, `Contradiction`, `Gap`. | Read-only. Supplies thin/contradicted-evidence findings (`SourceEvidence`). |
| `packages/lawapplication` | `IssueApplication.Confidence`, `ConflictingAuthority`. | Read-only. Supplies low-confidence and unsettled/conflicting-law findings (`SourceLawApplication`). |
| `packages/synthesisagent` | `TentativeConclusion.Confidence`, `.Text`, `.WeakestLink`. | Read-only. Supplies low-confidence conclusion findings (`SourceConclusion`) and is the text scanned for over-confident phrasing. **Never mutated.** |

### Distinct from `synthesisagent`'s own `WeakestLink`

`synthesisagent.TentativeConclusion.WeakestLink` is a single string,
computed per conclusion, naming that one conclusion's single weakest
supporting element. It is produced as a side effect of synthesis itself,
scoped to one issue at a time, with no visibility into the rest of the
case.

This package is not a reimplementation of that idea and does not read or
recompute `WeakestLink`. It is a separate, cross-cutting pass that runs
*after* synthesis, over *all four* upstream result types *together*, and
does more than name one weak point:

- **Multiple, distinct findings per issue**, not one string: an issue can
  simultaneously carry a low-confidence framing finding, a thin-evidence
  finding, a conflicting-authority finding, and a low-confidence
  law-application finding — see `identify.go`.
- **Outcome-level ranking, not per-conclusion**: every finding across
  the whole case is ranked by `ImpactRank`, combining the finding's own
  `Severity` with its issue's `issueagent.FramedIssue.MaterialityRank` —
  see "Ranking algorithm" below. `WeakestLink` has no equivalent; it does
  not know how material its own issue is relative to the rest of the
  case.
- **Explicit, generated `Caveat` text**, worded for a reviewer, not an
  internal debugging label — see "Caveat generation" below.
- **A conclusion-text quality scan** (`OverconfidencePhrasing`) that
  `WeakestLink` has no equivalent of at all: `WeakestLink` says nothing
  about how a conclusion is *worded*, only what it relies on.

A caller wanting "the single weakest point of conclusion X" should keep
reading `TentativeConclusion.WeakestLink`. A caller wanting "rank every
doubt across this whole case by how much it matters, and tell me if any
conclusion is over-stating its confidence in its own wording" calls this
package's `Surface`.

## Primary types

```go
type Source string

const (
    SourceIssueFraming    Source = "issue_framing"
    SourceEvidence        Source = "evidence"
    SourceLawApplication  Source = "law_application"
    SourceConclusion      Source = "conclusion"
)

type Uncertainty struct {
    IssueNodeID string
    Source      Source
    Severity    float64
    ImpactRank  int
    ImpactScore float64
    Caveat      string
    Detail      string
}

type OverconfidencePhrasing struct {
    IssueNodeID string
    Phrase      string
    Excerpt     string
}

type Report struct {
    CaseID              string
    Uncertainties       []Uncertainty
    OverconfidenceFlags []OverconfidencePhrasing
    GeneratedAt         time.Time
}

func Surface(req Request) (Report, error)
func Analyze(req Request) (Report, error) // alias
func (r Report) ByIssue() map[string][]Uncertainty
```

`Request` bundles `issueagent.IssueAnalysisResult`,
`evidenceweighing.Result`, `lawapplication.Result`, and
`synthesisagent.Opinion` for one case, plus optional threshold overrides.

## Identifying uncertainty (three sources, four kinds of finding)

`identify.go` runs three independent scans over `Request`:

1. **Low-confidence reasoning steps** (`identifyLowConfidence`): scans
   every `FramedIssue.Confidence`, `IssueApplication.Confidence`, and
   `TentativeConclusion.Confidence` against a configurable threshold
   (`Request.LowConfidenceThreshold`, default `0.5`), tagging each
   finding with the `Source` it came from — three separate scans rather
   than one generic pass, because each source's confidence measures a
   different thing and earns its own `Caveat` wording.
2. **Thin or disputed evidence** (`identifyThinEvidence`): surfaces every
   `FactWeight` that is `Contradicted` or falls at or below
   `Request.ThinEvidenceWeightThreshold` (default `0.4`), every
   `Contradiction`, and every `Gap`. A `FactWeight` finding (which
   carries no `IssueNodeID` of its own) is attached to every issue that
   cites it, resolved via the law application's `ElementFactMap` — the
   only upstream structure that already links a fact to an issue.
3. **Unsettled or conflicting law** (`identifyConflictingLaw`): surfaces
   every `ConflictingAuthority` across every `IssueApplication.Conflicts`,
   as a distinct, fixed-severity structural finding — separate from (and
   additional to) that issue's own low-confidence finding, if any.

Every finding above carries a fixed or confidence-derived `Severity` in
`[0,1]` (see `severity.go`): a `Contradiction` and a contradicted
`FactWeight` are both pinned at `0.7`; a `Gap` at `0.6`; a
`ConflictingAuthority` at `0.8`; a low-confidence finding's severity is
`1 - confidence`; a merely-thin (not contradicted) `FactWeight`'s
severity is `1 - weight`.

## Ranking algorithm

`rank.go`'s `rankUncertainties` computes each finding's `ImpactScore` as:

```
ImpactScore = Severity * materialityAmplification(MaterialityRank, totalIssues)
```

`materialityAmplification` linearly decays from `1.0` at the case's most
material issue (`MaterialityRank == 1`) to a floor of `0.5` at its least
material issue, so an uncertainty on a peripheral issue is never zeroed
out — it is still worth surfacing, just ranked below an equally severe
finding on a more material issue. A finding with no resolvable issue
(`MaterialityRank` unknown) or a single-issue case gets the full `1.0`
multiplier, since there is no materiality context to discount by.

Findings are then sorted by descending `ImpactScore` and assigned
`ImpactRank` `1..n`; ties are broken deterministically by `Source`, then
`IssueNodeID`, then `Detail`, so repeated runs against identical input
always produce identical output.

## Caveat generation

`caveat.go` generates one `Caveat` string per finding, worded for direct
display to a reviewing judge rather than as an internal label — e.g.:

- *"This conclusion relies on a fact contradicted by opposing evidence."*
- *"The controlling authority for this issue is contested between two
  conflicting rules (rule-negligence and rule-strict-liability), invoked
  by opposing parties."*
- *"This issue is argued without any party citing supporting evidence
  for it."*

Caveat text is templated from the finding's `Source`/kind and its
concrete identifiers (fact/rule IDs, confidence values) — not model-
generated, so it is reproducible and requires no additional review for
hallucination.

## Attaching uncertainty to conclusions

`Report.ByIssue()` groups `Uncertainties` by `IssueNodeID`, preserving
each group's relative `ImpactRank` ordering. A caller with a
`synthesisagent.TentativeConclusion` in hand looks up
`report.ByIssue()[conclusion.IssueNodeID]` to answer "what's shaky about
this conclusion" — every finding attached, already ranked.

## Blocking over-confident phrasing (a flag, not a rewrite, not a gate)

`overconfidence.go`'s `overconfidentPhrases` wordlist —
`"definitely"`, `"certainly"`, `"undeniably"`, `"beyond doubt"`,
`"beyond any doubt"`, `"clearly proves"`, `"without question"`,
`"unquestionably"`, `"indisputably"`, `"there is no doubt"`,
`"conclusively proves"` — is scanned case-insensitively against every
`TentativeConclusion.Text`. Each match produces one
`OverconfidencePhrasing` recording the issue, the matched phrase, and a
short surrounding excerpt. `Text` itself is never rewritten or mutated;
rewriting over-confident conclusion text is out of scope for this
package, which reports rather than edits.

### Distinct from `irac.ContainsVerdictLanguage`

`irac.ContainsVerdictLanguage` checks a narrow, fixed wordlist of
verdict/directive phrasing — `"guilty"`, `"liable"`, `"is hereby
ordered"`, `"convicted"`, `"sentenced"`, and similar — binding-outcome
language that must never appear in reasoning output at all, enforced
today at `synthesisagent.Provider`'s tree-assembly boundary (rejecting,
not just flagging, any conclusion whose text trips it).

This package's `overconfidentPhrases` wordlist is deliberately disjoint:
it flags *epistemic over-claiming* — phrasing that overstates how certain
a draft, non-binding analysis should ever sound — without necessarily
asserting a binding outcome at all. "The evidence definitely establishes
negligence" trips this package's scan but not
`irac.ContainsVerdictLanguage`; "the defendant is guilty" trips the
reverse. The two checks are complementary, not overlapping.

### What Phase 057 does instead

This package's over-confidence scan is a **quality signal**: it never
blocks a run, never rejects a conclusion, and `Surface` always returns a
`Report` alongside a nil error for any input that passes basic
validation, regardless of how many `OverconfidenceFlags` it finds. Making
`irac.ContainsVerdictLanguage` (or any other check) an enforced,
run-blocking hard gate is explicitly **Phase 057 — Non-binding guardrail
enforcement**'s job, layered on top of this package rather than
duplicated here.

## Feeding the case-workspace UI (Part 6)

`Report` is designed to back a future case-workspace UI's "uncertainty
callouts" for a reviewing judge: `Report.ByIssue()` maps directly onto a
per-conclusion callout list ("3 uncertainties attached to this
conclusion, ranked, with caveats"), `Uncertainty.ImpactRank` drives which
callouts surface first case-wide, and `OverconfidenceFlags` backs an
inline text-highlighting affordance pointing at the exact phrase and
excerpt to soften. Nothing in this package renders UI or depends on any
UI package — it is a pure data-shaping step a future Part 6 frontend
package consumes.

## What this package deliberately does not do

- It does not call an LLM anywhere in its detection, ranking, or caveat-
  generation path (see "Design decision" above).
- It does not construct, mutate, or persist any `irac` tree node or
  edge, and it never mutates a `synthesisagent.Opinion` or rewrites any
  `TentativeConclusion.Text` — over-confidence detection is flag-only.
- It does not duplicate or recompute `synthesisagent.TentativeConclusion.
  WeakestLink` — see "Distinct from `synthesisagent`'s own `WeakestLink`"
  above.
- It does not enforce `irac.ContainsVerdictLanguage` or any other hard,
  run-blocking guardrail — that is Phase 057's job, layered on top of
  this package.
- It does not call `knowledgeapi` or any other I/O boundary itself —
  callers are expected to already hold the four upstream `Result`/
  `Opinion` values and pass them in via `Request`, keeping this
  package's core logic pure and easy to unit test.
- It does not decide case outcomes, resolve conflicting authority, or
  produce verdict/directive language of its own — every `Caveat` and
  `Report` this package emits describes a doubt, it never resolves one.
