# Localization & i18n (Phase 090)

Full UI and generated-output localization for the Verdex judicial
reasoning platform: externalized strings, locale switching, and
locale-aware dates, numbers, citations, and exported reports across
English, Arabic, Urdu, and Tamil.

## Why this phase exists

Every prior phase's UI copy and generated-report text was hardcoded
English. Phase 090 does not change what the platform says -- the
non-binding disclaimer, case-status labels, report sections all mean
exactly what they meant before -- it changes *how many languages* that
same substance can be rendered in, and lets a user or deployment choose
which one they see it in.

## Composition table

This phase is deliberately scoped narrowly against several existing,
related packages, reusing each rather than duplicating it:

| Prior phase | Package | What it owns | What Phase 090 reuses from it | What Phase 090 does NOT do |
|---|---|---|---|---|
| 023 | `packages/multilingual` | Unicode normalization, script/language detection, transliteration, RTL run detection for **ingested** case content (transcripts, filings) | The RTL posture (Arabic script, covering Arabic+Urdu, is right-to-left) and the four-language set (ar/ur/ta/en) | Does not import it; does not re-run script/bidi detection over compiled-in UI strings (there is no free text to detect -- `Locale.Direction` is a static property of a known locale) |
| 007 | `packages/jurisdiction` | Which languages a jurisdiction/deployment operates in (`Jurisdiction.Languages []string`, ISO 639-1) | The same ISO 639-1 keying convention for `Locale`, so a caller can derive "which locales does this deployment need" from a jurisdiction lookup | Does not import it; does not invent a second jurisdiction-to-language mapping |
| 046 | `packages/citation` | Jurisdiction-appropriate citation *style* (`Formatter`/`Registry`: common-law vs civil-law templates) | Calls through a supplied `citation.Formatter` in `LocalizeCitation`, then applies bidi embedding for RTL target locales | Does not reimplement citation-style templates; does not re-render citation-embedded numerals with locale-aware grouping (not a real legal-citation convention -- see `citation.go`) |
| 073 | `packages/reportexport` | Assembling and rendering a case `Report` to PDF/DOCX/Markdown/text | The *shape* of a report (case title, jurisdiction key, per-issue analysis + citations) via the structurally-compatible `ReportLike` adapter type | Does not import it (would pull in its full `synthesisagent`/`caselifecycle`/`reasoningtrace` dependency graph) |
| 057 | `packages/guardrail` | The only enforcement mechanism for the non-binding disclaimer (`RequireDisclaimer`, `CheckText`) | Coordinates wording substance with `outputDisclaimer` and with `apps/web`'s `Disclaimer.tsx` | Does not add a second disclaimer-enforcement mechanism; `disclaimer.non_binding_*` are additional-language *renderings* of the same warning, not a competing check |
| 006 | `packages/identity` | RBAC (`Role`/`Permission`/`HasPermission`) | The same authenticate -> tenant-match -> permission-check -> mutate -> audit `Engine` discipline | Does not add a new `Permission` constant -- see below |

## What is new here

- **`Catalog`** (`catalog.go`): an in-memory `locale -> key -> string`
  table with real fallback-to-English logic. `Translate` resolves a key
  for a locale, falling back to `FallbackLocale` (English) when the
  target locale lacks the key, and *records the gap* via
  `MissingKeys`/`UntranslatedKeys`/`CoveragePercent` -- a genuine
  translation-management surface, not a silent no-op.
- **`SeedCatalog`** (`seed.go`): real en/ar/ur/ta translations for
  case-status labels (mirroring `apps/web`'s `CASE_STATE_LABELS`
  key-for-key), common UI actions, month names, and the non-binding
  disclaimer.
- **`FormatDate`/`FormatInteger`/`FormatFloat`** (`format.go`):
  locale-aware date/number formatting via `golang.org/x/text`, with
  digit shape pinned to Western/Latin numerals across every locale
  (see below).
- **`LocalizeCitation`** (`citation.go`): composes a
  `citation.Formatter`'s output with locale-aware bidi embedding for
  right-to-left target locales.
- **`ReportLike`/`LocalizeReport`** (`report.go`): a documented adapter
  producing a localized rendering of a report-shaped value, without a
  hard dependency on `packages/reportexport`.
- **`Preference`/`PreferenceRepository`/`Engine`** (`preference.go`,
  `repository.go`, `engine.go`): the one durable, stateful piece of
  this phase -- a per-user, per-tenant locale preference, backed by a
  small Postgres table
  (`packages/persistence/migrations/000040_create_localization.up.sql`).
- **`apps/web` wiring**: a `LocaleProvider`/`useLocale` cookie-backed
  switcher, a `useDirection` hook applying `dir="rtl"` conditionally, an
  externalized strings module, and a locale switcher wired into
  `TopBar`.

## Why no new `identity.Permission`

A locale preference is a self-service user setting, not a privileged
operation. `Engine.SetPreference`/`ClearPreference` authorize via a
simple rule (`access.go`'s `requireSelfOrManage`): an actor may always
set their own preference, and an actor holding the *existing*
`identity.PermManageUsers` (already used for role/status changes) may
set another user's preference too (e.g. an administrator provisioning a
new user's default locale). Adding a dedicated `PermManageLocalization`
constant for this would be the kind of permission-taxonomy sprawl this
phase's brief explicitly cautioned against creating without genuine
need.

## Report localization: why an adapter, not an import

`packages/reportexport.Report` is reachable, but importing it here
would pull in `packages/citation`, `packages/synthesisagent`,
`packages/caselifecycle`, and `packages/reasoningtrace` as transitive
dependencies of what should stay a light, UI-adjacent package. Instead:

- `ReportLike`/`ReportIssueLike`/`ReportCitationInput` are minimal
  structural types describing exactly the fields `LocalizeReport`
  needs.
- `report_test.go` exercises this against a local `stubReport` shaped
  like `reportexport.Report` (same field names, same nesting), proving
  the adapter contract without the import.
- A future phase wiring this directly against `packages/reportexport`
  only needs a one-line-per-field copy from `reportexport.Report`/
  `ReportIssue` into `ReportLike`/`ReportIssueLike` -- no change to this
  package would be required, since those types already carry
  structurally compatible fields (`CaseTitle`, `JurisdictionKey`,
  `Issues[].IssueNodeID/Analysis`, and a citation list per issue).

## Date/number formatting: why not a full CLDR engine

`golang.org/x/text` does not ship a generated CLDR date-pattern engine
as part of its public, non-generated API surface (that lives in
internal tables consumed by tooling this repository does not otherwise
depend on). `FormatDate` instead combines two things this package
already owns: `golang.org/x/text/language` to resolve a BCP-47 tag, and
this package's own `Catalog` for translated month names, plus a small
explicit per-locale field-order table. This produces genuinely
different, locale-correct output per locale (see `format_test.go`),
without a heavyweight new dependency.

## Why numerals stay Western/Latin even in Arabic and Urdu

`golang.org/x/text/message.Printer` defaults an `"ar"`/`"ur"` language
tag to native Eastern Arabic-Indic numeral shapes (`٠١٢٣...`) for
`%d`/`%f`. This package overrides that default via the standard BCP-47
Unicode extension `-u-nu-latn` (`langTagFor` in `format.go`), pinning
every locale to Western/Latin digit shapes. This is a deliberate
domain decision, not an oversight: this package's numbers exist for
legal-citation figures and report/date figures, and UAE/Gulf judicial
and official documents -- this platform's primary jurisdiction focus
per `packages/jurisdiction`'s seed data -- conventionally render such
figures in Western numerals even within Arabic-script legal text,
unlike informal Arabic prose. Locale-appropriate grouping separators
and decimal points are still applied; only the digit *shape* is
pinned.

A related, separate decision: `LocalizeCitation` (`citation.go`)
deliberately does **not** re-render citation-embedded digit runs with
locale-aware thousands-grouping. An earlier version of this function
did, and a test caught the resulting bug directly: rendering `"[2020]
UKSC 1"` as `"[2,020] UKSC 1"`. Comma-grouping a citation year or
docket number is not a real legal-citation convention in any of this
package's four locales, so `LocalizeCitation` leaves the underlying
`citation.Formatter` output's numerals exactly as produced. What
citations genuinely need locale-aware handling for is bidi
directionality: `LocalizeCitation` wraps an inherently Latin-script/LTR
citation string in explicit `LRE...PDF` bidi embedding controls when
the target locale is right-to-left, so it displays correctly within
RTL surrounding prose.

## Frontend inventory (`apps/web`)

- `src/lib/i18n/strings.ts` -- an externalized strings catalog mirroring
  this package's `Catalog` shape, consumed by `StatusActionsBar` and
  `caseLifecycle.ts`'s existing label maps.
- `src/lib/i18n/LocaleContext.tsx` -- `LocaleProvider`/`useLocale`: a
  React context backed by a cookie (`verdex_locale`), so a chosen
  locale survives a reload without a full routing-based i18n framework.
- `src/lib/i18n/useDirection.ts` -- resolves a `Locale` to `'ltr'/'rtl'`
  using the same four-locale table this Go package seeds
  (`SupportedLocales`), so the two never drift apart.
- `src/components/layout/LocaleSwitcher.tsx` -- a small dropdown wired
  into `TopBar`, next to the existing user menu.
- `src/app/layout.tsx` -- applies `dir` on the root `<html>` element via
  `LocaleProvider`.
- `__tests__/LocaleSwitcher.test.tsx` -- a Jest/RTL test covering the
  switcher and the direction-resolution logic.

## Fallback and translation-management workflow

1. `Translate(cat, locale, key, args...)` is the one function almost
   every call site uses. It never errors and never panics.
2. When a key is missing from `locale` but present in `FallbackLocale`,
   the gap is recorded. `cat.MissingKeys(locale)` (observed-at-runtime)
   and `cat.UntranslatedKeys(locale)` (proactive, diffed against the
   full fallback key set) are the two ways a translation-management
   workflow discovers what still needs translating.
3. `cat.CoveragePercent(locale)` gives a compact one-number summary
   alongside the full key lists.
4. `MustTranslate` is the strict variant for build-time tooling: it
   errors (rather than rendering a `!(key)!` placeholder) only when a
   key is missing even from `FallbackLocale` -- an authoring bug, not a
   translation gap.
