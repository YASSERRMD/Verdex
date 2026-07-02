// Package vectorindex embeds and indexes the content-bearing leaf nodes of
// an assembled IRAC reasoning tree (packages/irac) for semantic recall.
//
// # Scope
//
// This package sits downstream of packages/graph (tree storage) and
// packages/embedding (embedding generation), and upstream of a future
// hybrid-retrieval phase (Phase 044) that fuses this package's vector
// similarity scores with a graph-traversal score. It does not talk to any
// LLM provider directly: all embeddings are produced through
// packages/embedding's EmbeddingService interface, and all tree content is
// read through packages/graph's GraphStore interface rather than
// duplicating node storage.
//
// # Leaf-node projection
//
// Not every irac.Node is worth indexing for semantic recall. IssueNode and
// ApplicationNode are structural: they exist to connect a reasoning tree
// together, and their Text is typically a paraphrase of the Rule/Fact/
// Conclusion nodes they connect rather than new content. FactNode, RuleNode,
// and ConclusionNode are the tree's leaves in a content sense — the actual
// factual assertions, legal rules/precedent text, and reasoned outcomes a
// downstream retrieval query is looking for. See projection.go and
// doc/vector-index.md for the full rationale.
//
// # Storage model
//
// VectorStore (store.go) is the storage-agnostic contract, mirroring
// packages/graph's GraphStore/InMemoryGraphStore split: InMemoryVectorStore
// (inmemory.go) is a brute-force cosine-similarity reference implementation
// suitable for tests and small deployments, structured so a real ANN
// backend (e.g. pgvector) can implement the same interface later without
// changing any caller.
package vectorindex
