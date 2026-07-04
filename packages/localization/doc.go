// Package localization is Phase 090: full UI and generated-output
// localization for the Verdex judicial reasoning platform — externalized
// strings, locale switching, and locale-aware dates, numbers, citations,
// and exported reports across English, Arabic, Urdu, and Tamil.
//
// This phase is deliberately scoped differently from three existing,
// related packages, and composes with each rather than duplicating any
// of them:
//
//   - packages/multilingual (Phase 023) normalizes and detects the
//     script/language of INGESTED case content (transcripts, filings) —
//     content the platform did not author. This package instead
//     localizes OUTPUT the platform itself renders: UI copy, generated
//     reports, dates, numbers, citations. Different concern, same
//     four-language set. This package reuses
//     multilingual.IsRTLScript/DetectScript's RTL-detection posture by
//     convention (see Direction in types.go) rather than reimplementing
//     bidi/script detection — it does not import packages/multilingual,
//     because a compiled-in UI string catalogue has no free-text script
//     to detect; Locale.Direction is a static property of a known
//     locale, not a runtime classification of arbitrary text.
//   - packages/jurisdiction (Phase 007) already records which languages
//     a deployment's jurisdictions operate in (Jurisdiction.Languages).
//     This package's Catalog is deliberately keyed by the same ISO
//     639-1 codes jurisdiction.Jurisdiction.Languages already uses
//     ("ar", "ur", "ta", "en", ...), so a caller can derive "which
//     locales does this deployment need" from
//     packages/jurisdiction.Lookup results and feed that set into
//     Catalog.Translate — without this package importing
//     packages/jurisdiction or inventing a second
//     jurisdiction-to-language mapping.
//   - packages/citation (Phase 046) already owns jurisdiction-aware
//     citation *formatting* (citation.Formatter/citation.Registry:
//     common-law vs civil-law citation style). This package's
//     LocalizeCitation (citation.go) composes that Formatter output
//     with locale-aware bidi handling (wrapping an inherently
//     LTR/Latin-script citation string in explicit bidi embedding
//     controls when the target locale is right-to-left) rather than
//     re-implementing citation-style logic — see citation.go's doc
//     comment for exactly which half each package owns, and why this
//     package deliberately does NOT re-render citation-embedded digits
//     with locale-aware thousands-grouping (not a real legal-citation
//     convention).
//   - packages/reportexport (Phase 073) already assembles a case's
//     Report and renders it to PDF/DOCX/Markdown/text. This package
//     does not import packages/reportexport (see report.go's doc
//     comment for why: it would pull reportexport's full
//     citation/synthesisagent/caselifecycle/reasoningtrace dependency
//     graph into what should stay a light, UI-adjacent package).
//     Instead, report.go defines ReportLike — a minimal structural
//     interface any report-shaped value can satisfy — plus
//     LocalizeReport, a documented adapter that translates section
//     labels and formats embedded dates/numbers/citations through this
//     package's own Catalog/Formatter, unit-tested against a local
//     stand-in struct shaped like reportexport.Report. A future phase
//     wiring this directly into reportexport only needs to make
//     reportexport.Report satisfy ReportLike (it already has the
//     fields); no change to this package would be required.
//
// # What is new here
//
//   - Locale / Direction (types.go): the four seeded locales (en, ar,
//     ur, ta) plus an open Direction (LTR/RTL) per locale — the Go-side
//     half of task 3's RTL support.
//   - Catalog / Translate / MissingKeys (catalog.go): an in-memory
//     locale -> key -> translated-string table with real
//     fallback-to-English logic (task 8) and a translation-management
//     gap report (task 7) — see catalog.go's doc comment for the exact
//     fallback and gap-recording semantics.
//   - SeedCatalog (seed.go): real en/ar/ur/ta translations (task 2) for
//     case-status labels (mirroring apps/web's CASE_STATE_LABELS),
//     common UI actions, and the non-binding disclaimer sentence —
//     coordinated with, never contradicting, packages/guardrail's
//     English wording (see seed.go's doc comment on that coordination).
//   - FormatDate / FormatNumber / FormatInteger (format.go): locale-aware
//     date and number formatting (task 4) via golang.org/x/text
//     (message + language), already a transitive dependency across
//     this platform's other packages. Digit shape is pinned to
//     Western/Latin numerals for every locale (see format.go's doc
//     comment on why Arabic-Indic numeral shapes are wrong for this
//     platform's UAE/Gulf judicial-document domain).
//   - LocalizeCitation (citation.go): task 4's citation half — composes
//     a citation.Formatter's jurisdiction-appropriate text with
//     locale-aware bidi embedding for right-to-left target locales.
//   - ReportLike / ReportSection / LocalizeReport (report.go): task 5 —
//     a localized rendering of a report-shaped value's section labels,
//     dates, and citation list.
//   - Preference / PreferenceRepository / InMemoryPreferenceRepository /
//     PostgresPreferenceRepository (preference.go, repository.go): a
//     durable, tenant-scoped per-user locale preference — the one piece
//     of this phase that is genuinely stateful (task 6's server-side
//     half). See migrations/000036_create_localization.up.sql.
//   - Engine (engine.go): ties Catalog, the preference store, and
//     AuditSink together behind the identity permission/tenant-scoping
//     discipline every other packages/* Engine in this codebase follows.
//   - AuditSink (audit.go): records every preference change through
//     packages/auditlog.Store — no second audit table.
//   - apps/web wiring: LocaleProvider/useLocale (a context + cookie-
//     backed switcher, task 6), a useDirection hook applying dir="rtl"
//     conditionally (task 3's frontend half), and an externalized
//     strings module consumed by a representative component (task 1).
//     See doc/localization.md and apps/web/src/lib/i18n/README for the
//     full frontend inventory.
//
// # What is explicitly reused, not duplicated
//
//   - packages/multilingual's RTL/script-detection posture and
//     four-language set (Arabic, Urdu, Tamil, English) are the reference
//     this package's Direction/Locale follow; this package does not
//     import packages/multilingual and does not re-run script detection
//     over compiled-in UI strings.
//   - packages/jurisdiction's per-jurisdiction Languages list remains
//     the only jurisdiction-to-language mapping in this codebase; this
//     package's Catalog is keyed compatibly (same ISO 639-1 codes) but
//     does not import packages/jurisdiction or re-derive that mapping.
//   - packages/citation's Formatter/Registry remain the only
//     jurisdiction-aware citation-style logic in this codebase;
//     LocalizeCitation calls through a supplied citation.Formatter
//     rather than re-implementing common-law/civil-law citation
//     templates.
//   - packages/guardrail's RequireDisclaimer/outputDisclaimer remain the
//     only English non-binding-disclaimer text and enforcement
//     mechanism in this codebase; this package's seeded
//     "disclaimer.non_binding" translations are additional-language
//     renderings of the same substantive warning, reviewed for
//     consistency with guardrail's wording, not a competing or
//     independently-enforced disclaimer.
//   - identity.Role / identity.Permission / identity.HasPermission
//     (Phase 006) remain the coarse RBAC gate Engine methods call
//     through; this phase does not add a new Permission constant
//     (locale preference is a self-service user setting an
//     authenticated actor sets for themselves, not a privileged
//     operation gated behind a new capability — see access.go).
//   - packages/auditlog.Store is the only durable event sink this
//     package writes to, via AuditSink.
//
// See doc/localization.md for the full write-up, including the
// composition table and the apps/web frontend inventory.
package localization
