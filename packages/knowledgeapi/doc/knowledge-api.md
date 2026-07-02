# Knowledge Layer API

`packages/knowledgeapi` is the capstone of Part 4 (Phases 041-048): a
single, stable internal contract composing every layer of Verdex's
knowledge/retrieval stack — the case-scoped graph store, `treeindex`,
`traversal`, `hybridretrieval`, `adaptiveretrieval`, `citation`, and
`treevalidation` — behind one `KnowledgeAPI` type. It is the intended
integration point for every future consumer of case knowledge: Part 5's
reasoning agents, and any future case-workspace UI, should depend on this
package instead of importing `treeindex`/`traversal`/`hybridretrieval`/
`citation` directly.

## Composes with, does not duplicate

`knowledgeapi` reimplements none of the retrieval logic in the packages it
composes. Every method is a thin adapter:

| KnowledgeAPI method | Delegates to |
|---|---|
| `GetTree`, `GetNode` | `knowledgeisolation.CaseScopedStore` (`Traverse`, `GetNode`) |
| `LookupPaths` | `treeindex.Indexer.LookupPaths` / `LookupPathsWithDepth` (auto-`RebuildCase` on first use) |
| `Retrieve` | `hybridretrieval.Retriever.Retrieve` |
| `ResolveCitation` | a configured `citation.Resolver`, then `citation.Verify` and `citation.ScoreConfidence` |
| `ValidationStatus` | `treevalidation.TreeValidationService.Validate` and `treevalidation.CanFinalize` |

Every read that touches case-scoped graph or vector data is served through
a `knowledgeisolation.CaseScopedStore` / `knowledgeisolation.
CaseScopedVectorStore` — never a raw `graph.GraphStore` or `vectorindex.
VectorStore`. This is the enforcement point that makes Phase 047's
cross-case isolation guarantee actually bind on every request this
package serves, rather than depending on every future caller remembering
to wrap their own store.

`adaptiveretrieval.Builder` is a valid alternative execution strategy for
a future endpoint wanting cost-bounded, on-demand subgraph construction
instead of a full hybrid fusion pass; this phase wires `hybridretrieval`
as the primary retrieval path and leaves `adaptiveretrieval` composable
by a caller that constructs its own `Builder` over the same case-scoped
store, without knowledgeapi needing its own dedicated endpoint for it yet.

## Primary type: `KnowledgeAPI`

```go
type KnowledgeAPI struct { /* unexported */ }

func NewKnowledgeAPI(
    caseID string,
    store *knowledgeisolation.CaseScopedStore,
    vectorStore *knowledgeisolation.CaseScopedVectorStore,
    indexer *treeindex.Indexer,
    retriever *hybridretrieval.Retriever,
) (*KnowledgeAPI, error)

func (api *KnowledgeAPI) WithCitationResolver(resolver citation.Resolver) *KnowledgeAPI
func (api *KnowledgeAPI) WithValidation(jurisdictionCode string, allowedOverrides []string, confidenceThreshold float64) *KnowledgeAPI
func (api *KnowledgeAPI) CaseID() string

func (api *KnowledgeAPI) GetTree(ctx context.Context, req GetTreeRequest) (GetTreeResponse, error)
func (api *KnowledgeAPI) GetNode(ctx context.Context, req GetNodeRequest) (GetNodeResponse, error)
func (api *KnowledgeAPI) LookupPaths(ctx context.Context, req LookupPathsRequest) (LookupPathsResponse, error)
func (api *KnowledgeAPI) Retrieve(ctx context.Context, req RetrieveRequest) (RetrieveResponse, error)
func (api *KnowledgeAPI) ResolveCitation(ctx context.Context, req ResolveCitationRequest) (ResolveCitationResponse, error)
func (api *KnowledgeAPI) ValidationStatus(ctx context.Context, req ValidationStatusRequest) (ValidationStatusResponse, error)
```

A `KnowledgeAPI` value is scoped to exactly one case: construct one per
case, passing a `CaseScopedStore`/`CaseScopedVectorStore` already scoped
to that case's `CaseID`, plus a `treeindex.Indexer` and `hybridretrieval.
Retriever` built over that same store/vector-store pair. Every request DTO
also carries a `CaseID` field; a request naming a different case than the
instance is scoped to is rejected with `ErrEmptyCaseID` before any store
call, so the case boundary is enforced both structurally (at construction)
and defensively (per request).

## DTO contracts

Every endpoint has its own exported request/response struct in `dto.go`,
distinct from the internal package types they wrap (`irac.Node`,
`treeindex.Path`, `hybridretrieval.Item`, `citation.ResolvedCitation`,
`treevalidation.Finding`, ...). Every response DTO carries a `Version`
field set to `APIVersionV1`, so a future breaking change to this
package's wire contract can be introduced as `APIVersionV2` without
disturbing existing consumers.

List-returning responses (`GetTreeResponse`, `LookupPathsResponse`,
`RetrieveResponse`) carry a `PageMeta` built by `paginate.go`, mirroring
`packages/gateway`'s `PaginationMeta`/`ParsePagination`/`PaginateSlice`
convention exactly: `page`/`per_page` query-parameter semantics, a
default page size of 20, and a 100-item cap.

See `dto_test.go` for JSON round-trip proofs of every response DTO.

## Access-control model

Every `KnowledgeAPI` method calls `authorize(ctx)` (`access.go`) before
touching any store. `authorize` requires an authenticated `*identity.User`
on the context (set by `identity.AuthMiddleware`, or directly via
`identity.WithUser` in tests) holding `identity.PermViewCase`:

```go
func authorize(ctx context.Context) (*identity.User, error)
```

This is a second, complementary layer to Phase 047's case/tenant
isolation:

- **identity/RBAC** (this package's `authorize`) decides *"should this
  actor see any case knowledge at all."* An unauthenticated caller gets
  `ErrUnauthenticated`; an authenticated caller whose roles do not grant
  `PermViewCase` gets `ErrForbidden`.
- **`knowledgeisolation`** (Phase 047, unchanged) decides *"can this
  data ever cross a case boundary."* A caller who clears the RBAC gate
  can still never read another case's private facts — every store call
  underneath `authorize` is scoped through `CaseScopedStore`/
  `CaseScopedVectorStore`, which reject or filter cross-case access
  regardless of the caller's role.

A caller must clear both gates. See `access_test.go` for proof every
exported method enforces the RBAC gate, and `isolation_test.go` for a
regression guard proving the case-isolation gate still holds when
reached through this facade (not just when `CaseScopedStore` is
exercised directly, as Phase 047's own tests already prove).

`http.go`'s `RequirePermissionMiddleware(identity.PermViewCase)` gives an
HTTP caller an earlier, router-level 401/403 short-circuit ahead of this
per-method check — a defense-in-depth convenience, not a replacement for
it: a caller who forgets to wire the middleware does not bypass access
control, since every method's own `authorize` call still runs.

## API versioning and HTTP surface

`knowledgeapi` applies `packages/gateway`'s existing versioning scheme
rather than inventing its own:

```go
func NewRouter() *gateway.Router // pre-configured, mounted under /v1
```

`Handler` (`http.go`) adapts a `KnowledgeAPI` onto `packages/gateway`'s
HTTP conventions: `gateway.Response[T]`/`gateway.ErrorResponse` envelopes,
`gateway.ParsePagination`'s page/per_page query parameters, and
`gateway.APIError` codes mapped from this package's sentinel errors
(`ErrUnauthenticated` → 401, `ErrForbidden`/`knowledgeisolation.
ErrCrossCaseAccess` → 403, `ErrEmptyCaseID`/`ErrEmptyNodeID`/
`ErrInvalidPagination`/`ErrEmptyQuery` → 400).

```go
handler, _ := knowledgeapi.NewHandler(api)
router := knowledgeapi.NewRouter()
handler.Routes(router, knowledgeapi.RequirePermissionMiddleware(identity.PermViewCase))
```

Routes registered: `GET /v1/tree`, `GET /v1/nodes`, `GET /v1/paths`,
`POST /v1/retrieve`, `GET /v1/citations`, `GET /v1/validation-status`
(all relative to the `Handler`'s case-scoped mount point — one `Handler`
serves exactly one case, matching `KnowledgeAPI`'s own per-case
construction).

## Boundary: what this package is not

- It does not implement any retrieval algorithm — vector search, graph
  traversal, path materialization, citation resolution, and tree
  validation all remain the sole responsibility of their respective
  packages.
- It does not replace `knowledgeisolation`'s enforcement; it depends on
  it and would be unsafe to use with a raw, unwrapped `graph.GraphStore`
  or `vectorindex.VectorStore`.
- It does not implement authentication (token issuance/validation) —
  that remains `packages/identity`'s `Provider`/`AuthMiddleware`. This
  package only checks the permission of whatever `*identity.User` a
  caller has already placed on the context.
- It is not a general-purpose case-management API — it exposes read/query
  operations over an already-assembled reasoning tree, not case CRUD,
  filings, or scheduling (see `packages/gateway`'s own doc comment for
  those concerns, which this package does not touch).

Future consumers (Part 5's reasoning agents, a case-workspace UI) should
treat `KnowledgeAPI` as the only supported entrypoint into this stack;
importing `treeindex`, `traversal`, `hybridretrieval`, or `citation`
directly from a new package outside this one is a signal that either a
new `KnowledgeAPI` method is needed, or the new package's concern
actually belongs inside `knowledgeapi` itself.
