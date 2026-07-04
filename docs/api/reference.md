# API reference

This is an index into Verdex's HTTP API surface — it links to the
authoritative specifications rather than re-specifying them. Two
documents own the actual contract:

- [`packages/gateway/doc/api-conventions.md`](../../packages/gateway/doc/api-conventions.md)
  (Phase 009) — the conventions **every** Verdex HTTP API must follow:
  versioning, envelopes, pagination, request IDs, authentication, rate
  limiting, CORS, and security headers.
- [`packages/knowledgeapi/doc/knowledge-api.md`](../../packages/knowledgeapi/doc/knowledge-api.md)
  (Phase 048) — the stable internal facade over the entire knowledge/
  retrieval stack (Part 4 of [`docs/architecture/overview.md`](../architecture/overview.md)).

If you are integrating against Verdex, start with the gateway
conventions below, then use the per-domain reference at the bottom of
this page for the endpoint or API surface you need.

## Gateway conventions (apply to every endpoint)

Full detail: [`packages/gateway/doc/api-conventions.md`](../../packages/gateway/doc/api-conventions.md).

- **Versioning** — every endpoint is served under `/v{major}/resource`
  (e.g. `/v1/cases`). New optional response fields do not bump the
  version; consumers must ignore unknown fields.
- **Response envelope** — every successful response is
  `{ version, status: "success", data, meta?, request_id? }`.
- **Error envelope** — every error response is
  `{ version, status: "error", code, message, details?, request_id? }`,
  with `code` one of `BAD_REQUEST` (400), `UNAUTHORIZED` (401),
  `FORBIDDEN` (403), `NOT_FOUND` (404), `CONFLICT` (409),
  `TOO_MANY_REQUESTS` (429), or `INTERNAL_ERROR` (500).
- **Pagination** — offset-based via `page` (default 1) and `per_page`
  (default 20, max 100) query parameters, with a `meta` object
  (`page`, `per_page`, `total`, `total_pages`) in the response.
- **Request ID** — every request should carry `X-Request-ID`;
  `RequestIDMiddleware` generates one if absent, echoes it in the
  response header, and includes it in every response envelope's
  `request_id` field.
- **Authentication** — every endpoint except `/health` requires a
  Bearer JWT (`Authorization: Bearer <token>`); a missing or invalid
  token returns `401 UNAUTHORIZED`.
- **Rate limiting** — 60 requests/minute/IP by default (`429
  TOO_MANY_REQUESTS` beyond that); overridable per route with a custom
  key function (e.g. keyed by tenant ID).
- **CORS and security headers** — configured at service startup; see
  the gateway doc for the default allowed-header list and preflight
  handling.

## The knowledge-layer API (`packages/knowledgeapi`)

Full detail: [`packages/knowledgeapi/doc/knowledge-api.md`](../../packages/knowledgeapi/doc/knowledge-api.md).

`KnowledgeAPI` is a single, stable contract composing every layer of
Part 4 of the architecture (graph store, `treeindex`, `traversal`,
`hybridretrieval`, `adaptiveretrieval`, `citation`, `treevalidation`)
behind one type, scoped to exactly one case per instance:

| Method | Purpose |
|---|---|
| `GetTree` | Fetch a case's assembled IRAC reasoning tree. |
| `GetNode` | Fetch a single tree node by ID. |
| `LookupPaths` | Structured path lookups over the tree (via `treeindex`). |
| `Retrieve` | Fused graph + vector retrieval (via `hybridretrieval`). |
| `ResolveCitation` | Resolve and verify a citation, with a confidence score. |
| `ValidationStatus` | The tree's current integrity/validation status (via `treevalidation`). |

Every request DTO carries a `CaseID`; a request naming a different case
than the `KnowledgeAPI` instance is scoped to is rejected with
`ErrEmptyCaseID` before any store call — the case boundary is enforced
both at construction and per request. Every response DTO carries a
`Version` field (`APIVersionV1`) so a future breaking change ships as
`APIVersionV2` without disturbing existing consumers. List-returning
responses paginate using the exact same `PageMeta`/`page`/`per_page`
convention as the gateway's own pagination rules above.

**Access control is two layers, both must clear:** every method first
requires an authenticated caller holding `identity.PermViewCase`
(`ErrUnauthenticated`/`ErrForbidden` otherwise) — this decides *whether
this actor sees any case knowledge at all*. Independently,
`packages/knowledgeisolation` (Phase 047) decides *whether the
requested data can ever cross a case boundary* — a caller who clears
the RBAC gate still can never read another case's private facts, since
every store call is scoped through `CaseScopedStore`/
`CaseScopedVectorStore` regardless of role.

## Other domain surfaces

The full case-lifecycle and workspace HTTP surface (case CRUD, evidence
review, sign-off, annotations, search, case history) is documented per
package and per UI panel rather than re-specified here:

| Domain | Reference |
|---|---|
| Case lifecycle | [`packages/caselifecycle`](../../packages/caselifecycle) |
| Sign-off | [`packages/signoff/doc/signoff-workflow.md`](../../packages/signoff/doc/signoff-workflow.md) |
| Case search | [`packages/casesearch/doc/case-search.md`](../../packages/casesearch/doc/case-search.md) |
| Annotations | [`packages/annotations/doc/annotations.md`](../../packages/annotations/doc/annotations.md) |
| Case versioning | [`packages/caseversioning/doc/case-versioning.md`](../../packages/caseversioning/doc/case-versioning.md) |
| First-run setup wizard | [`packages/setup/doc/setup-api-contract.md`](../../packages/setup/doc/setup-api-contract.md) |
| Jurisdiction registry | [`packages/jurisdiction/doc/jurisdiction-schema.md`](../../packages/jurisdiction/doc/jurisdiction-schema.md) |
| Report export | [`packages/reportexport/doc/report-export.md`](../../packages/reportexport/doc/report-export.md) |

For the UI clients built against these APIs, see
[`apps/web/docs/`](../../apps/web/docs) and
[`docs/user-guide/judges-advocates.md`](../user-guide/judges-advocates.md).
