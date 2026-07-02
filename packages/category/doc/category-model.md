# Verdex Case Category Model

## Overview

`packages/category` categorizes a case as civil, criminal, domestic
violence, consumer, family, commercial, labor, or other, **per
jurisdiction**, and maps that category to the procedural rules and statute
partitions applicable to it — so downstream IRAC
(Issue/Rule/Application/Conclusion) reasoning knows which procedural code
and which slice of the statute corpus governs a given case.

Like `packages/evidence`'s classification and `packages/pii`'s detection,
the default suggestion engine in this package is deterministic and
lexical/pattern-based. No component depends on a machine-learning model at
runtime — but suggestion is designed as a pluggable extension point so a
real model can be swapped in later without touching any caller.

The key difference from `packages/evidence`'s `EvidenceType` is that a case
category is not a small, fixed, jurisdiction-independent Go enum: **which
categories (and sub-categories) are valid is data, scoped per
jurisdiction**, because legal systems differ in how they carve up case
types. `Taxonomy` reflects this directly.

---

## The Category Taxonomy

```go
type CategoryCode string

const (
    CodeCivil            CategoryCode = "civil"
    CodeCriminal         CategoryCode = "criminal"
    CodeDomesticViolence CategoryCode = "domestic-violence"
    CodeConsumer         CategoryCode = "consumer"
    CodeFamily           CategoryCode = "family"
    CodeCommercial       CategoryCode = "commercial"
    CodeLabor            CategoryCode = "labor"
    CodeOther            CategoryCode = "other"
)

type Category struct {
    Code       CategoryCode
    Name       string
    ParentCode CategoryCode // empty for top-level categories
}

type Taxonomy map[string]map[CategoryCode]Category // jurisdictionCode -> code -> Category
```

`DefaultTopLevelCategories()` returns the eight top-level categories above.
`NewDefaultTaxonomy(jurisdictionCodes...)` seeds a `Taxonomy` with that
default set for each given jurisdiction. Jurisdictions are free to add
sub-categories (or entirely custom top-level categories) via
`Taxonomy.AddCategory`, which rejects a sub-category whose `ParentCode`
does not already resolve within that jurisdiction (`ErrUnknownParent`).

---

## Suggestion: A Pluggable Interface

```go
type Suggester interface {
    Suggest(ctx context.Context, text string, taxonomy Taxonomy) ([]Suggestion, error)
}

type Suggestion struct {
    Category   Category
    Confidence float64 // [0, 1]
}
```

`KeywordSuggester` is the default implementation. It scores every
top-level category present anywhere in the supplied `Taxonomy` by counting
lexical keyword matches against the input text (e.g. "breach of contract"
for civil, "the accused" / "penal code" for criminal, "protective order"
for domestic violence), converts the match count into a bounded confidence
score (`0.5` for a single match, `+0.15` per additional match, capped at
`0.95`), and returns the candidates sorted by descending confidence.
Suggestions are scoped strictly to categories the given `Taxonomy`
recognizes, so a jurisdiction that has not registered a given category will
never receive a suggestion for it.

A confidence of `1.0` is reserved for a human `ManualOverride` — the
keyword heuristic never claims full certainty.

---

## Sub-Categories

```go
func SubCategories(taxonomy Taxonomy, jurisdictionCode string, parent Category) []Category
func ParentChain(taxonomy Taxonomy, jurisdictionCode string, cat Category) ([]Category, error)
func ResolveParent(taxonomy Taxonomy, jurisdictionCode string, cat Category) (parent Category, ok bool, err error)
func ValidateSubCategory(taxonomy Taxonomy, jurisdictionCode string, cat Category) error
```

A `Category` may be a sub-category of another by setting `ParentCode`.
`ParentChain` walks from a category up through each `ParentCode` to its
top-level ancestor, returning `ErrUnknownParent` if any link does not
resolve (including a cycle — the walk is bounded to 32 levels as a safety
net). `SubCategories` returns the direct children of a given category
within a jurisdiction.

---

## Manual Override

```go
type CategoryAssignment struct {
    CaseID      string
    Category    Category
    Confidence  float64
    Suggestions []Suggestion
    Override    *ManualOverride
}

type ManualOverride struct {
    CaseID     string
    Category   Category
    Reason     string
    ReviewedBy string
    ReviewedAt time.Time
    Previous   *CategoryAssignment
}

func ApplyOverride(original CategoryAssignment, override ManualOverride) (CategoryAssignment, error)
```

A human reviewer's correction always takes precedence over the suggested
category. `ApplyOverride` does not mutate `original` in place: it returns a
new `CategoryAssignment` with `Confidence` set to `1.0` and an `Override`
field that records the correction *and* a snapshot of the pre-override
determination (`Override.Previous`) — so the suggestion is never silently
discarded, only superseded, and both remain inspectable for audit.
`original.Suggestions` is carried forward unchanged onto the overridden
assignment, so the full suggestion list and the override both remain
retained side by side.

---

## Procedural Rules and Statute Partitions

```go
type ProceduralRuleRef struct {
    Code        string
    Name        string
    Description string
}

type StatutePartitionRef struct {
    PartitionID string
    Description string
}
```

`ProceduralRules` and `StatutePartitions` are per-jurisdiction lookup
tables mapping a `CategoryCode` to the procedural rule references and
statute partition references applicable to it, respectively. Both mirror
the same `Register`/`Lookup`/`LookupCategory` shape and default to empty
(a fresh table returns `nil` for every lookup until populated).

`StatutePartitionRef` is deliberately a simple string-keyed reference with
**no dependency on `packages/statute`**. Phase 035 will introduce
`packages/statute` and own the real statute corpus; until then, this
package only stores and looks up partition identifiers as opaque strings,
so `packages/category` can be built and tested independently.

---

## Category Change Audit

```go
type CategoryAuditEvent struct {
    EventType        string // category.suggested | category.overridden | category.changed | category.validated
    CaseID           string
    JurisdictionCode string
    CategoryCode     CategoryCode
    Actor            string
    Confidence       float64
    Timestamp        time.Time
}

type AuditSink interface {
    Emit(ctx context.Context, event CategoryAuditEvent) error
}
```

Mirroring `packages/intake`'s `IntakeAuditEvent`/`AuditSink` pattern, every
significant transition — a suggestion being produced, a category being
validated against a jurisdiction, an override being applied, or the final
category changing — is recorded as a `CategoryAuditEvent`.
`NoOpAuditSink` discards events (the default), `LoggingAuditSink` writes
them to `slog`, and `CapturingAuditSink` retains them in memory for test
assertions on audit-trail completeness.

---

## Validation Against Jurisdiction

```go
func ValidateCategory(jurisdictionCode string, category Category, taxonomy Taxonomy) error
```

`ValidateCategory` rejects a `Category` that is not present — exactly, by
code, name, and parent — in `jurisdictionCode`'s entry within `taxonomy`
(`ErrCategoryNotInJurisdiction`), that targets an unknown jurisdiction
(`ErrUnknownJurisdiction`), or whose sub-category parent chain does not
resolve (`ErrUnknownParent`).

---

## The Pipeline: CategoryService

```go
svc := category.NewCategoryService()
result, err := svc.Categorize(ctx, category.CategorizeRequest{
    CaseID:           "case-123",
    JurisdictionCode: "IN",
    Text:             caseText,
    Taxonomy:         taxonomy,
    Override:         override, // *ManualOverride, optional
    Actor:            "user-42",
})
// result.Assignment, result.ProceduralRules, result.StatutePartitions
```

`CategoryService.Categorize` runs, for a single case: suggest candidate
categories from `Text` (via `Suggester`) → validate the top-ranked
suggestion against the jurisdiction's `Taxonomy` → apply `Override` if
supplied (itself validated against the taxonomy first) → map the final
category to `ProceduralRules` and `StatutePartitions` for the jurisdiction
→ emit a `CategoryAuditEvent` for every step → return the
`CategoryResult`.

This mirrors `packages/evidence`'s `EvidenceService` orchestration
pattern: a single entry point wiring together this package's otherwise
independent, individually testable building blocks.
