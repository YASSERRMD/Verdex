# Verdex Legal Ontology

## Overview

`packages/ontology` implements a classification ontology over legal
concepts (e.g. "negligence", "breach of contract", "consent") that rules
and issues in the IRAC reasoning tree link into. It provides jurisdiction
overlays, synonym/alias handling, multilingual labels, and immutable
versioning.

The fixed IRAC schema (Issue/Rule/Fact/Application/Conclusion) was
established in Phase 031 and is never modified afterward. A `Concept` is
**not** one of those five node types, so this package deliberately does
**not** import `packages/graph`: it maintains its own in-memory
`OntologyStore` and links Concepts to `irac.NodeLike` values (rules,
issues, etc.) via a local `ConceptLink` association record, keyed by the
linked node's ID and `irac.NodeType`, rather than by extending the graph
schema.

---

## The Model

```
Concept              — a single ontology node (ID, Name, Description,
                        CategoryCodes, Labels)
Relation             — RelIsA | RelPartOf | RelRelatedTo | RelContradicts
                        between two Concept IDs
JurisdictionOverlay   — per-jurisdiction-code additional/modified Concepts,
                        merged on top of the core seed set
ConceptLink           — Concept <-> irac.NodeLike association, captured
                        by NodeID + irac.NodeType + Confidence
AliasRegistry         — synonym/alias -> canonical Concept.ID resolution
OntologyVersion       — immutable version snapshot (mirrors
                        irac.TreeRevision's shape)
```

## The Pipeline

```
seed core concepts -> apply jurisdiction overlay -> register
  aliases/labels -> link concepts to nodes -> version -> persist
  -> expose query methods
```

`OntologyService.Bootstrap` runs the seed -> overlay -> persist -> version
stages in one call:

```go
svc := ontology.NewOntologyService()

concepts, version, err := svc.Bootstrap(ontology.BootstrapRequest{
    Taxonomy: category.NewDefaultTaxonomy("US-CA"),
    Overlay:  overlay, // optional JurisdictionOverlay
})
```

Aliases, labels, and node links are registered afterward, against the
concepts already persisted by `Bootstrap`:

```go
_ = svc.RegisterAlias("civil:negligence", "carelessness")
_, _ = svc.RegisterLabel("civil:negligence", "ar", "إهمال")
_, _ = svc.LinkConceptToNode("civil:negligence", ruleNode, 0.85)
```

Every `Bootstrap` call records a new `OntologyVersion`: the first call
records version 1 (no parent), and every subsequent call records the
successor of the store's current latest version — mirroring
`packages/irac`'s `TreeRevision` convention that a tree (here, the
ontology) is never mutated in place, only superseded by a new immutable
revision.

## Jurisdiction Overlays

`SeedCoreConcepts` seeds 3-5 representative core legal concepts for each
top-level category (civil, criminal, domestic-violence, consumer, family,
commercial, labor) present in a `category.Taxonomy`. A
`JurisdictionOverlay` then layers jurisdiction-specific concepts on top
via `MergeOverlay`: a Concept in the overlay whose ID matches a core
concept's ID **replaces** it; a Concept with a new ID is **added**. Core
concepts untouched by the overlay pass through unchanged.

## Multilingual Labels

`Concept.Labels` is a `map[string]string` keyed by language code (e.g.
"en", "ar", "ur", "ta"). `Concept.Label(languageCode)` resolves in order:
the requested language, then the English ("en") label, then
`Concept.Name` — a missing translation never blocks rendering a concept.
