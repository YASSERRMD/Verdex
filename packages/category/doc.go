// Package category categorizes a case as civil, criminal, domestic
// violence, consumer, family, commercial, labor, or other — per
// jurisdiction — and maps that category to the procedural rules and
// statute partitions applicable to it, so downstream IRAC
// (Issue/Rule/Application/Conclusion) reasoning knows which procedural code
// and which slice of the statute corpus governs a given case.
//
// Core concepts:
//
//   - Category / CategoryCode / Taxonomy: the category taxonomy — a small
//     set of top-level codes (CodeCivil, CodeCriminal,
//     CodeDomesticViolence, CodeConsumer, CodeFamily, CodeCommercial,
//     CodeLabor, CodeOther) plus jurisdiction-specific sub-categories,
//     keyed per jurisdiction because which categories are recognized
//     varies by legal system (taxonomy.go).
//   - Suggestion / Suggester / KeywordSuggester: the pluggable suggestion
//     interface and its default, deterministic implementation, which
//     scores candidate categories by lexical keyword matches against case
//     text (suggest.go).
//   - CategoryAssignment / ManualOverride / ApplyOverride: human-correction
//     support that lets a reviewer's judgment take precedence over the
//     suggestion, with the original suggestion preserved distinctly on
//     CategoryAssignment.Override.Previous rather than being discarded
//     (override.go).
//   - SubCategories / ParentChain / ResolveParent / ValidateSubCategory:
//     sub-category support — resolving and validating a Category's
//     ParentCode chain up to its top-level ancestor (subcategory.go).
//   - ProceduralRuleRef / ProceduralRules: a lookup table mapping a
//     Category to the procedural rule references that govern it, per
//     jurisdiction (procedural.go).
//   - StatutePartitionRef / StatutePartitions: a lookup table mapping a
//     Category to statute partition identifiers, per jurisdiction — a
//     forward-looking hook that packages/statute (Phase 035) will later
//     populate with a real statute corpus (statute_partition.go).
//   - CategoryAuditEvent / AuditSink: an audit trail recording every
//     suggestion, validation, override, and final category change, with
//     timestamp and actor, mirroring packages/intake's audit pattern
//     (audit.go).
//   - ValidateCategory: rejects a Category that is not present in a given
//     jurisdiction's Taxonomy (validate.go).
//   - CategoryService: orchestrates the full pipeline — suggest -> validate
//     against jurisdiction -> apply any override -> map to procedural
//     rules + statute partitions -> audit -> return the final Category
//     assignment (service.go).
//
// Design principles:
//
//   - No ML models. KeywordSuggester is a deterministic function of lexical
//     pattern matching, mirroring packages/evidence and packages/pii's
//     "no ML models, rule based" design principle. A future phase can swap
//     in a real classifier model by implementing the Suggester interface;
//     no caller needs to change.
//   - Categories are data, not a fixed enum. Unlike packages/evidence's
//     EvidenceType (a closed set of Go constants), which categories are
//     valid — and what sub-categories exist beneath them — varies by
//     jurisdiction. Taxonomy is therefore a per-jurisdiction map, seeded
//     from a shared set of top-level codes via NewDefaultTaxonomy but
//     freely extensible per jurisdiction via AddCategory.
//   - Human correction is first-class, not a patch. ManualOverride and
//     ApplyOverride are dedicated types, not a mutation of
//     CategoryAssignment in place — the suggestion-derived determination is
//     always recoverable from CategoryAssignment.Override.Previous, and the
//     full list of Suggestions is always retained on the assignment.
//   - No dependency on packages/statute. StatutePartitionRef is a simple
//     string-keyed reference so this package can be built and tested ahead
//     of packages/statute (Phase 035) landing.
//
// See doc/category-model.md for a detailed model write-up.
package category
