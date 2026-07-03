# Case search

`packages/casesearch` implements Phase 069's cross-case search: finding
cases (and the content within them) across a tenant's entire case
portfolio, rather than the single-case reads every prior phase's packages
provide.

## Why this package exists now

Everything Phase 069 needs already exists as building blocks:
`packages/hybridretrieval` fuses semantic recall with graph traversal for
one case, `packages/treeindex` materializes issue/rule paths for one case,
`packages/knowledgeapi` is the stable per-case facade over both, and
`packages/caselifecycle` persists `Case` metadata with tenant-scoped CRUD.
None of them search *across* cases — `caselifecycle.Repository.List` was
built for "list this tenant's cases matching a filter", not "rank cases by
how well their content matches a query". `casesearch` is the thin
orchestration layer that closes that gap, composing the existing packages
rather than reimplementing retrieval.

## The composition

```
Engine.Search(ctx, tenantID, Query)
  1. caselifecycle.Repository.List(ctx, tenantID, CaseFilter)
     -> tenant-scoped candidate cases, narrowed by category/jurisdiction/state
  2. applyPartyFilter (via PartyLookup)      -> narrows by party name
  3. applyDateFilter                          -> narrows by CreatedAt range
  4. for each candidate case:
       CaseSearcherResolver(ctx, caseID) -> CaseSearcher
       CaseSearcher.Search{Keyword,Semantic,IssueOrRule}(ctx, ..., topK) -> []Hit
  5. rank all (case, best-hit-score) pairs, descending
  6. paginate
```

`Engine` never constructs a `knowledgeapi.KnowledgeAPI` itself.
`knowledgeapi.KnowledgeAPI` is deliberately scoped to exactly one case (its
`hybridretrieval.Retriever` and `treeindex.Indexer` are built once, over
one case's `knowledgeisolation`-scoped store) — a cross-case search
therefore cannot hold a single shared `KnowledgeAPI`. `CaseSearcherResolver`
is the seam: a caller supplies a function from case ID to a `CaseSearcher`
(typically backed by a per-case `KnowledgeAPI`, constructed on demand or
served from a cache), and `Engine` calls it once per candidate case.

`KnowledgeAPISearcher` (`knowledgeapi.go`) is the reference `CaseSearcher`
implementation, adapting a single case's `*knowledgeapi.KnowledgeAPI`:

- `SearchKeyword` walks the case's tree via `GetTree` and matches text as
  a case-insensitive substring — no embeddings, no LLM call.
- `SearchSemantic` embeds the query via a caller-supplied `EmbedFunc` and
  delegates to `KnowledgeAPI.Retrieve` (real `hybridretrieval` fusion).
- `SearchIssueOrRule` delegates to `KnowledgeAPI.LookupPaths` rooted at
  the query's `IssueOrRuleID`, flattening every reachable node into a Hit
  — "find cases where this issue/rule/statute was applied."

## Search modes

`Query.Mode` selects the strategy: `ModeKeyword`, `ModeSemantic`,
`ModeIssueRule`, or the default `ModeAuto`, which resolves to
`ModeIssueRule` when `Query.IssueOrRuleID` is set and `ModeSemantic`
otherwise. A `Query` with neither `Text` nor `IssueOrRuleID` set is
rejected with `ErrEmptyQuery` unless `AllowEmptyText` is set alongside a
non-zero `Filter` — an explicit filter-only "list matching cases" search.

## Filters

`Filter` narrows the candidate case set before any content matching runs:

- `CategoryCode`, `JurisdictionID`, `State` map directly onto
  `caselifecycle.CaseFilter`'s existing fields.
- `PartyName` is resolved via an injected `PartyLookup` function (case ID
  -> party names), since `caselifecycle.CaseFilter` has no party concept
  and this package does not take a hard dependency on
  `packages/timeline`'s construction pattern. **A `Query` with
  `PartyName` set but no `PartyLookup` configured on the `Engine` matches
  zero cases** rather than silently ignoring the filter — see
  `Engine.WithPartyLookup`.
- `DateFrom`/`DateTo` filter on `Case.CreatedAt`, applied in-process after
  `List` (no date range field on `CaseFilter` today).

## Ranking and snippets

Within a case, `CaseSearcher` implementations already sort their own
`Hit`s by descending `Score`. Across cases, `Engine.Search` ranks each
matched `Result` by its best (highest-scoring) `Hit`, breaking ties by
most-recent `CreatedAt` for determinism. Ranking is intentionally *not*
normalized across modes — a `ModeKeyword` coverage score and a
`ModeSemantic` cosine-fusion score are not on a comparable scale, and a
single `Query` only ever runs in one resolved mode, so cross-mode score
comparison never actually happens within one `Search` call.

`ExtractSnippet` (`snippet.go`) builds a short, highlighted excerpt
(`SnippetContextChars` on each side of the match, wrapped in
`SnippetHighlightOpen`/`Close`) from the top hit's text, falling back to a
plain leading excerpt when the query text isn't found verbatim (e.g. a
semantic hit whose match isn't a literal substring).

## Access control

Every `Engine.Search`/`SavedSearchService` method requires an
authenticated `identity.User` on `ctx` holding `identity.PermViewCase`
(`access.go`, mirroring `knowledgeapi`'s own gate exactly) — search is a
read-only view over case knowledge, gated the same as any other case
read.

Tenant isolation is structural, not a filter a caller can forget:
`caselifecycle.Repository.List(ctx, tenantID, ...)` only ever returns
cases belonging to `tenantID` (the same guarantee every other
`caselifecycle.Repository` method makes), so a case belonging to a
different tenant is never even a candidate — it cannot leak through
`Search` regardless of what `Query` is supplied. Saved searches carry the
same tenant scoping through `SavedSearchRepository`, backed by
`saved_searches`' Row-Level Security policy
(`persistence/migrations/000011_enable_rls_saved_searches.up.sql`) for
the Postgres-backed `TenantScopedRepository`, in addition to
`requireMatchingTenant`'s application-level check.

A per-case `CaseSearcherResolver` failure (e.g. a case whose tree has
never been assembled/indexed) does not fail the whole `Search` call — it
is excluded from results and counted in `Results.SkippedCases`, so one
broken case cannot block search across every other case in the tenant.

## Saved searches

`SavedSearch` persists a named `Query` (JSON-encoded in Postgres) per
owner, scoped to a tenant. `SavedSearchService` wraps
`SavedSearchRepository` with ownership scoping: `List` only returns the
authenticated user's own saved searches, and `Run`/`Delete` reject
(`ErrNotFound`) an attempt to act on another user's saved search — even
within the same tenant — mirroring how `packages/signoff`'s
`RequireSignoffPermission`/ownership checks are enforced in the service
layer rather than trusted to callers.

Three `SavedSearchRepository` implementations exist, mirroring
`packages/signoff`'s exact repository-layering convention:
`InMemorySavedSearchRepository` (tests/fixtures), `PostgresRepository`
(accepts a `persistence.Executor`), and `TenantScopedRepository` (wraps
`PostgresRepository` in a `tenancy.WithTenantScope`-scoped transaction —
the type production code should use against a live `*pgxpool.Pool`).

## What this package deliberately does not do

- **No new ranking model.** `ModeKeyword`'s relevance score is a small,
  deterministic coverage heuristic (match-length / text-length), not
  TF-IDF or BM25. `ModeSemantic`'s ranking is whatever
  `hybridretrieval.Retriever` already computes. Building a heavier
  cross-mode ranking model was judged out of scope for v1 — see
  `keywordScore`'s doc comment.
- **No embedding implementation.** `EmbedFunc` is a caller-supplied seam;
  this package has no dependency on any model provider, mirroring how
  `hybridretrieval.HybridQuery` itself takes an already-embedded vector
  rather than raw text.
- **No new party/timeline schema.** `PartyLookup` is a narrow function
  seam a caller adapts from whatever party source (typically
  `packages/timeline`) already exists per case.
