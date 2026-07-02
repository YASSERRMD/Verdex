# Non-binding guardrail enforcement (`packages/guardrail`)

Verdex's entire legal and ethical premise rests on one sentence from
`CONTRIBUTING.md`:

> Every module that produces reasoning output must attach the
> `draft_analysis` label. Verdict or directive language is rejected by
> the output pipeline.

Fifty-six phases built the reasoning pipeline that sentence describes.
`packages/guardrail` is the project-wide policy layer that turns the
sentence itself into code — a hard-enforcement surface every
reasoning-output-producing package can, and per policy must, call.

## Why this needs its own package

The guarantee already existed in pieces before this phase:

- `packages/irac.NewConclusionNode` already unconditionally attaches
  `DraftAnalysisLabel` to every `ConclusionNode`.
- `packages/irac.ContainsVerdictLanguage` already detects verdict and
  directive phrasing.
- `packages/synthesisagent.Provider` already called that detector inline,
  before ever constructing a `ConclusionNode`.

That was real enforcement, but it was local to one call site in one
package, with the underlying keyword-matching logic uncomfortably close
to the one caller that used it. Nothing stopped some *future* phase's
reasoning-output type — an exported report, a jurisdiction-specific
summary, a batch-export job — from skipping the check entirely, because
there was no shared, reusable, project-wide surface to call. This
package is that surface: every check `synthesisagent.Provider` needed is
now available, generalized, to every future producer of reasoning
output, and `synthesisagent.Provider` itself has been refactored to call
it rather than keep its own copy.

## The policy surface

### 1. Labeling: `OutputLabel`, `Labeled`, `RequireLabel`, `ValidateLabeled`

```go
const DraftAnalysisLabel OutputLabel = OutputLabel(irac.DraftAnalysisLabel)

type Labeled interface { Label() string }

func RequireLabel(label string) error
func ValidateLabeled(x Labeled) error
```

`DraftAnalysisLabel` is not a new label — it is `irac.DraftAnalysisLabel`
re-exported under this package's name, so every consumer of `guardrail`
has one canonical source for the label value without needing to also
import `packages/irac` just to check it.

**Which existing output types already satisfy this, and which don't:**

| Type | Satisfies `Labeled` today? | Notes |
|---|---|---|
| `irac.ConclusionNode` | Yes, via `WrapConclusionNode` | `ConclusionNode.Label` is always `draft_analysis` when built through `irac.NewConclusionNode` — the only exported constructor. `WrapConclusionNode` adapts the existing `Label` field to `Labeled` for defensive re-verification at this package's boundary (e.g. after a node round-trips through untrusted deserialization). |
| `synthesisagent.Opinion` / `TentativeConclusion` | **No — documented gap** | Neither type carries a `Label` field. `Opinion` is documentation-only "non-binding": its guarantee comes entirely from its `ConclusionProvider` (`synthesisagent.Provider`) refusing to convert verdict-flavored conclusions into `ConclusionNode`s. This package deliberately does not add a wrapper type pairing `Opinion` with a label, because `Opinion` is an intermediate, pre-tree artifact — labeling it would create a second, easily-desynchronized source of truth alongside the `ConclusionNode`s it eventually produces. A future phase exporting `Opinion` directly (e.g. as a standalone report before tree assembly) should either add its own `Label() string` method backed by `guardrail.DraftAnalysisLabel`, or route through `RequireDisclaimer`/`EnsureDisclaimer` (below) instead, since a human-facing export needs a disclaimer more than it needs a machine-checkable label field. |

### 2. Verdict/directive blocking: `CheckText`

```go
func CheckText(s string) error // wraps irac.ContainsVerdictLanguage
```

`CheckText` returns `ErrVerdictLanguageDetected` instead of a bare bool.
This is not a stylistic preference: a bool-returning check can be called
and its result silently discarded (`_ = irac.ContainsVerdictLanguage(s)`
compiles and does nothing); an error-returning check that a caller must
either check or explicitly discard with `_ = err` is much harder to
integrate incorrectly, and the explicit discard is a visible, greppable
code smell in review.

`CheckText` is a **strict superset** of what
`packages/synthesisagent.Provider` checked before this phase: both call
the exact same `irac.ContainsVerdictLanguage`, so the set of rejected
text is byte-for-byte identical. `packages/synthesisagent/provider.go`
has been refactored to call `guardrail.CheckText` instead of
`irac.ContainsVerdictLanguage` directly (see
`packages/synthesisagent/provider_guardrail_test.go` for the
override-prevention proof that this refactor changed nothing about what
gets rejected).

**A known, documented gap in the underlying wordlist:** the phrase
"judgment is entered for the plaintiff" is *not* rejected, because
`irac.ContainsVerdictLanguage`'s wordlist matches the substring
`"judgment for"`, and "is entered" sits between "judgment" and "for" in
that phrasing. "judgment for the plaintiff is entered" (and the more
common courtroom phrasing "judgment for the plaintiff") *are* rejected.
This package does not attempt to fix or extend the wordlist — that
belongs to `packages/irac` (the wordlist's owner) — but flags it here so
a future phase strengthening `irac.ContainsVerdictLanguage`'s coverage
knows this is a real, test-confirmed gap (see
`packages/guardrail/verdict_test.go`).

### 3. Mandatory disclaimer injection: `RequireDisclaimer` / `EnsureDisclaimer`

```go
func RequireDisclaimer(text string) string // idempotent
func EnsureDisclaimer(text string) string  // alias
func HasDisclaimer(text string) bool
```

`packages/prompts`'s `nonBindingDisclaimer` already solves this for
LLM-facing **input**: `PromptTemplate.NonBindingLabel` appends a
disclaimer to a rendered prompt before it is sent to a model. This
package's disclaimer is the mirror-image mechanism for reasoning
**output** — text a human will read, e.g. a rendered `Opinion` or an
exported report. The two disclaimers are deliberately separate constants
with separate wording (though the same spirit), because `prompts.Render`
operates on a `*prompts.PromptTemplate` and has no notion of an arbitrary
output string; unifying them would require either package to depend on
the other for no structural benefit.

`RequireDisclaimer`/`EnsureDisclaimer` never return an error: there is no
counterfactual "reject" outcome for text missing a disclaimer, only "add
it" — which they always do, idempotently.

### 4. Human sign-off gate: `SignoffStatus`, `SignoffGate`, `CanFinalize`

```go
type SignoffStatus int
const (
    SignoffPending SignoffStatus = iota
    SignoffApproved
    SignoffRejected
)

type SignoffGate interface {
    Status(ctx context.Context, caseID string) (SignoffStatus, error)
}

type NoSignoffRecordedGate struct{}

func CanFinalize(ctx context.Context, caseID string, gate SignoffGate) (bool, error)
```

Human sign-off is not a feature that exists anywhere in Verdex today —
it is Phase 068 (Human sign-off workflow, Part 6 of the implementation
plan), far beyond this batch of phases. `SignoffGate` is this phase's
forward-looking extension point for that future work, mirroring
`packages/treeassembly.ConclusionProvider`'s pattern precisely:
`treeassembly.ComposeTree` accepted a `ConclusionProvider` interface
starting at Phase 039 with only a no-op implementation
(`NoOpConclusionProvider`) until Phase 055 supplied
`packages/synthesisagent.Provider` — no change to `treeassembly`'s own
composition logic was required when that happened. The same shape
applies here: once Phase 068 exists, it need only implement
`SignoffGate` and be wired into whatever caller invokes `CanFinalize`; no
change to this package is required.

`NoSignoffRecordedGate` is the only implementation today, and it always
reports `SignoffPending` — **fail-closed, not fail-open**. There is, as
of this phase, no mechanism anywhere in the codebase by which a case
could ever legitimately have an approved sign-off, so reporting anything
else would be a fail-open lie that defeats the entire point of the gate.
`CanFinalize` requires exactly `SignoffApproved`; `SignoffPending` and
`SignoffRejected` both block, distinguished only for audit purposes (
"nobody has looked yet" vs. "somebody looked and said no").

`guardrail.CanFinalize` is a **different, narrower gate** than
`treevalidation.CanFinalize`. The latter blocks on tree **structural**
integrity (`ErrCriticalFindings`, from `packages/treevalidation`); this
one blocks on **human sign-off state**. Neither package imports the
other; a caller that needs both guarantees (which every real
finalization path should) calls both gates explicitly.

### 5. Audit: `Event`, `ViolationKind`, `AlertSink`, `Recorder`

```go
type ViolationKind string
const (
    ViolationMissingLabel    ViolationKind = "missing_label"
    ViolationVerdictLanguage ViolationKind = "verdict_language"
    ViolationFinalizeBlocked ViolationKind = "finalize_blocked"
)

type Event struct {
    Kind       ViolationKind
    CaseID     string
    Detail     string
    OccurredAt time.Time
}

type AlertSink interface { Notify(event Event) }
```

This mirrors `packages/knowledgeisolation`'s `AccessAttempt`/`AlertSink`
pattern (Phase 047) exactly, rather than inventing a new audit style:
`NoOpAlertSink` (default, discards silently), `FuncAlertSink` (adapts a
plain function), `MultiAlertSink` (fans out to several sinks), and a
`Recorder` (mutex-protected event log + `Events()` defensive-copy
accessor) mirroring `knowledgeisolation`'s unexported `auditRecorder`
convention, exported here as `Recorder` since this package has no
internal-only guard type to hide it behind.

Recording is **opt-in and additive**: none of `CheckText`, `RequireLabel`,
`ValidateLabeled`, or `CanFinalize` call a `Recorder` themselves — they
are plain functions with no recorder to hold. A caller that wants an
audit trail constructs a `Recorder` and calls the matching
`RecordCheckTextFailure`/`RecordLabelFailure`/`RecordFinalizeBlocked`
helper alongside its own error handling. This keeps the core checks
dependency-free (no forced audit-log allocation for a caller that
doesn't need one) while still providing the standard audit shape for
callers that do.

## Why override-prevention matters here specifically

Every other kind of bug in this platform is recoverable after the fact: a
bad citation can be corrected in a later revision, a mis-weighted fact
can be reweighted, a gap in issue coverage can be filled in a follow-up
pass. A reasoning output that reaches a human, a filing, or a downstream
system carrying verdict or directive language — even once — cannot be
walked back, because the reader has no way to know, after the fact, that
what they read was supposed to have been blocked. The platform's legal
and ethical premise is not "we usually catch verdict language"; it is
"verdict language never ships."

That is why this package is built with these constraints, deliberately:

- **Every check returns only an error, never a bare bool alongside it.**
  `CheckText`, `RequireLabel`, `ValidateLabeled`, and `CanFinalize` each
  have exactly one exported signature. There is no
  `ContainsVerdictLanguage`-style bool variant in this package that a
  caller could call instead and silently ignore the result of.
- **There is no "skip validation" flag anywhere in this package.** No
  constructor parameter, no package-level variable, no build tag disables
  any check. The only way to avoid a check is to not call this package at
  all — which is a visible omission in code review, not a flippable
  switch.
- **`CanFinalize` fails closed.** `NoSignoffRecordedGate` reports
  `SignoffPending`, never `SignoffApproved`, until a real Phase 068
  implementation exists and is wired in. A caller cannot get an
  `approved` outcome by simply forgetting to wire up sign-off tracking —
  the default behavior is the safe one.
- **The synthesisagent override-prevention test
  (`packages/synthesisagent/provider_guardrail_test.go`,
  `TestProvider_NoPathToVerdictConclusionNode`) is a concrete proof, not
  just a design claim:** it exercises `Provider.Provide` against verdict
  language in every position (sole conclusion, mixed with legitimate
  conclusions, trailing in an otherwise-clean sentence, case-varied, and
  an all-verdict `Opinion`), and confirms zero verdict-flavored
  `ConclusionNode`s ever survive, with every surviving node independently
  re-checked against `guardrail.ValidateLabeled` and `guardrail.CheckText`.

## Composes with, does not duplicate

| Package | Owns | This package's relationship |
|---|---|---|
| `packages/irac` | `DraftAnalysisLabel`, `NewConclusionNode`, `ContainsVerdictLanguage`. | Wraps, never redefines: `guardrail.DraftAnalysisLabel` is `irac.DraftAnalysisLabel` re-exported; `CheckText` wraps `irac.ContainsVerdictLanguage` with an error-returning signature. |
| `packages/synthesisagent` | `Provider`, `Opinion`, `TentativeConclusion`. | `Provider.Provide` has been refactored (this phase) to call `guardrail.CheckText` instead of `irac.ContainsVerdictLanguage` directly — a real dependency, not a doc-only reference. |
| `packages/prompts` | `PromptTemplate.NonBindingLabel`, `Render`'s input-facing disclaimer injection. | Not imported. This package's `RequireDisclaimer`/`EnsureDisclaimer` mirror the spirit for the output-facing surface, as a deliberately separate mechanism. |
| `packages/treevalidation` | `CanFinalize` / `ErrCriticalFindings` (tree structural-integrity gate). | Not imported, not replaced. This package's `CanFinalize` is a separate, narrower human-sign-off gate; a caller needing both guarantees calls both. |
| `packages/treeassembly` | `ConclusionProvider` extension-point pattern (Phase 039). | Not imported. `SignoffGate`/`NoSignoffRecordedGate` mirror the same "define the seam now, provide a safe default, let a much later phase fill it in" pattern. |
| `packages/knowledgeisolation` | `AccessAttempt`/`AlertSink` audit convention (Phase 047). | Not imported. `Event`/`AlertSink`/`Recorder` mirror the same shape for a different violation domain, rather than sharing a cross-package audit type for what is otherwise a one-method interface. |

## What this package deliberately does not do

- **It does not make any LLM/provider calls.** No hardcoded provider, no
  provider calls of any kind — this is a pure policy/validation layer.
- **It does not persist anything.** `Recorder` is an in-memory audit log;
  a caller wanting durable audit storage is expected to wire its own
  `AlertSink` implementation to a real backend.
- **It does not implement the Phase 068 human sign-off workflow.** It
  only defines the `SignoffGate` seam Phase 068 is expected to fill;
  `NoSignoffRecordedGate` is a safe placeholder, not a feature.
- **It does not extend or fix `irac.ContainsVerdictLanguage`'s
  wordlist.** That wordlist is owned by `packages/irac`; this package
  documents a known gap (see above) rather than silently working around
  it with a second, divergent wordlist.
- **It does not expose any bypass, skip-validation flag, or bool-only
  variant of any check.** See "Why override-prevention matters here
  specifically," above — this is a deliberate, permanent design
  constraint, not a temporary limitation.
- **It does not add a `Label`/`Labeled` field to `synthesisagent.Opinion`
  or `TentativeConclusion`.** That gap is documented above as an
  explicit design decision, not an oversight.
