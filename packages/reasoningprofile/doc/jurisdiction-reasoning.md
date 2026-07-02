# Jurisdiction-parameterized reasoning (`packages/reasoningprofile`)

Phase 058's cross-cutting reminder, verbatim from the implementation plan:

> Jurisdiction parameterization (Phase 58) is the core differentiator: one
> engine, reweighted by legal family per deployment.

`packages/reasoningprofile` is that reweighting: one canonical `Weights`
shape, resolved per `Family`, for all four legal families
`packages/jurisdiction` recognizes — `common_law`, `civil_law`, `mixed`,
and `islamic_law`.

## The gap this phase closes

Before this phase, two packages already had jurisdiction-weighting
profiles, and both had the same gap:

- `packages/evidenceweighing.JurisdictionProfile` / `ProfileForFamily`
  covered `common_law` (testimony-heavy) and `civil_law`
  (documentary-heavy), and silently fell back to `NeutralProfile()` for
  everything else — including `mixed` and `islamic_law`, which
  `packages/jurisdiction.LegalFamily` had declared as first-class
  constants since a much earlier phase.
- `packages/lawapplication.OriginProfile` / `ProfileForFamily` had the
  identical shape and the identical gap for statute-vs-precedent
  weighting.

In other words: two of the four canonical legal families this platform
claims to model were reasoning under a profile that treats every kind of
evidence and every kind of legal authority as equally weighted — not
because that is a considered position about mixed or Islamic-law
jurisdictions, but because nobody had written the weights yet. This phase
writes them, in one canonical place, and threads the result into both
sibling packages.

## The four canonical profiles

`Weights` captures both dimensions the sibling packages model separately:

```go
type Weights struct {
    TestimonyEmphasis   float64 // vs. evidenceweighing.JurisdictionProfile.Testimony
    DocumentaryEmphasis float64 // vs. evidenceweighing.JurisdictionProfile.Documentary
    StatuteEmphasis     float64 // vs. lawapplication.OriginProfile.Statute
    PrecedentEmphasis   float64 // vs. lawapplication.OriginProfile.Precedent
}
```

| Family        | Testimony | Documentary | Statute | Precedent |
|---------------|-----------|-------------|---------|-----------|
| `common_law`  | 1.0       | 0.9         | 0.8     | 1.0       |
| `civil_law`   | 0.8       | 1.0         | 1.0     | 0.8       |
| `mixed`       | 0.9       | 0.95        | 0.9     | 0.9       |
| `islamic_law` | 0.85      | 1.0         | 1.0     | 0.95      |

### Common law: precedent-heavy and testimony-heavy

`CommonLawWeights()` matches what `evidenceweighing.CommonLawProfile()`
and `lawapplication.CommonLawProfile()` already encoded: adversarial live
testimony and cross-examination sit at the center of common-law
fact-finding (`TestimonyEmphasis` at full strength), and judicial
precedent (*stare decisis*) is itself a primary, binding source of law
(`PrecedentEmphasis` above `StatuteEmphasis`).

### Civil law: statute-heavy and documentary-heavy

`CivilLawWeights()` mirrors `evidenceweighing.CivilLawProfile()` and
`lawapplication.CivilLawProfile()`: inquisitorial procedure and a
documentary/written-record tradition dominate fact-finding
(`DocumentaryEmphasis` above `TestimonyEmphasis`), and codified statute is
the primary source of law, with judicial decisions persuasive but not
formally binding (`StatuteEmphasis` above `PrecedentEmphasis`).

### Mixed: a genuine blend, not an alias for neutral

`MixedWeights()` sits at the midpoint of `CommonLawWeights()` and
`CivilLawWeights()` on every dimension. This was a deliberate choice over
the two more obvious alternatives:

- **Alias `NeutralProfile()` (all 1.0s).** Rejected: a mixed-family
  jurisdiction (the plan's own example is "common law + Islamic law, or
  civil law + customary law" — see
  `packages/jurisdiction/legal_family.go`) is not a jurisdiction with *no*
  evidentiary or authority tradition; it is one with *two or more*
  traditions in tension. Neutral says "no basis to prefer," which is
  false for a mixed system — it is more accurate to say the deployment
  should expect real, but attenuated, pulls in both directions.
- **Pick one parent tradition arbitrarily.** Rejected: there is no
  general basis for assuming a mixed jurisdiction leans common-law over
  civil-law (or vice versa) without deployment-specific configuration
  this package does not have. The midpoint is the least-committal
  non-neutral choice available without that configuration.

This is a coarse model. A specific mixed jurisdiction (e.g. Louisiana,
Quebec, or a Gulf state blending common law commercial courts with
Sharia-influenced personal-status law) may lean far more toward one
parent tradition than a flat midpoint suggests. `SetOverride` (below)
exists precisely so a deployment with better information about its
specific mixed jurisdiction can correct for this per case.

### Islamic law: rationale and limitations (read this before using in production)

`IslamicLawWeights()` is this phase's most values-sensitive design
choice, and it is treated with the same care the rest of the platform's
non-binding-guardrail work demonstrates. Read this section fully before
relying on these specific numbers.

**The rationale for the chosen values:** "Islamic law" as tracked by
`packages/jurisdiction.LegalFamilyIslamicLaw` spans an enormous range of
actual legal systems — from fully codified civil-law-style statutes
heavily influenced by Sharia (much of the modern Gulf, e.g. UAE and Saudi
commercial codes) to systems with substantial direct judicial
application of classical fiqh. Two considerations informed the specific
weights chosen here:

- **Statute emphasis is set high (1.0, matching civil law).** Most
  Islamic-law jurisdictions a platform is likely to encounter today
  operate through heavily codified statute — modern Gulf states in
  particular have codified commercial, contract, and much of family law
  even where the underlying substantive rules are Sharia-derived.
- **Precedent emphasis is set nearly as high (0.95).** Classical Islamic
  jurisprudence gives substantial weight to established juristic
  consensus and precedent-like reasoning (*ijma* and the accumulated
  rulings of recognized schools of jurisprudence,
  *madhahib*) even where it is not "precedent" in the common-law
  *stare decisis* sense. The result is a profile where statute and
  precedent are both weighted highly and close together, rather than one
  dominating the other the way it does under common law or civil law.
- **Documentary emphasis is set to 1.0 and testimony emphasis to 0.85.**
  Islamic commercial and family law both treat written instruments
  (contracts, deeds) with particular evidentiary formality, while
  witness testimony remains significant but is not given the
  cross-examination-centered primacy it has under common law.

**The explicit limitation:** this is a simplified computational model of
a single scalar profile applied to an entire, highly diverse legal
family — it is not, and does not claim to be, a statement of Islamic
legal or religious doctrine, and it does not distinguish between schools
of jurisprudence, between classical and modern codified application, or
between the many national systems the `islamic_law` tag can cover. A
deployment operating in a specific Islamic-law jurisdiction should treat
`IslamicLawWeights()` as a reasonable, documented starting default, not
as an authoritative or final answer, and is expected to use
`SetOverride` (below) where its own domain expertise indicates a
different weighting is more accurate for that jurisdiction. This
humility note is deliberately as prominent as the guardrail package's own
"why override-prevention matters" section — the design principle is the
same: be explicit about what this code does and does not claim to know.

## This package is the canonical source; sibling packages derive from it, not the other way around

`packages/evidenceweighing/jurisdiction.go` and
`packages/lawapplication/jurisdiction.go` have each been extended, in
this phase, with:

- New `LegalFamily` constants — `MixedFamily` / `IslamicLawFamily` in
  both packages.
- New constructors — `MixedProfile()` / `IslamicLawProfile()` in both
  packages, whose `Testimony`/`Documentary` (evidenceweighing) or
  `Statute`/`Precedent` (lawapplication) values are copied directly from
  this package's `MixedWeights()`/`IslamicLawWeights()`.
- An extended `ProfileForFamily` switch covering all four families
  explicitly, still defaulting unknown/empty input to `NeutralProfile()`
  (unlike this package's own `WeightsForFamily`, which treats an
  unrecognized `Family` as a hard error — see "Two different
  fallback philosophies," below).

Neither sibling package imports `packages/reasoningprofile`. The
constants are duplicated, not shared, by deliberate choice: both sibling
packages predate this phase and have their own dependency-light
conventions (see each package's own doc comment on `LegalFamily` being
"an opaque, caller-defined string rather than a hard dependency on
packages/jurisdiction"). Introducing a new cross-package dependency for
four float64 constants would cost more in coupling than it saves in
duplication. Instead, `packages/reasoningprofile`'s own test suite
(`weights_test.go`,
`TestEvidenceWeighingAlignment`/`TestLawApplicationAlignment`) imports
*both* sibling packages and asserts their `CommonLawProfile()`/
`CivilLawProfile()` values match this package's `CommonLawWeights()`/
`CivilLawWeights()` exactly — so drift between the three packages' common
and civil law numbers would fail this package's tests, not go unnoticed.
(The two packages' own test suites separately confirm mixed/islamic_law
are no longer aliases of `NeutralProfile()` — see each package's
`jurisdiction_test.go`.)

### Two different fallback philosophies, both intentional

- **`evidenceweighing.ProfileForFamily` / `lawapplication.ProfileForFamily`**
  default any unrecognized `LegalFamily` (including empty) to
  `NeutralProfile()`. This is the pre-existing, unchanged behavior of
  those packages — appropriate for a low-level scoring function that
  must always return *something* usable rather than force every caller
  to handle an error for what might be a genuinely unclassified
  jurisdiction.
- **`reasoningprofile.WeightsForFamily`** has no such fallback: an
  unrecognized `Family` returns the zero `Weights` value and
  `ErrUnknownFamily`. This package sits one layer up, at the point where
  a deployment is meant to have already resolved a real `Family` from
  `packages/jurisdiction` — silently returning a meaningless neutral
  profile here would hide a configuration bug (e.g. a typo'd family
  string) that should surface immediately instead.

## Resolving a family, with override and audit

```go
func ResolveFamily(j jurisdiction.Jurisdiction) Family
func WeightsForFamily(family Family) (Weights, error)

type OverrideRegistry struct{ /* ... */ }
func NewOverrideRegistry(sink AlertSink, now func() time.Time) *OverrideRegistry
func (r *OverrideRegistry) SetOverride(caseID string, family Family, reason string) error
func (r *OverrideRegistry) OverrideFor(caseID string) (Family, bool)
func ResolveWithOverride(r *OverrideRegistry, caseID string, fallback Family) Family
```

`ResolveFamily` is a one-line wrapper reading `Jurisdiction.LegalFamily`.
The more interesting seam is `OverrideRegistry`: a deployment may know
more about a specific case than the jurisdiction registry does — for
example, parties who have contractually agreed to argue under a
different evidentiary regime, or a mixed jurisdiction where the
deployment has domain expertise that the flat `MixedWeights()` midpoint
doesn't capture for that specific case. `SetOverride` records the change
unconditionally as an `Event{CaseID, PreviousFamily, OverrideFamily,
Reason, AppliedAt}` and forwards it to the configured `AlertSink` —
mirroring `packages/guardrail.Event`/`AlertSink`/`NoOpAlertSink` exactly,
down to the same `FuncAlertSink`/`MultiAlertSink` helpers and the same
defensive-copy `Events()` accessor. There is no way to set an override
without it being audited, for the same reason `packages/guardrail` has no
"skip validation" flag: a silently-applied override to a case's legal
reasoning profile is exactly the kind of change that must be visible to
a later reviewer.

`ResolveWithOverride` is the convenience composition: call
`ResolveFamily` to get the jurisdiction-derived default, then pass it as
`fallback` so an override (if any) wins.

## Validation

```go
func Validate(w Weights) error       // rejects NaN/Inf/negative/out-of-[0,1] fields
func ValidateFamily(family Family) error // rejects anything but the canonical four
```

`Validate` returns an `*InvalidWeightError` (wrapping `ErrInvalidWeight`,
so `errors.Is` works) naming the specific offending field, so a
deployment that builds a custom `Weights` value (e.g. for a
`SetOverride`-driven per-case adjustment beyond what this package
predefines) gets an actionable error rather than a silently-wrong
profile propagating through the pipeline.

## Composes with, does not duplicate

| Package | Owns | This package's relationship |
|---|---|---|
| `packages/jurisdiction` | `LegalFamily` and its four canonical values; `Jurisdiction.LegalFamily`. | `ResolveFamily` reads `Jurisdiction.LegalFamily` directly. `Family`'s four string values are chosen to match `LegalFamily`'s exactly, verified by `family_test.go`, but `Family` is declared independently (not a type alias), matching the sibling packages' own decoupling convention. |
| `packages/evidenceweighing` | `JurisdictionProfile`, `ProfileForFamily`, `CommonLawProfile`/`CivilLawProfile`/`NeutralProfile`. | Extended (this phase) with `MixedProfile`/`IslamicLawProfile` whose values are copied from this package's `MixedWeights`/`IslamicLawWeights`. Not imported by this package's non-test code; imported by this package's tests to assert alignment. |
| `packages/lawapplication` | `OriginProfile`, `ProfileForFamily`, `CommonLawProfile`/`CivilLawProfile`/`NeutralProfile`. | Same relationship as `packages/evidenceweighing`, for the statute/precedent dimension. |
| `packages/guardrail` | `Event`/`AlertSink`/`NoOpAlertSink`/`FuncAlertSink`/`MultiAlertSink`/`Recorder` audit convention. | Not imported. `override.go`'s `Event`/`AlertSink`/`NoOpAlertSink`/`FuncAlertSink`/`MultiAlertSink` mirror the same shape for a different audit domain (family overrides instead of guardrail violations), rather than sharing a cross-package audit type. |

## What this package deliberately does not do

- **It does not call `packages/evidenceweighing.ScoreFact`/`Weigh` or
  `packages/lawapplication.WeightByOrigin`/`Apply`.** This package has no
  notion of a `FactRef`, a `RuleRef`, a case's evidence set, or its rule
  set — it resolves and validates `Weights`, full stop. Threading a
  resolved `Family`/`Weights` through an actual end-to-end case run,
  calling into `evidenceweighing` and `lawapplication` with that
  resolved profile at the right pipeline stages, is Phase 059's
  reasoning-orchestration pipeline, not this package.
- **It does not persist anything.** `OverrideRegistry` is an in-memory
  map plus an in-memory audit log; a deployment wanting durable override
  storage is expected to wire its own `AlertSink` implementation to a
  real backend and/or persist `SetOverride` calls at a higher layer.
- **It does not modify `packages/jurisdiction`, `packages/evidenceweighing`,
  or `packages/lawapplication`'s own `LegalFamily` type.** Each package
  keeps its own independently-declared `LegalFamily`/`Family` string
  type, by the established convention; this package does not attempt to
  unify them into one shared type.
- **It does not claim legal or religious authority for
  `IslamicLawWeights()`, or completeness for `MixedWeights()`.** Both are
  documented, simplified computational defaults meant to be corrected
  per deployment and per case via `SetOverride` where better information
  is available — see the rationale sections above.
