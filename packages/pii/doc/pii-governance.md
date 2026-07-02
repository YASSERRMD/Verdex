# Verdex PII Detection & Governance Model

## Overview

`packages/pii` detects and governs sensitive personal data in case text
before it reaches downstream reasoning or storage. It operates on plain or
`packages/segmentation`-produced text: any string content flowing through
the Verdex pipeline (a `Segment.Text`, an intake artifact's extracted text,
a raw case narrative) can be passed to `PIIService.Process` before it is
persisted, embedded, or handed to an `LLMProvider`.

Like `packages/segmentation`'s splitting/heading/exhibit detection and
`packages/multilingual`'s tokenizer, the default detector in this package is
deterministic and rule-based. No component depends on a machine-learning
model at runtime — but the detection surface is designed as a pluggable
extension point so a real NER (named-entity-recognition) model or hosted PII
API can be swapped in later without touching any caller.

---

## The PIIMatch Entity

A `PIIMatch` is the atomic unit of detection:

```go
type PIIMatch struct {
    Start      int
    End        int
    Text       string
    Category   PIICategory
    Pattern    string
    Confidence float64
}
```

`Start`/`End` are rune offsets into the source text, mirroring
`packages/segmentation`'s `SourceSpan` convention (`Start` inclusive, `End`
exclusive), so matches locate precisely within multi-byte-rune text.

`PIICategory` classifies the kind of PII:

| Category             | Meaning                                                   |
| --------------------- | ---------------------------------------------------------- |
| `CategoryName`        | Person names (heuristic capitalized sequences, honorifics). |
| `CategoryContact`     | Email addresses, phone numbers.                            |
| `CategoryIdentifier`  | National IDs, passport numbers, SSNs, and similar.         |
| `CategoryAddress`     | Physical/postal addresses.                                 |
| `CategoryFinancial`   | Bank account numbers, card numbers, IBANs.                 |
| `CategoryOther`       | Anything else a Detector reports.                          |

---

## Detection: A Pluggable Interface

```go
type Detector interface {
    Detect(ctx context.Context, text string) ([]PIIMatch, error)
}
```

`RuleBasedDetector` is the default implementation: a deterministic set of
regular expressions and light heuristics covering email addresses, phone
numbers (including international `+` formats), national-ID-like number
sequences, physical street addresses, and person names (honorific-prefixed
or bare "First Last" capitalization). It calls no external service and its
output is fully reproducible given the same input.

`RuleBasedDetector` favors **recall over precision**: borderline matches
(e.g. any "Capitalized Word Capitalized Word" sequence) are still reported,
because downstream redaction is expected to over-redact rather than leak
PII. Overlapping matches from different patterns are de-duplicated,
preferring the earliest-starting, then longest, match.

A future phase can implement `Detector` with a real NER model or a hosted
PII-detection API and swap it into `PIIService.Detector` (or
`StoragePolicy.Detector`) without changing any other code in this package
or its callers.

---

## Redaction and Pseudonymization

```go
type RedactionMode string

const (
    ModeRedact             RedactionMode = "redact"
    ModePseudonymize       RedactionMode = "pseudonymize"
    ModeIrreversibleRedact RedactionMode = "irreversible_redact"
)
```

- **`ModeRedact`** replaces a match with a fixed placeholder,
  `[REDACTED:category]` (e.g. `[REDACTED:contact]` for an email match,
  keyed by `PIICategory`). No mapping is stored.
- **`ModePseudonymize`** replaces a match with a stable per-entity token
  (e.g. `PERSON_1`, `CONTACT_2`) allocated from a `PseudonymMap`. The same
  original value always maps to the same token within a given map, so
  repeated mentions of the same entity stay consistent. **This is the only
  reversible mode.**
- **`ModeIrreversibleRedact`** produces the same `[REDACTED:category]`
  placeholder as `ModeRedact`, but is a distinct, explicit mode so callers,
  jurisdiction rules, and storage policy can require "this category must
  never be recoverable" and have that requirement visible in code and audit
  logs rather than being an accidental side effect of configuration.

`RedactionMode.IsReversible()` reports `true` only for `ModePseudonymize`.

A `Redactor` applies one mode as a package-wide default, with optional
per-category overrides (`ModeByCategory`), over a set of classified matches
within a text.

---

## Mapping Under Access Control

```go
type AccessPolicy interface {
    CanReveal(ctx context.Context, requester string) bool
}
```

`PseudonymMap` stores the original↔token mapping produced under
`ModePseudonymize`, gated behind an `AccessPolicy`. There is no default
permissive policy: `NewPseudonymMap(nil)` defaults to `DenyAllAccessPolicy`,
so "no policy configured" is a safe, explicit failure mode rather than an
accidental leak.

`PseudonymMap.Reveal(ctx, requester, token)` returns:

- `ErrUnknownToken` if the token was never allocated,
- `ErrAlreadyIrreversible` if the token was allocated via
  `TokenForIrreversible` (see below) — regardless of what the
  `AccessPolicy` says,
- `ErrAccessDenied` if the policy denies `requester`,
- otherwise the original value.

`PseudonymMap.TokenForIrreversible` allocates a stable token exactly like
`TokenFor`, but **discards the original value immediately and stores no
recoverable mapping at all**. `Reveal` always fails with
`ErrAlreadyIrreversible` for such tokens, even for a requester an
`AccessPolicy` would otherwise trust — there is no "break glass" path around
irreversibility by design.

---

## Jurisdiction-Specific Rules

`JurisdictionPIIRules`, keyed by jurisdiction code (mirroring
`packages/jurisdiction`'s `CountryCode` convention), lets a specific
jurisdiction override the sensitivity score and/or required `RedactionMode`
for a category — e.g. "national IDs must always be irreversibly redacted for
jurisdiction X" — without changing the core detect/redact pipeline.
`JurisdictionPIIRules.ApplyToMatches` resolves these overrides into a
`ModeByCategory`-shaped map that both `PIIService` and `StorageGuard` merge
on top of their own configured defaults.

---

## Storage Boundary Enforcement

`StorageGuard` wraps a write path: given text about to be persisted, it
detects and classifies PII, rejects the write outright
(`ErrPolicyViolation`) if any match falls in a configured
`RejectCategories` set, and otherwise redacts remaining matches per policy
(including jurisdiction overrides) before returning text safe to pass to the
caller's actual storage write. `StorageGuard` performs no database I/O
itself — it is a boundary-enforcement function a caller places immediately
before its own write.

---

## Audit Logging

`AuditEvent` records every detect/redact/reveal action with a timestamp and
actor, mirroring `packages/intake`'s `AuditSink` pattern exactly
(`NoOpAuditSink`, `LoggingAuditSink`, `CapturingAuditSink`). Audit events
**never include the raw PII value** — only match counts, category-scoped
outcomes, tokens (not the values behind them), and allow/deny results for
reveal attempts.

---

## Pipeline: PIIService

```
input text
  │
  ▼
Detector.Detect()                      ← RuleBasedDetector by default
  │
  ▼
ClassifyMatches()                       ← attach PIICategory
  │
  ▼
JurisdictionPIIRules.ApplyToMatches()   ← per-category mode overrides
  │
  ▼
Redactor.Redact()                       ← apply mode(s), consult PseudonymMap
  │
  ▼
AuditSink.Emit()                        ← detect + redact events
  │
  ▼
sanitized text + match report
```

`PIIService.Reveal` is the audited pass-through to
`PseudonymMap.Reveal`, so every reveal attempt — successful or not — is
recorded.

---

## Non-Goals

- This package does not call any external ML/NER service today; `Detector`
  is the seam where one would be added.
- `StorageGuard` does not perform actual database, file, or queue I/O; it
  is a pure boundary-enforcement function.
- Detection is intentionally recall-biased, not precision-tuned; false
  positives (e.g. a capitalized two-word phrase that isn't actually a
  person's name) are expected and considered an acceptable cost against the
  risk of missing real PII.
