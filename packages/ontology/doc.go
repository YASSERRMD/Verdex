// Package ontology implements Verdex's legal ontology layer: a
// classification ontology over legal concepts (e.g. "negligence",
// "breach of contract", "consent") that rules and issues link into, with
// jurisdiction overlays, synonyms, multilingual labels, and versioning.
//
// Core concepts:
//
//   - Concept / Relation: a Concept is a single ontology node — ID, Name,
//     Description, and the packages/category taxonomy CategoryCodes it
//     belongs to. A Relation links two Concepts by ID with a RelationType
//     (RelIsA, RelPartOf, RelRelatedTo, RelContradicts) (concept.go,
//     relation.go).
//   - SeedCoreConcepts: seeds 3-5 representative core legal Concepts for
//     each top-level category.CategoryCode present in a
//     category.Taxonomy (civil, criminal, domestic-violence, consumer,
//     family, commercial, labor) (seed.go).
//   - JurisdictionOverlay / MergeOverlay: per-jurisdiction-code
//     additional or modified Concepts, merged on top of the core seed
//     set without clobbering untouched core concepts (overlay.go).
//   - ConceptLink / LinkConcept: associates a Concept with an
//     irac.NodeLike (a rule or issue node, typically), capturing the
//     linked node's ID and irac.NodeType at link time. This package
//     never imports packages/graph: a Concept is not one of the five
//     fixed IRAC node types (Issue/Rule/Fact/Application/Conclusion,
//     fixed in Phase 031), so linkage is this package's own association
//     record rather than a graph edge (link.go).
//   - AliasRegistry: registers concept synonyms/aliases and resolves
//     them back to a canonical Concept.ID (alias.go).
//   - Concept.Labels / Concept.Label: multilingual display labels keyed
//     by language code (e.g. "en", "ar", "ur", "ta"), with Label
//     falling back to the English label, then Concept.Name, when the
//     requested language is absent (label.go).
//   - OntologyVersion: an immutable ontology snapshot, mirroring
//     packages/irac's TreeRevision shape — VersionNumber, CreatedAt,
//     ParentVersion (version.go).
//   - OntologyStore / InMemoryOntologyStore: the persistence contract
//     and in-memory implementation holding concepts, relations,
//     overlays, links, aliases, and version history, mirroring
//     packages/evidence/store.go's pattern (store.go).
//   - OntologyService: orchestrates the full pipeline — seed core
//     concepts -> apply jurisdiction overlay -> register
//     aliases/labels -> link concepts to nodes -> version -> persist
//     -> expose query methods (service.go).
//
// Design principles:
//
//   - No new IRAC node type. Like packages/application and
//     packages/statute/precedent, this package never extends the fixed
//     IRAC schema. A Concept is linked to existing irac nodes via
//     ConceptLink, never modeled as a graph node itself.
//   - No dependency on packages/graph. This package owns its own
//     in-memory store (OntologyStore) rather than routing Concepts
//     through graph.GraphStore, because the IRAC schema is fixed and
//     never modified by downstream phases.
//   - Data-driven, not hard-coded. Core concepts, overlays, aliases, and
//     labels are all data assembled at runtime, mirroring
//     packages/category's Taxonomy convention of keying by jurisdiction
//     code rather than importing packages/jurisdiction directly.
//
// See doc/legal-ontology.md for a detailed prose write-up.
package ontology
