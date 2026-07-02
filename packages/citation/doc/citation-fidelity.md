# Citation fidelity layer (`packages/citation`)

Phase 046 gives Verdex a guarantee that every unit of retrieved legal
reasoning carries a verifiable source: a resolvable citation string, the
exact `irac.SourceSpan`(s) it traces back to in ingested source text, and
an independent verification outcome confirming the cited node actually
exists in the case's reasoning graph. This document explains the
anti-hallucination guarantee, the `Formatter` extension point, and how
this package composes with `packages/hybridretrieval` (Phase 044) and the
future knowledge-layer API (Phase 048).

## The anti-hallucination guarantee

A retrieval or reasoning pipeline that hands a user (or a downstream
agent) a citation string is making an implicit claim: "this text really
exists, at this location, in this case's record." Nothing about a
formatted citation string on its own proves that claim — an LLM-based
extraction or reasoning step can produce plausible-looking citations to
rules or precedents that were never actually part of the case.

This package's guarantee is: **a `CitedUnit`'s `Citation` text is never
trusted blindly.** Every citation this package produces or accepts can be
independently checked against the case's `graph.GraphStore`:

1. **Attach a span, not just text.** `CitedUnit` (`unit.go`) wraps a
   retrieved node reference with its `irac.SourceSpan`(s) — the rune-
   offset (or OCR page / STT millisecond) range in the *original ingested
   source* the claim was drawn from, not just the extracted text. `FromItem`
   and `FromItems` build a `CitedUnit` directly from a
   `hybridretrieval.Item`, so every item in a `hybridretrieval.Result` can
   be turned into a citable unit in one call.

2. **Resolve without hallucination risk.** `Resolve` (`resolver.go`)
   fetches the unit's underlying node via `graph.GraphStore.GetNode` —
   it never invents a node ID or trusts caller-supplied text about a node
   without checking the store first — and only then runs the pluggable
   `Resolver` function to produce citation text.

3. **Verify against the graph.** `Verify` (`verify.go`) is the core
   anti-hallucination check: given a `CitedUnit`, it confirms the node
   really exists in the `GraphStore` *under the claimed case*. Three
   outcomes are distinguished:

   | Status | Meaning |
   |---|---|
   | `StatusVerified` | The node exists, under the claimed case. Safe to present. |
   | `StatusHallucinated` | The node does not exist anywhere. The citation was never real. |
   | `StatusWrongCase` | The node exists, but under a *different* case — a cross-case citation leak. |

4. **Distinguish broken from hallucinated.** `DetectBroken`
   (`broken.go`) recognizes that "missing" is not always "never existed."
   Given caller-supplied `KnownNodeIDs` (evidence a node previously
   passed verification, e.g. from a prior `Verify` pass or a
   `Repository` snapshot), a now-missing node is classified
   `BrokenReasonDeleted` (it was real, then removed — e.g. by
   `GraphStore.DeleteTree` or a tree revision that dropped it) rather than
   folded into the same bucket as a citation that was fabricated outright.
   `DetectBroken` also catches `BrokenReasonStale`: the node still exists,
   but its current `Text` no longer matches what the citation/spans were
   built from — the source has since been edited or re-extracted, so the
   quoted span can no longer be trusted to point at the same claim.

5. **Flag, don't just discard.** `Finding`/`Severity`/`Report`
   (`finding.go`) mirror `packages/treevalidation`'s convention exactly:
   `FindingsFromVerification` and `FindingsFromBroken` translate a
   verification or broken-check outcome into zero or one `Finding`, scored
   `SeverityCritical` for anything that undermines trust in the citation
   (hallucinated, wrong-case, deleted) and `SeverityWarning` for staleness
   or missing metadata (`FindingsFromUnit`). A `Report` aggregates
   `Finding`s across a whole retrieval batch, exactly as
   `treevalidation.Report` aggregates tree-validation findings.

6. **Score confidence honestly.** `ScoreConfidenceWith` (`confidence.go`)
   combines the node's own extraction confidence, the `Resolver`'s
   `Certainty` (exact structured match vs. heuristic fallback vs. none),
   and the `Verify` outcome into one `[0, 1]` score — as a **product**, not
   an average. This is deliberate: a highly-confident node extraction must
   never mask a hallucinated or unverifiable citation. The moment any one
   factor is zero (unresolved citation, or failed verification), the
   combined score collapses to zero.

## The `Formatter` extension point

Citation *formatting* is jurisdiction- and legal-tradition-specific:
common-law systems cite decided cases ("Smith v Jones [2020] UKSC 1"),
civil-law systems cite enacted articles ("Art. 5, Code Civil"). This
package does not hardcode either convention into `CitedUnit` — it exposes
a pluggable `Formatter` interface and a concurrency-safe `Registry` keyed
by an **opaque** jurisdiction or legal-family string, mirroring how
`irac.RuleNode.JurisdictionCode`/`LegalFamily` are themselves opaque
strings with no hard dependency on `packages/jurisdiction`:

```go
registry := citation.NewDefaultRegistry() // "common_law", "civil_law" pre-registered
registry.Register("sharia_law", myCustomFormatter)

text, err := registry.Format(ruleNode.LegalFamily, citation.FormatInput{
    Act:     "Act 12",
    Section: "5",
    Clause:  "a",
    Origin:  citation.OriginStatute,
})
```

Two concrete formatters ship as examples:

- `CommonLawFormatter` — case-citation style for precedents
  (`"<CaseName> <RawCitation>"`) and `packages/statute`-style section
  citations for statutes (`"<Act>, s.<Section>(<Clause>)"`).
- `CivilLawFormatter` — article-citation style for statutes
  (`"Art. <Section> <Act>"`) and a comma-joined case reference for
  precedents.

A caller can register any number of additional keys (specific
jurisdiction codes, not just legal families) and set a `WithFallback`
formatter for anything unrecognized.

## Composes with, does not duplicate

| Package | Owns | This package's relationship |
|---|---|---|
| `packages/hybridretrieval` | `Item`/`Result`, fused vector+graph retrieval. | Read-only consumer: `FromItem`/`FromItems` build `CitedUnit`s from `hybridretrieval.Item` values. This package never re-ranks or re-fuses retrieval results. |
| `packages/graph` | `GraphStore`/`InMemoryGraphStore`, node persistence. | Read-only consumer via `GraphStore.GetNode`, used by both `Resolve` and `Verify`/`DetectBroken`. This package never writes to a `GraphStore`. |
| `packages/irac` | `Node`/`NodeType`/`SourceSpan`/`Spans`. | `CitedUnit` embeds `irac.Spans` directly and copies `irac.Node.Text`/`Type` at resolution time; no new span/node type is invented. |
| `packages/statute`, `packages/precedent` | Structured `Citation`/`PrecedentRule` types with formatted citation text. | **Not imported.** A caller that already has statute/precedent output supplies a `Resolver` (e.g. via `LookupResolver` with a `map[string]ResolvedCitation` built from `statute.Citation.String()` or `precedent.PrecedentRule.Citation`) — the same decoupling pattern `packages/traversal.PrecedentResolver` and `packages/application.OriginatedRule` already use to avoid a hard dependency on either package. |
| `packages/treevalidation` | `Finding`/`Severity`/`Report` for whole-tree structural validation. | Not imported; this package redeclares an equivalent `Finding`/`Severity`/`Report` shape scoped to citation-specific checks, matching the convention without adding a cross-package dependency for a three-value enum. |

## Composing with the future knowledge-layer API (Phase 048)

Phase 048's planned "knowledge-layer service interface" is expected to
expose a citation-resolution endpoint over exactly the primitives this
package provides: given a node ID (or a `hybridretrieval.Item` returned
from its own hybrid-retrieval endpoint), resolve, verify, and return a
`CitedUnit` plus its `VerificationResult`/`Finding`s and confidence score
in one response. Concretely, a Phase 048 handler is expected to:

1. Call `hybridretrieval.Retriever.Retrieve` for the query.
2. Call `citation.FromItems` to wrap every `Item`.
3. Call `citation.ResolveAll` with a `Resolver` backed by whatever
   statute/precedent lookup the deployment maintains.
4. Call `citation.VerifyAll` (and, where prior-existence evidence is
   available, `citation.DetectBrokenAll`) to flag any hallucinated,
   wrong-case, or broken citations before they reach a caller.
5. Persist the resolved `CitedUnit`s via a `citation.Repository` for
   later audit, and return the `CitedUnit`s plus their `Finding`s and
   `Confidence` scores as the endpoint's response payload.

No part of this package assumes an HTTP/RPC transport or any particular
service framework — Phase 048 is expected to be a thin adapter over the
functions and types documented here.
