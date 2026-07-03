// Package reportexport assembles a case's draft analysis — facts,
// issues, analysis, and citations drawn from packages/synthesisagent's
// Opinion, packages/caselifecycle's Case, and packages/citation's
// jurisdiction-aware formatting — into a structured Report, and renders
// that Report into PDF, DOCX, Markdown, and plain-text bytes for
// download.
//
// Every rendered format carries the same mandatory, non-binding
// disclaimer packages/guardrail enforces elsewhere in the platform
// (see guardrail.RequireDisclaimer), and every Report optionally
// appends a reasoning-trace appendix built from
// packages/reasoningtrace's already-assembled Trace, rather than
// re-deriving any narrative itself.
//
// Exports may optionally be redacted before rendering, reusing
// packages/pii's detection and redaction pipeline (PIIService) instead
// of reimplementing PII detection. Every export — whether redacted or
// not — is recorded in an append-only audit log (see audit.go),
// queryable by case, actor, and time range.
//
// See doc/report-export.md for the full design.
package reportexport
