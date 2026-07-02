// Package hybridretrieval fuses packages/vectorindex's semantic recall with
// packages/traversal's graph expansion into a single ranked retrieval
// result.
//
// This package is a pure orchestration/fusion layer: it does not
// re-implement vector search or graph traversal, it composes the two
// existing packages. Given a semantic query (already embedded into a
// vector) and optional structural anchors, it runs a vectorindex.VectorStore
// query for semantic candidates, expands the strongest candidates via a
// traversal.Walker to pull in structurally-connected nodes (e.g. a fact's
// governing rule chain), and reciprocal-rank-fuses the two ranked lists into
// one explainable, deduplicated Result. See doc/hybrid-retrieval.md for the
// fusion algorithm, tunables, and composition boundary in full detail.
package hybridretrieval
