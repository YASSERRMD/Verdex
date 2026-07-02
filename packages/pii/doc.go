// Package pii detects and governs sensitive personal data in case text
// before it reaches downstream reasoning or storage.
//
// Core concepts:
//
//   - Detector / PIIMatch: the pluggable detection interface
//     (Detect(ctx, text) ([]PIIMatch, error)) plus RuleBasedDetector, a
//     deterministic regex/heuristic default implementation covering
//     person-name heuristics, phone numbers, email addresses,
//     national-ID-like number patterns, and physical addresses (detector.go).
//   - PIICategory / ClassifyMatches: classifies each PIIMatch into Name,
//     Contact, Identifier, Address, Financial, or Other (category.go).
//   - RedactionMode / Redactor: applies a configured mode over detected
//     matches -- ModeRedact ("[REDACTED:category]"), ModePseudonymize (a
//     stable per-entity token like "PERSON_1", reversible), or
//     ModeIrreversibleRedact (a placeholder with no mapping ever stored,
//     irreversible) (redact.go).
//   - PseudonymMap / AccessPolicy: retains the original<->token mapping
//     produced by ModePseudonymize, gated behind an AccessPolicy so only
//     authorized callers can reverse a pseudonymization (mapping.go).
//   - JurisdictionPIIRules: per-jurisdiction category sensitivity and
//     required-redaction-mode overrides, keyed by jurisdiction code
//     (jurisdiction_rules.go).
//   - AuditEvent / AuditSink: records every detect/redact/reveal action
//     with a timestamp and actor, mirroring packages/intake's audit
//     pattern -- never logging the raw PII value itself (audit.go).
//   - StorageGuard / StoragePolicy: a boundary-enforcement wrapper that
//     rejects or redacts PII immediately before a write path considers
//     data "at rest" (policy.go).
//   - PIIService: orchestrates the full pipeline -- detect -> classify ->
//     apply jurisdiction rules -> redact/pseudonymize per configured mode
//     -> audit -> return sanitized text + match report (service.go).
//
// Design principles:
//
//   - No ML models by default, but a pluggable extension point. Detector is
//     an interface; RuleBasedDetector is a deterministic, regex/heuristic
//     implementation with no ML/NER dependency, mirroring
//     packages/segmentation and packages/multilingual's "no ML models, rule
//     based" design principle. A future phase can swap in a real NER model
//     by implementing the same Detector interface -- no caller needs to
//     change.
//   - Redaction never leaks raw PII. Every RedactionMode replaces matched
//     text; none ever leaves the original substring in the output text.
//   - Reversibility is explicit and access-controlled. Only
//     ModePseudonymize retains a mapping to the original value, and that
//     mapping can only be revealed through an AccessPolicy. ModeRedact and
//     ModeIrreversibleRedact never write the original value anywhere this
//     package controls.
//   - Governance is jurisdiction-aware. JurisdictionPIIRules lets a
//     jurisdiction with stricter privacy law force stronger handling (e.g.
//     mandatory irreversible redaction) for a given category, without
//     changing the core pipeline.
//
// See doc/pii-governance.md for a detailed design write-up.
package pii
