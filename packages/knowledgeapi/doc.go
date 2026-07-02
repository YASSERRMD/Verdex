// Package knowledgeapi is the stable internal facade over Verdex's
// knowledge/retrieval stack: packages/graph (via packages/knowledgeisolation),
// packages/treeindex, packages/traversal, packages/hybridretrieval,
// packages/adaptiveretrieval, packages/citation, and packages/treevalidation.
//
// This is the capstone of Part 4 (Phases 041-048): everything upstream
// builds one retrieval capability each; this package composes all of them
// behind a single KnowledgeAPI entrypoint so that future consumers — Part
// 5's reasoning agents, and any future case-workspace UI — depend on one
// stable, versioned, access-controlled contract instead of reaching into
// treeindex/traversal/hybridretrieval/citation directly.
//
// # Composition, not duplication
//
// KnowledgeAPI reimplements none of the retrieval logic in the packages it
// composes:
//
//   - Every read that touches case-scoped graph or vector data is served
//     through a knowledgeisolation.CaseScopedStore /
//     knowledgeisolation.CaseScopedVectorStore, never a raw
//     graph.GraphStore or vectorindex.VectorStore. This is the enforcement
//     point that makes Phase 047's cross-case isolation guarantee actually
//     bind on every request this package serves, not just on packages that
//     happen to remember to wrap their store.
//   - Tree reads delegate to treeindex.Indexer.LookupPaths /
//     LookupPathsWithDepth for materialized path queries, and to the
//     case-scoped GraphStore's own GetNode/Traverse for node/edge reads.
//   - Hybrid retrieval delegates to hybridretrieval.Retriever.Retrieve.
//     adaptiveretrieval.Builder.Build is available as an alternative
//     execution strategy for callers that want cost-bounded, on-demand
//     subgraph construction instead of a full hybrid fusion pass.
//   - Citation resolution delegates to packages/citation's Resolve,
//     Verify, and DetectBroken functions.
//   - Tree validation status surfaces treevalidation.TreeValidationService.
//     Validate's Report and gate.CanFinalize verdict; the six integrity
//     checks themselves are never re-derived here.
//
// # Access control
//
// Every KnowledgeAPI method takes a context carrying an authenticated
// *identity.User (see identity.UserFromContext) and checks
// identity.Permission via identity.User.HasPermission before invoking any
// underlying store. This is a complementary, independent layer from
// knowledgeisolation's case/tenant boundary: identity/RBAC decides
// "should this actor see any of this at all," while knowledgeisolation
// decides "can this data ever cross a case boundary" — a caller must clear
// both gates. See doc/knowledge-api.md for the full access-control model.
//
// # Contracts
//
// Every method accepts and returns knowledgeapi's own request/response DTO
// types (dto.go), never the underlying packages' internal types directly,
// so this package's wire contract can evolve independently of any one
// retrieval package's internal shape. Responses are versioned and
// enveloped consistently with packages/gateway's conventions
// (gateway.Response[T], gateway.PaginationMeta, gateway.APIError), and
// pagination follows packages/gateway's page/per_page convention
// (packages/gateway's ParsePagination/PaginateSlice).
//
// # No hardcoded LLM provider
//
// knowledgeapi makes no model or embedding provider calls of its own; hybrid
// retrieval callers are expected to supply an already-computed
// embedding.EmbeddingVector, exactly as packages/hybridretrieval requires.
//
// See doc/knowledge-api.md for the full API surface, the DTO contracts,
// the access-control model, and integration guidance for future
// consumers.
package knowledgeapi
