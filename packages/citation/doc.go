// Package citation guarantees every unit of retrieved legal reasoning
// carries a verifiable source: a resolvable citation string, the exact
// irac.SourceSpan(s) it was drawn from, and a verification outcome
// confirming the cited node actually exists in the case's reasoning
// graph.
//
// This package is the anti-hallucination layer sitting downstream of
// packages/hybridretrieval (Phase 044) and packages/adaptiveretrieval
// (Phase 045): every hybridretrieval.Item a retriever surfaces can be
// wrapped into a CitedUnit, resolved to a formatted citation via a
// pluggable Resolver, verified against packages/graph's GraphStore, and
// scored for confidence — without this package importing
// packages/statute or packages/precedent directly (see resolver.go).
//
// The core guarantee: a CitedUnit's Citation text is never trusted
// blindly. Verify (verify.go) confirms the claimed node still exists for
// the claimed case; DetectBroken (broken.go) further distinguishes a
// citation whose target was deleted/moved from one whose source text has
// drifted (staleness) from current node text. Findings (finding.go)
// surface both classes of failure with a Severity a caller can gate on,
// mirroring packages/treevalidation's Finding/severity convention.
//
// See doc/citation-fidelity.md for the full anti-hallucination guarantee,
// the Formatter extension point, and how this package composes with
// hybridretrieval and the future knowledge-layer API (Phase 048).
package citation
