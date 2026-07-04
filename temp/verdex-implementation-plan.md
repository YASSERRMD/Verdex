# Verdex — Phased Implementation Plan

**Project:** Verdex — Jurisdiction-Aware Judicial Reasoning & Case Analysis Platform
**Nature:** Model-agnostic legal reasoning engine producing non-binding reasoned opinions
**Architecture core:** Transcribe-and-discard ingestion → IRAC reasoning tree → hybrid graph + vector retrieval → adversarial synthesis → human-signed draft opinion

---

## Engineering Conventions (apply to every phase)

- **Git identity:** username `YASSERRMD`, email `arafath.yasser@gmail.com`
- **Branching:** one branch per phase, named `phase-NNN-short-slug` (e.g. `phase-007-jurisdiction-loader`)
- **Commits:** minimum 10 atomic commits per phase, each a single logical change with an imperative message
- **Merging:** no squash merges, no direct push to `main`, PR per phase
- **Non-binding guardrail:** every reasoning output is labelled a draft analysis, never a verdict; this constraint is enforced in code, not just documentation
- **Provenance:** original ingest artifacts are hashed before discard; the hash and chain-of-custody metadata persist
- **Model-agnostic rule:** no phase may hardcode a provider; all model calls route through the provider abstraction layer (Phase 11 onward)

> Each phase lists its branch, goal, and a representative atomic commit sequence. Where a phase shows fewer than 10 sample commits, expand each sample into smaller atomic steps to reach the 10-commit minimum (e.g. split "implement X" into scaffold, types, core logic, error paths, tests, docs).

---

## Part 1 — Foundation & Provisioning (Phases 1–10)

### Phase 001 — Repository bootstrap
**Branch:** `phase-001-repo-bootstrap`
**Goal:** Establish monorepo skeleton, tooling, and CI gate.
1. Initialise repository and base `.gitignore`
2. Add `README.md` with project charter and non-binding disclaimer
3. Add `LICENSE` and `CONTRIBUTING.md`
4. Define monorepo layout (`/services`, `/packages`, `/infra`, `/docs`)
5. Add language toolchain config (Go modules, Rust workspace, TS config)
6. Add `Makefile` / task runner entrypoints
7. Configure linters and formatters
8. Add pre-commit hooks
9. Add CI pipeline skeleton (lint + build gate)
10. Add CODEOWNERS and PR template enforcing no-squash policy

### Phase 002 — Configuration & secrets framework
**Branch:** `phase-002-config-framework`
**Goal:** Typed, layered configuration with no secrets in code.
1. Define config schema struct and defaults
2. Add environment variable loader
3. Add file-based config layer (YAML)
4. Add config precedence and merge logic
5. Add secret reference resolver (env / vault placeholder)
6. Add config validation on startup
7. Add redaction for logged config
8. Add per-deployment config profiles
9. Unit tests for merge precedence
10. Document configuration reference

### Phase 003 — Observability baseline
**Branch:** `phase-003-observability`
**Goal:** Structured logging, metrics, tracing scaffolding.
1. Add structured logger with levels and fields
2. Add request/correlation ID propagation
3. Add metrics registry and counters
4. Add tracing spans abstraction
5. Add health and readiness endpoints
6. Add audit log channel (separate from app log)
7. Add log redaction for PII fields
8. Wire logger into config framework
9. Tests for correlation ID propagation
10. Document observability conventions

### Phase 004 — Persistence layer foundation
**Branch:** `phase-004-persistence`
**Goal:** Database abstraction, migrations, connection lifecycle.
1. Select and pin Postgres + pgvector and graph store drivers
2. Add connection pool and lifecycle management
3. Add migration runner
4. Author initial schema migration (tenants, deployments)
5. Add repository interface pattern
6. Add transaction helper utilities
7. Add health probe for DB connectivity
8. Add migration rollback support
9. Integration tests against ephemeral DB
10. Document data layer conventions

### Phase 005 — Tenancy & deployment model
**Branch:** `phase-005-tenancy`
**Goal:** Multi-tenant isolation keyed to a court/organisation deployment.
1. Define tenant and deployment entities
2. Add tenant context middleware
3. Add row-level isolation strategy
4. Add deployment provisioning record
5. Add tenant-scoped repository wrappers
6. Add tenant resolution from request
7. Enforce cross-tenant access denial
8. Seed default sandbox tenant
9. Tests for isolation guarantees
10. Document tenancy model

### Phase 006 — Identity & RBAC
**Branch:** `phase-006-identity-rbac`
**Goal:** Roles for judge, advocate, clerk, admin, auditor.
1. Define user and role entities
2. Add authentication interface (pluggable provider)
3. Add session/token issuance and validation
4. Define permission matrix per role
5. Add authorization middleware
6. Add role assignment workflow
7. Add auditor read-only role
8. Add account lifecycle (invite, disable)
9. Tests for permission enforcement
10. Document RBAC matrix

### Phase 007 — Jurisdiction registry
**Branch:** `phase-007-jurisdiction-registry`
**Goal:** Country + court-level catalogue with legal-family flag.
1. Define jurisdiction entity (country, court level, legal family)
2. Add legal-family enum (common law, civil law, mixed, Sharia-influenced)
3. Add jurisdiction seed data for initial countries
4. Add jurisdiction lookup service
5. Add procedural-rule reference per jurisdiction
6. Add language set per jurisdiction
7. Add jurisdiction validation rules
8. Add admin CRUD for jurisdictions
9. Tests for jurisdiction resolution
10. Document jurisdiction schema

### Phase 008 — Deploy-time setup wizard (backend)
**Branch:** `phase-008-setup-wizard-api`
**Goal:** First-run provisioning selecting country, court, language.
1. Define setup-state machine
2. Add step: select country/jurisdiction
3. Add step: select court level and place
4. Add step: select languages
5. Add step: provider configuration stub
6. Add setup completion and lock
7. Add idempotent re-run protection
8. Persist deployment profile
9. Tests for setup state transitions
10. Document setup API contract

### Phase 009 — API gateway & contract layer
**Branch:** `phase-009-api-gateway`
**Goal:** Versioned API surface and contract definitions.
1. Add API versioning scheme
2. Define OpenAPI/contract base document
3. Add request validation middleware
4. Add standard error envelope
5. Add rate limiting middleware
6. Add pagination conventions
7. Add CORS and security headers
8. Wire auth + tenancy into gateway
9. Contract tests
10. Document API conventions

### Phase 010 — Frontend shell & setup UI
**Branch:** `phase-010-frontend-shell`
**Goal:** App shell, auth flow, and setup wizard UI.
1. Scaffold frontend app and routing
2. Add design tokens and base layout
3. Add auth/login screens
4. Add setup wizard country step UI
5. Add court/place selection UI
6. Add language selection UI
7. Add provider config UI stub
8. Add setup completion screen
9. Component tests for wizard flow
10. Document frontend architecture

---

## Part 2 — Provider Abstraction & Model Agnosticism (Phases 11–18)

### Phase 011 — Provider abstraction interface
**Branch:** `phase-011-provider-interface`
**Goal:** Single interface all model calls flow through.
1. Define `LLMProvider` interface (chat, embed, stream)
2. Define request/response normalized types
3. Add capability descriptor per provider
4. Add token accounting hooks
5. Add timeout and cancellation support
6. Add error taxonomy and mapping
7. Add provider registry
8. Add no-op/mock provider for tests
9. Tests for interface conformance
10. Document provider contract

### Phase 012 — Provider router & policy
**Branch:** `phase-012-provider-router`
**Goal:** Route per task type, with fallback and per-deployment policy.
1. Define routing policy schema
2. Add task-type to provider mapping
3. Add fallback chain on failure
4. Add per-tenant provider override
5. Add cost/latency aware selection
6. Add circuit breaker per provider
7. Add routing telemetry
8. Add local/air-gapped-only mode flag
9. Tests for routing and fallback
10. Document routing policy

### Phase 013 — Cloud provider adapters
**Branch:** `phase-013-cloud-adapters`
**Goal:** Adapters for major hosted model APIs.
1. Add adapter scaffolding shared logic
2. Implement adapter A (chat + embed)
3. Implement adapter B (chat + embed)
4. Implement adapter C (chat + embed)
5. Normalize streaming responses
6. Map provider-specific errors
7. Add retry with backoff
8. Add per-adapter config validation
9. Adapter conformance tests
10. Document adapter setup

### Phase 014 — Local/self-hosted adapter
**Branch:** `phase-014-local-adapter`
**Goal:** OpenAI-compatible local runtime adapter for sovereign deploys.
1. Add local runtime adapter (OpenAI-compatible)
2. Add model discovery from local endpoint
3. Add embedding support
4. Add GGUF/served-model health check
5. Add offline-mode enforcement
6. Add resource-aware concurrency limit
7. Add quantization-aware config notes
8. Validate against air-gapped constraint
9. Conformance tests
10. Document local deployment

### Phase 015 — Embedding service abstraction
**Branch:** `phase-015-embedding-service`
**Goal:** Provider-agnostic embedding pipeline with caching.
1. Define embedding service interface
2. Add batching and chunk sizing
3. Add embedding cache keyed by content hash
4. Add dimension/metadata tracking
5. Add re-embedding on model change
6. Add embedding versioning
7. Add failure isolation
8. Add metrics for embedding throughput
9. Tests for cache hit/miss
10. Document embedding service

### Phase 016 — Prompt template registry
**Branch:** `phase-016-prompt-registry`
**Goal:** Versioned, jurisdiction-parameterized prompt templates.
1. Define template schema with variables
2. Add template versioning and pinning
3. Add jurisdiction/legal-family variants
4. Add language variants
5. Add template render engine
6. Add template validation
7. Add safe-variable injection (no prompt injection holes)
8. Add template test harness
9. Tests for render correctness
10. Document template authoring

### Phase 017 — Token accounting & budget
**Branch:** `phase-017-token-accounting`
**Goal:** Per-case, per-tenant usage tracking and limits.
1. Define usage record entity
2. Add per-request token capture
3. Aggregate usage per case and tenant
4. Add budget thresholds and alerts
5. Add usage export
6. Add hard-stop on budget breach (configurable)
7. Add usage dashboard API
8. Add reconciliation job
9. Tests for accounting accuracy
10. Document usage model

### Phase 018 — Model evaluation harness
**Branch:** `phase-018-model-eval-harness`
**Goal:** Compare models on legal-reasoning tasks before deployment.
1. Define eval task schema
2. Add golden-set storage
3. Add eval runner across providers
4. Add scoring rubrics (retrieval, reasoning, citation fidelity)
5. Add side-by-side comparison report
6. Add regression gate on model swap
7. Add deterministic seed handling
8. Add eval result persistence
9. Tests for eval runner
10. Document evaluation process

---

## Part 3 — Ingestion & Transcribe-and-Discard (Phases 19–30)

### Phase 019 — Upload intake service
**Branch:** `phase-019-upload-intake`
**Goal:** Accept files/audio/video without persisting binaries.
1. Define intake endpoint and streaming receiver
2. Add MIME and size validation
3. Add virus/malware scan hook
4. Add temporary buffer with strict TTL
5. Add provenance hash computation
6. Enforce no-binary-persistence policy
7. Add intake audit event
8. Add per-tenant intake quotas
9. Tests for discard guarantee
10. Document intake contract

### Phase 020 — Provenance & chain-of-custody
**Branch:** `phase-020-provenance`
**Goal:** Persist cryptographic proof of original even after discard.
1. Define provenance record entity
2. Capture hash, timestamp, uploader, case ref
3. Add signature over provenance record
4. Add immutable append-only store
5. Add custody event chain
6. Add verification endpoint
7. Add tamper-evidence check
8. Link provenance to extracted text
9. Tests for verification
10. Document custody model

### Phase 021 — Speech-to-text pipeline
**Branch:** `phase-021-stt-pipeline`
**Goal:** Transcribe audio/video to text, provider-agnostic.
1. Define STT provider interface
2. Add audio normalization and segmentation
3. Add adapter for STT provider(s)
4. Add language hint from jurisdiction
5. Add speaker diarization hooks
6. Add timestamped transcript output
7. Add confidence scores per segment
8. Discard source audio post-transcription
9. Tests for transcript shape
10. Document STT pipeline

### Phase 022 — OCR & document extraction
**Branch:** `phase-022-ocr-extraction`
**Goal:** Extract text from scanned docs and images.
1. Define OCR provider interface
2. Add image pre-processing (deskew, denoise)
3. Add OCR adapter
4. Add layout/region detection
5. Add table extraction
6. Add multi-language OCR support
7. Add confidence and source-span capture
8. Discard source images post-extraction
9. Tests for extraction accuracy
10. Document OCR pipeline

### Phase 023 — Multilingual normalization
**Branch:** `phase-023-multilingual-normalization`
**Goal:** Normalize Tamil, Urdu, Arabic, English transcripts.
1. Add Unicode normalization
2. Add script/language detection
3. Add transliteration where required
4. Add RTL handling for Arabic/Urdu
5. Add term normalization for legal vocabulary
6. Add optional translation pass with original retained
7. Add per-language tokenization rules
8. Add normalization audit trail
9. Tests across all four languages
10. Document normalization rules

### Phase 024 — Document segmentation
**Branch:** `phase-024-segmentation`
**Goal:** Split normalized text into logical units.
1. Define segment entity (paragraph, statement, exhibit)
2. Add sentence and clause splitting
3. Add heading/section detection
4. Add speaker-attributed segmentation for transcripts
5. Add exhibit and citation boundary detection
6. Preserve source-span offsets per segment
7. Add segment ordering and linkage
8. Add segment-level confidence
9. Tests for segmentation fidelity
10. Document segmentation model

### Phase 025 — PII detection & handling
**Branch:** `phase-025-pii-handling`
**Goal:** Detect and govern sensitive personal data in text.
1. Add NER-based PII detector
2. Classify PII categories
3. Add configurable redaction/pseudonymization
4. Preserve mapping under access control
5. Add jurisdiction-specific PII rules
6. Add PII audit logging
7. Add reversible vs irreversible modes
8. Enforce PII policy at storage boundary
9. Tests for detection recall
10. Document PII governance

### Phase 026 — Evidence classification
**Branch:** `phase-026-evidence-classification`
**Goal:** Tag segments as testimony, exhibit, statute citation, argument.
1. Define evidence-type taxonomy
2. Add classifier over segments
3. Add witness-statement detection
4. Add documentary-evidence detection
5. Add statutory-citation detection
6. Add party attribution (first/second party)
7. Add confidence and override support
8. Persist classifications
9. Tests for classification
10. Document evidence taxonomy

### Phase 027 — Case category engine
**Branch:** `phase-027-case-category`
**Goal:** Categorize case as civil, criminal, domestic violence, consumer, etc.
1. Define category taxonomy per jurisdiction
2. Add category suggestion from content
3. Add manual override by user
4. Add sub-category support
5. Map category to procedural rules
6. Map category to applicable statute partitions
7. Add category change audit
8. Validate category against jurisdiction
9. Tests for categorization
10. Document category model

### Phase 028 — Party & timeline modeling
**Branch:** `phase-028-party-timeline`
**Goal:** Model the two parties and the chronological case timeline.
1. Define party entity (first/second, roles, counsel)
2. Add party-fact attribution
3. Add event extraction with dates
4. Add timeline assembly and ordering
5. Add conflict detection across statements
6. Add party-claim linkage
7. Add relationship modeling between parties
8. Persist party and timeline graph
9. Tests for timeline assembly
10. Document party/timeline model

### Phase 029 — Ingestion orchestration
**Branch:** `phase-029-ingestion-orchestration`
**Goal:** Coordinate intake → STT/OCR → normalize → segment → classify.
1. Define ingestion workflow/state machine
2. Add async job queue
3. Add step retries and idempotency
4. Add partial-failure recovery
5. Add progress reporting
6. Add discard verification at each stage
7. Add per-case ingestion status API
8. Add dead-letter handling
9. Tests for full pipeline
10. Document ingestion workflow

### Phase 030 — Ingestion UI
**Branch:** `phase-030-ingestion-ui`
**Goal:** User-facing case creation and upload experience.
1. Add case creation form (category, parties)
2. Add multi-file/audio upload UI
3. Add live transcription/extraction status
4. Add discard confirmation messaging
5. Add extracted-text review panel
6. Add classification correction UI
7. Add party/timeline editor
8. Add validation and error states
9. Component tests
10. Document ingestion UX

---

## Part 4 — IRAC Reasoning Tree & Knowledge Layer (Phases 31–48)

### Phase 031 — IRAC schema definition
**Branch:** `phase-031-irac-schema`
**Goal:** Formal schema for Issue, Rule, Application, Conclusion tree.
1. Define node types (Issue, Rule, Fact, Application, Conclusion)
2. Define edge types and constraints
3. Add source-span linkage on every node
4. Add confidence and provenance on nodes
5. Add jurisdiction tagging on rule nodes
6. Add versioning for tree revisions
7. Add validation rules for tree integrity
8. Add serialization format
9. Tests for schema validation
10. Document IRAC schema

### Phase 032 — Graph store integration
**Branch:** `phase-032-graph-store`
**Goal:** Persist reasoning trees in a graph database.
1. Select and configure graph store
2. Add node/edge persistence layer
3. Add tenant-scoped graph isolation
4. Add graph migration tooling
5. Add indexing for traversal performance
6. Add transactional graph writes
7. Add graph backup/restore
8. Add graph health checks
9. Integration tests for graph ops
10. Document graph layer

### Phase 033 — Issue extraction
**Branch:** `phase-033-issue-extraction`
**Goal:** Derive legal issues from case facts and claims.
1. Add issue-identification pass over segments
2. Map claims to candidate issues
3. Add issue deduplication and merging
4. Add sub-issue decomposition
5. Link issues to parties and facts
6. Add confidence scoring
7. Add human review/override hooks
8. Persist issue nodes
9. Tests for issue extraction
10. Document issue extraction

### Phase 034 — Fact node construction
**Branch:** `phase-034-fact-nodes`
**Goal:** Build fact nodes with evidence backing.
1. Convert classified segments into fact nodes
2. Attach evidence references (testimony, exhibit)
3. Add party attribution to facts
4. Add disputed/undisputed flagging
5. Add temporal anchoring
6. Add corroboration linkage
7. Add reliability scoring
8. Persist fact nodes and edges
9. Tests for fact construction
10. Document fact model

### Phase 035 — Statute & rule ingestion
**Branch:** `phase-035-statute-ingestion`
**Goal:** Load jurisdiction statutes as structured rule nodes.
1. Define statute corpus loader
2. Parse statute hierarchy (act, section, clause)
3. Build rule nodes with citations
4. Tag rules by category and jurisdiction
5. Add effective-date and amendment tracking
6. Add cross-reference resolution
7. Embed rule text for retrieval
8. Persist rule corpus per jurisdiction
9. Tests for statute parsing
10. Document statute model

### Phase 036 — Precedent corpus ingestion
**Branch:** `phase-036-precedent-ingestion`
**Goal:** Load prior cases/precedents for common-law jurisdictions.
1. Define precedent loader and schema
2. Extract holding and ratio decidendi
3. Build precedent nodes with citations
4. Tag by issue and category
5. Add court-hierarchy weighting
6. Embed precedent text
7. Add precedent recency/authority metadata
8. Persist precedent corpus
9. Tests for precedent ingestion
10. Document precedent model

### Phase 037 — Rule linkage & application nodes
**Branch:** `phase-037-rule-application`
**Goal:** Connect issues to governing rules and applications.
1. Match issues to candidate rules
2. Build application nodes (rule applied to facts)
3. Add precedent-to-issue linkage
4. Add distinguishing-facts capture
5. Add multi-hop rule chains
6. Add weighting by legal family (statute vs precedent)
7. Add confidence on each linkage
8. Persist application subgraph
9. Tests for linkage correctness
10. Document application model

### Phase 038 — Legal ontology layer
**Branch:** `phase-038-legal-ontology`
**Goal:** Classification ontology over legal concepts.
1. Define ontology schema (concept, relation)
2. Seed core legal concepts per category
3. Add jurisdiction-specific ontology overlays
4. Link rules and issues to ontology concepts
5. Add concept synonym/alias handling
6. Add multilingual concept labels
7. Add ontology versioning
8. Persist ontology layer
9. Tests for ontology linkage
10. Document ontology

### Phase 039 — Tree assembly orchestrator
**Branch:** `phase-039-tree-assembly`
**Goal:** Assemble full IRAC tree from extracted components.
1. Define assembly workflow
2. Compose issues → rules → facts → applications → conclusions
3. Add integrity validation on assembled tree
4. Add gap detection (unsupported conclusions)
5. Add tree revision/versioning on update
6. Add incremental re-assembly on new evidence
7. Add assembly telemetry
8. Persist assembled tree
9. Tests for assembly
10. Document assembly process

### Phase 040 — Tree integrity & validation
**Branch:** `phase-040-tree-validation`
**Goal:** Guarantee every conclusion traces to facts and rules.
1. Add rule: conclusion must link to ≥1 fact and ≥1 rule
2. Add orphan-node detection
3. Add cycle detection
4. Add unsupported-claim flagging
5. Add confidence-propagation checks
6. Add jurisdiction-consistency checks
7. Add validation report generation
8. Block downstream use on critical failures
9. Tests for validation rules
10. Document integrity guarantees

### Phase 041 — Vector index over leaf nodes
**Branch:** `phase-041-vector-index`
**Goal:** Embed and index leaf nodes for semantic recall.
1. Define indexable leaf-node projection
2. Generate embeddings per leaf
3. Store vectors with node references
4. Add metadata filters (category, jurisdiction, party)
5. Add ANN index configuration
6. Add re-index on tree revision
7. Add hybrid score fields
8. Add index health checks
9. Tests for retrieval recall
10. Document vector index

### Phase 042 — Tree-structured indexing
**Branch:** `phase-042-tree-indexing`
**Goal:** Index the hierarchy itself for structured retrieval.
1. Define hierarchical index schema
2. Index issue → sub-issue paths
3. Index rule → application → conclusion paths
4. Add path-based lookup API
5. Add depth-bounded traversal queries
6. Add index maintenance on writes
7. Add caching for hot paths
8. Add index statistics
9. Tests for path queries
10. Document tree indexing

### Phase 043 — Graph traversal queries
**Branch:** `phase-043-graph-traversal`
**Goal:** Multi-hop legal-logic traversal (issue→statute→precedent→facts).
1. Define traversal query DSL/builder
2. Add issue-to-governing-rule traversal
3. Add rule-to-controlling-precedent traversal
4. Add precedent-to-distinguishing-facts traversal
5. Add bounded-depth and pruning
6. Add weighted-path ranking
7. Add traversal result shaping
8. Add traversal caching
9. Tests for multi-hop queries
10. Document traversal API

### Phase 044 — Hybrid retrieval engine
**Branch:** `phase-044-hybrid-retrieval`
**Goal:** Fuse vector recall with graph traversal.
1. Define hybrid query interface
2. Run vector recall over leaf nodes
3. Expand results via graph traversal
4. Add reciprocal-rank fusion / re-ranking
5. Add jurisdiction and category filters
6. Add diversity and dedup
7. Add explanation of retrieval path
8. Add latency budget controls
9. Tests for hybrid relevance
10. Document hybrid retrieval

### Phase 045 — Adaptive retrieval structures
**Branch:** `phase-045-adaptive-retrieval`
**Goal:** Avoid costly full-graph pre-builds; build reasoning structure on demand.
1. Add query-driven subgraph construction
2. Add incremental structure caching
3. Add cost-aware build limits
4. Add staleness detection and refresh
5. Add fallback to pre-built paths
6. Add adaptive depth based on query
7. Add telemetry on build cost
8. Add update-latency safeguards
9. Tests for adaptive build
10. Document adaptive retrieval

### Phase 046 — Citation fidelity layer
**Branch:** `phase-046-citation-fidelity`
**Goal:** Guarantee every retrieved claim carries a verifiable source.
1. Attach source-span to every retrieved unit
2. Add citation resolver to statute/precedent
3. Add citation verification check
4. Flag unsupported or hallucinated citations
5. Add citation formatting per jurisdiction
6. Add broken-citation detection
7. Add citation confidence scoring
8. Persist citation links
9. Tests for citation integrity
10. Document citation standards

### Phase 047 — Cross-case knowledge isolation
**Branch:** `phase-047-knowledge-isolation`
**Goal:** Prevent leakage of one case's facts into another's reasoning.
1. Enforce case-scoped fact retrieval
2. Separate shared-law corpus from case facts
3. Add retrieval-scope guards
4. Add audit for cross-case access attempts
5. Add tenant + case compound scoping
6. Add explicit opt-in for cross-case analysis
7. Add leakage tests
8. Add scope violation alerts
9. Tests for isolation
10. Document isolation guarantees

### Phase 048 — Knowledge layer API
**Branch:** `phase-048-knowledge-api`
**Goal:** Stable internal API for tree, retrieval, and citations.
1. Define knowledge-layer service interface
2. Expose tree read/query endpoints
3. Expose hybrid retrieval endpoint
4. Expose citation resolution endpoint
5. Add pagination and filtering
6. Add access control on knowledge API
7. Add response schemas and contracts
8. Add API versioning
9. Contract tests
10. Document knowledge API

---

## Part 5 — Reasoning & Adversarial Synthesis (Phases 49–62)

### Phase 049 — Reasoning agent framework
**Branch:** `phase-049-agent-framework`
**Goal:** Base framework for tool-using reasoning agents.
1. Define agent interface and lifecycle
2. Add tool-calling abstraction over knowledge API
3. Add agent memory/scratchpad scoped to case
4. Add step budget and termination
5. Add agent telemetry and tracing
6. Add deterministic-seed support
7. Add provider routing per agent
8. Add error and timeout handling
9. Tests for agent loop
10. Document agent framework

### Phase 050 — Issue-analysis agent
**Branch:** `phase-050-issue-agent`
**Goal:** Agent that frames the legal issues for adjudication.
1. Define issue-agent prompt templates
2. Pull issues from tree
3. Rank issues by materiality
4. Identify governing legal questions
5. Surface ambiguities and gaps
6. Output structured issue list
7. Add jurisdiction-aware framing
8. Add confidence annotations
9. Tests for issue framing
10. Document issue agent

### Phase 051 — First-party argument agent
**Branch:** `phase-051-first-party-agent`
**Goal:** Build strongest good-faith case for first party.
1. Define argument-agent template
2. Retrieve favorable facts and rules
3. Construct argument chains per issue
4. Cite supporting precedent/statute
5. Anticipate counterarguments
6. Score argument strength
7. Constrain to evidence in tree (no fabrication)
8. Output structured argument set
9. Tests for argument grounding
10. Document first-party agent

### Phase 052 — Second-party argument agent
**Branch:** `phase-052-second-party-agent`
**Goal:** Build strongest good-faith case for second party.
1. Reuse argument-agent template for second party
2. Retrieve favorable facts and rules
3. Construct counter-argument chains
4. Cite supporting authority
5. Rebut first-party arguments
6. Score argument strength
7. Constrain to evidence in tree
8. Output structured argument set
9. Tests for argument grounding
10. Document second-party agent

### Phase 053 — Evidence-weighing module
**Branch:** `phase-053-evidence-weighing`
**Goal:** Assess reliability and weight of competing evidence.
1. Define evidence-weight rubric
2. Score testimony reliability (corroboration, consistency)
3. Score documentary evidence
4. Flag contradictions across statements
5. Weight by evidentiary standards per jurisdiction
6. Surface gaps in the evidentiary record
7. Output per-fact weight
8. Persist weighting rationale
9. Tests for weighing logic
10. Document evidence weighting

### Phase 054 — Law-application module
**Branch:** `phase-054-law-application`
**Goal:** Apply governing rules to weighed facts per issue.
1. Map each issue to controlling rules
2. Apply legal tests/elements to facts
3. Weight statute vs precedent by legal family
4. Handle conflicting authority
5. Produce per-issue legal analysis
6. Capture reasoning steps explicitly
7. Cite every applied authority
8. Output structured application records
9. Tests for application logic
10. Document law application

### Phase 055 — Synthesis & reasoned-opinion agent
**Branch:** `phase-055-synthesis-agent`
**Goal:** Weigh arguments × evidence × law into a draft analysis.
1. Define synthesis template
2. Ingest both parties' arguments
3. Ingest evidence weights and law application
4. Resolve each issue with reasoning
5. Produce per-issue tentative conclusion
6. Surface weakest-link reasoning
7. Trace every conclusion to fact + rule nodes
8. Output structured draft opinion
9. Tests for synthesis traceability
10. Document synthesis agent

### Phase 056 — Weakest-link & uncertainty surfacing
**Branch:** `phase-056-uncertainty-surfacing`
**Goal:** Make the analysis honest about where it is weak.
1. Identify low-confidence reasoning steps
2. Identify thin or disputed evidence
3. Identify unsettled or conflicting law
4. Rank uncertainties by impact on outcome
5. Generate explicit caveats
6. Attach uncertainty to relevant conclusions
7. Block over-confident phrasing
8. Output uncertainty report
9. Tests for uncertainty detection
10. Document uncertainty surfacing

### Phase 057 — Non-binding guardrail enforcement
**Branch:** `phase-057-nonbinding-guardrail`
**Goal:** Enforce "draft analysis, never verdict" in code.
1. Add output labeling schema (draft, non-binding)
2. Enforce label on all reasoning outputs
3. Block verdict/directive phrasing
4. Add mandatory disclaimer injection
5. Require human-signoff state before finalization
6. Add policy tests that fail on verdict language
7. Add audit of guardrail enforcement
8. Add override-prevention controls
9. Tests for guardrail
10. Document guardrail policy

### Phase 058 — Jurisdiction-parameterized reasoning
**Branch:** `phase-058-jurisdiction-reasoning`
**Goal:** Same engine, reweighted by legal family per deployment.
1. Define reasoning-weight profile per legal family
2. Common-law profile (precedent-heavy)
3. Civil-law profile (statute-heavy)
4. Mixed/Sharia-influenced profiles
5. Wire profile selection from jurisdiction registry
6. Apply profile to synthesis and application
7. Add profile override per case (with audit)
8. Add profile validation
9. Tests across legal families
10. Document parameterization

### Phase 059 — Reasoning orchestration pipeline
**Branch:** `phase-059-reasoning-orchestration`
**Goal:** Coordinate the full agent sequence end to end.
1. Define reasoning workflow/state machine
2. Sequence: issues → arguments → weighing → application → synthesis
3. Add parallelism where safe
4. Add checkpointing and resume
5. Add per-stage telemetry
6. Add cost and time budgets
7. Add failure isolation per stage
8. Persist intermediate artifacts
9. Tests for full pipeline
10. Document orchestration

### Phase 060 — Reasoning explanation & trace
**Branch:** `phase-060-reasoning-trace`
**Goal:** Produce a fully auditable reasoning trace.
1. Capture every agent step and tool call
2. Record retrieved nodes and citations
3. Build human-readable reasoning narrative
4. Link narrative back to tree nodes
5. Add expandable evidence/authority trail
6. Add trace export
7. Add trace access control
8. Add trace integrity hash
9. Tests for trace completeness
10. Document reasoning trace

### Phase 061 — Hallucination & grounding checks
**Branch:** `phase-061-grounding-checks`
**Goal:** Verify every assertion is grounded in case or law.
1. Add claim-extraction from outputs
2. Verify each claim against tree/corpus
3. Flag ungrounded assertions
4. Block finalization on critical ungrounded claims
5. Add citation-existence verification
6. Add numeric/date consistency checks
7. Add grounding confidence score
8. Add grounding report
9. Tests for grounding detection
10. Document grounding checks

### Phase 062 — Reasoning quality evaluation
**Branch:** `phase-062-reasoning-eval`
**Goal:** Continuously evaluate reasoning output quality.
1. Define quality rubric (grounding, citation, coherence)
2. Add automated scoring over sample outputs
3. Add expert-review capture workflow
4. Add regression detection on model/template change
5. Add per-jurisdiction quality tracking
6. Add quality dashboard API
7. Add alerting on quality drop
8. Persist evaluation history
9. Tests for eval pipeline
10. Document quality evaluation

---

## Part 6 — Case Management & Workflow (Phases 63–74)

### Phase 063 — Case lifecycle management
**Branch:** `phase-063-case-lifecycle`
**Goal:** Full case state machine from intake to closure.
1. Define case states (draft, active, under-review, closed)
2. Add state-transition rules and guards
3. Add per-state permitted actions
4. Add reopen and archive flows
5. Add case metadata management
6. Add transition audit log
7. Add bulk operations
8. Persist case lifecycle
9. Tests for transitions
10. Document case lifecycle

### Phase 064 — Case workspace UI
**Branch:** `phase-064-case-workspace`
**Goal:** Unified workspace for a case.
1. Build case overview layout
2. Add parties and category panel
3. Add evidence and timeline view
4. Add tree visualization entry point
5. Add reasoning/opinion panel
6. Add status and actions bar
7. Add quick navigation
8. Add responsive layout
9. Component tests
10. Document workspace UX

### Phase 065 — IRAC tree visualization
**Branch:** `phase-065-tree-visualization`
**Goal:** Interactive visual of the reasoning tree.
1. Render tree/graph layout
2. Color nodes by type (issue, rule, fact, conclusion)
3. Add node detail on selection
4. Show source-span and citation per node
5. Add collapse/expand and depth control
6. Highlight conclusion-to-evidence paths
7. Add confidence indicators
8. Add export of tree view
9. Component tests
10. Document tree visualization

### Phase 066 — Evidence review UI
**Branch:** `phase-066-evidence-ui`
**Goal:** Review and correct extracted evidence and classifications.
1. List evidence segments with type
2. Add inline correction of classification
3. Add party reassignment
4. Add disputed/undisputed toggles
5. Show provenance and confidence
6. Add bulk tagging
7. Add change audit
8. Add search/filter
9. Component tests
10. Document evidence review

### Phase 067 — Reasoned-opinion review UI
**Branch:** `phase-067-opinion-ui`
**Goal:** Present draft analysis with full traceability.
1. Render per-issue analysis sections
2. Show both parties' arguments side by side
3. Surface evidence weights inline
4. Show weakest-link/uncertainty callouts
5. Link every conclusion to its trace
6. Display non-binding disclaimer prominently
7. Add comment/annotation by judge
8. Add export controls
9. Component tests
10. Document opinion review

### Phase 068 — Human sign-off workflow
**Branch:** `phase-068-signoff-workflow`
**Goal:** Mandatory judge review and acknowledgement.
1. Define sign-off state and gates
2. Require explicit human acknowledgement
3. Capture reviewer identity and timestamp
4. Add reviewer notes and decisions
5. Prevent finalization without sign-off
6. Add sign-off audit record
7. Add re-review on case update
8. Add notification on pending sign-off
9. Tests for sign-off enforcement
10. Document sign-off workflow

### Phase 069 — Search & case discovery
**Branch:** `phase-069-search-discovery`
**Goal:** Search across cases using the tree-structured index.
1. Add case search API over hybrid index
2. Add filters (category, jurisdiction, party, date)
3. Add issue/rule-based search
4. Add semantic and keyword modes
5. Add result ranking and snippets
6. Add saved searches
7. Add access-scoped results
8. Add search UI
9. Tests for search relevance
10. Document search

### Phase 070 — Annotations & collaboration
**Branch:** `phase-070-annotations`
**Goal:** Multi-user notes, highlights, and discussion on a case.
1. Define annotation entity
2. Add node/segment-anchored annotations
3. Add threaded comments
4. Add mentions and notifications
5. Add resolve/close states
6. Add access control on annotations
7. Add annotation audit
8. Add annotation UI
9. Tests for collaboration
10. Document annotations

### Phase 071 — Versioning & case history
**Branch:** `phase-071-case-versioning`
**Goal:** Track every revision of tree, evidence, and opinion.
1. Add version snapshots per case artifact
2. Add diff between versions
3. Add restore to prior version
4. Add change-attribution per version
5. Add tree-revision linkage
6. Add opinion-revision history
7. Add history UI
8. Persist version store
9. Tests for versioning
10. Document version history

### Phase 072 — Notifications & tasks
**Branch:** `phase-072-notifications`
**Goal:** Surface pending actions to users.
1. Define notification entity and channels
2. Add ingestion-complete notifications
3. Add pending-sign-off notifications
4. Add quality-alert notifications
5. Add task assignment
6. Add notification preferences
7. Add read/unread state
8. Add notification UI
9. Tests for delivery
10. Document notifications

### Phase 073 — Reporting & export
**Branch:** `phase-073-reporting-export`
**Goal:** Export case analysis as a structured report.
1. Define report template (facts, issues, analysis, citations)
2. Add export to PDF and DOCX
3. Embed non-binding disclaimer
4. Include reasoning trace appendix
5. Add jurisdiction-correct citation formatting
6. Add redaction options for export
7. Add export audit
8. Add export UI
9. Tests for export integrity
10. Document reporting

### Phase 074 — Dashboards & analytics
**Branch:** `phase-074-dashboards`
**Goal:** Operational and case-portfolio analytics.
1. Define analytics metrics (caseload, categories, status)
2. Add aggregation pipeline
3. Add per-jurisdiction breakdowns
4. Add reasoning-quality trends
5. Add usage and cost views
6. Add export of analytics
7. Add access-scoped dashboards
8. Add dashboard UI
9. Tests for aggregation
10. Document analytics

---

## Part 7 — Security, Compliance & Sovereignty (Phases 75–86)

### Phase 075 — Encryption at rest & in transit
**Branch:** `phase-075-encryption`
**Goal:** Encrypt all stored and transmitted data.
1. Enforce TLS for all service traffic
2. Add DB-level encryption at rest
3. Add field-level encryption for sensitive data
4. Add key management integration
5. Add key rotation support
6. Add encrypted backups
7. Add cipher policy config
8. Verify no plaintext sensitive storage
9. Tests for encryption paths
10. Document encryption

### Phase 076 — Key management & secrets
**Branch:** `phase-076-key-management`
**Goal:** Centralized, rotatable secret and key handling.
1. Integrate secrets/KMS provider abstraction
2. Add key lifecycle management
3. Add secret rotation workflow
4. Add per-tenant key isolation
5. Add access policies on keys
6. Add break-glass procedure
7. Add audit of key access
8. Support offline key store for air-gap
9. Tests for key ops
10. Document key management

### Phase 077 — Comprehensive audit trail
**Branch:** `phase-077-audit-trail`
**Goal:** Immutable, queryable audit of all sensitive actions.
1. Define audit event schema
2. Capture data access events
3. Capture reasoning and sign-off events
4. Add tamper-evident append-only store
5. Add audit query API
6. Add retention policy
7. Add export for regulators
8. Add access control on audit
9. Tests for audit completeness
10. Document audit trail

### Phase 078 — Data residency & sovereignty
**Branch:** `phase-078-data-residency`
**Goal:** Guarantee data stays within jurisdiction boundaries.
1. Add residency policy per deployment
2. Pin storage region/locality
3. Block cross-border data movement
4. Add provider-locality enforcement
5. Add residency verification checks
6. Add air-gapped deployment mode
7. Add residency audit
8. Add violation alerting
9. Tests for residency enforcement
10. Document sovereignty model

### Phase 079 — Air-gapped deployment tier
**Branch:** `phase-079-airgapped-tier`
**Goal:** Fully offline deployment for sensitive courts.
1. Define air-gapped deployment profile
2. Bundle local model runtime
3. Remove all external network dependencies
4. Add offline corpus provisioning
5. Add offline update mechanism
6. Add offline license/activation
7. Verify zero egress
8. Add air-gap conformance checks
9. Tests for offline operation
10. Document air-gapped tier

### Phase 080 — Access governance & least privilege
**Branch:** `phase-080-access-governance`
**Goal:** Fine-grained, auditable access control.
1. Add attribute-based access policies
2. Add per-case access grants
3. Add time-bound and just-in-time access
4. Add access review workflow
5. Add segregation of duties
6. Add privileged-access monitoring
7. Add access certification reports
8. Add policy testing harness
9. Tests for access governance
10. Document access governance

### Phase 081 — Privacy & data subject controls
**Branch:** `phase-081-privacy-controls`
**Goal:** Honor data-subject rights and minimization.
1. Add data inventory and mapping
2. Add data-minimization enforcement
3. Add retention and deletion policies
4. Add subject-access request handling
5. Add right-to-erasure with provenance preservation
6. Add consent/legal-basis tracking
7. Add privacy audit
8. Add anonymization for analytics
9. Tests for privacy controls
10. Document privacy controls

### Phase 082 — Compliance mapping
**Branch:** `phase-082-compliance-mapping`
**Goal:** Map controls to relevant legal/regulatory frameworks.
1. Build control catalogue
2. Map UAE data-protection requirements
3. Map international frameworks as applicable
4. Map judicial-records handling rules
5. Add control-evidence collection
6. Add gap analysis reporting
7. Add per-deployment compliance profile
8. Add compliance dashboard
9. Tests for control coverage
10. Document compliance mapping

### Phase 083 — Threat modeling & hardening
**Branch:** `phase-083-threat-hardening`
**Goal:** Systematically reduce attack surface.
1. Produce threat model per service
2. Harden API input validation
3. Add prompt-injection defenses on ingestion
4. Add output-handling safeguards
5. Add dependency pinning and SBOM
6. Add secrets-scanning in CI
7. Add container hardening
8. Add network segmentation policy
9. Tests for hardening controls
10. Document threat model

### Phase 084 — Vulnerability & dependency management
**Branch:** `phase-084-vuln-management`
**Goal:** Continuous detection of vulnerable components.
1. Add SCA scanning in CI
2. Add SAST scanning
3. Add container image scanning
4. Add automated dependency updates
5. Add vulnerability triage workflow
6. Add SLA tracking for fixes
7. Add license compliance checks
8. Add reporting
9. Tests for scan integration
10. Document vuln management

### Phase 085 — Backup, restore & DR
**Branch:** `phase-085-backup-dr`
**Goal:** Resilient backup and disaster recovery.
1. Define backup policy per data class
2. Add encrypted automated backups
3. Add point-in-time recovery
4. Add cross-region/offline backup options
5. Add restore drills
6. Define RPO/RTO targets
7. Add DR runbook
8. Add backup integrity verification
9. Tests for restore
10. Document backup & DR

### Phase 086 — Security testing & red team
**Branch:** `phase-086-security-testing`
**Goal:** Validate defenses adversarially.
1. Add automated security regression tests
2. Add penetration-test scope and harness
3. Add prompt-injection adversarial suite
4. Add data-leakage tests
5. Add authz bypass tests
6. Add abuse-case tests
7. Add findings tracking
8. Add remediation verification
9. Tests for security suite
10. Document security testing

---

## Part 8 — Integration, Operations & Hardening (Phases 87–100)

### Phase 087 — External system integration framework
**Branch:** `phase-087-integration-framework`
**Goal:** Connect to court case-management systems where permitted.
1. Define integration adapter interface
2. Add inbound case import
3. Add outbound report delivery
4. Add field mapping configuration
5. Add integration auth and security
6. Add retry and reconciliation
7. Add integration audit
8. Add sandbox/test connectors
9. Tests for integration
10. Document integration framework

### Phase 088 — Bulk import & migration tools
**Branch:** `phase-088-bulk-import`
**Goal:** Onboard large historical case corpora.
1. Define bulk import schema
2. Add batched ingestion pipeline
3. Add validation and error reporting
4. Add resumable imports
5. Add deduplication
6. Add progress tracking
7. Add rollback on failure
8. Add import audit
9. Tests for bulk import
10. Document migration tools

### Phase 089 — Statute/precedent corpus updater
**Branch:** `phase-089-corpus-updater`
**Goal:** Keep legal corpora current per jurisdiction.
1. Define corpus update workflow
2. Add amendment ingestion
3. Add effective-date handling
4. Add re-embedding on updates
5. Add change notification to affected cases
6. Add update validation
7. Add rollback support
8. Add update audit
9. Tests for corpus updates
10. Document corpus updating

### Phase 090 — Localization & i18n
**Branch:** `phase-090-localization`
**Goal:** Full UI and output localization across languages.
1. Externalize UI strings
2. Add Arabic, Urdu, Tamil, English locales
3. Add RTL layout support
4. Localize dates, numbers, citations
5. Localize generated reports
6. Add locale switching
7. Add translation management
8. Add fallback handling
9. Tests for localization
10. Document i18n

### Phase 091 — Performance optimization
**Branch:** `phase-091-performance`
**Goal:** Meet latency and throughput targets.
1. Add performance benchmarks
2. Profile ingestion pipeline
3. Profile retrieval and traversal
4. Add caching where safe
5. Optimize graph queries and indexes
6. Add async and batching improvements
7. Add load testing
8. Tune resource limits
9. Tests for performance regressions
10. Document performance work

### Phase 092 — Scalability & horizontal scaling
**Branch:** `phase-092-scalability`
**Goal:** Scale services and stores under load.
1. Make services horizontally scalable
2. Add queue-based workload distribution
3. Add stateless service guarantees
4. Add store partitioning/sharding strategy
5. Add autoscaling policies
6. Add backpressure handling
7. Add capacity planning model
8. Add scale testing
9. Tests for scaling behavior
10. Document scalability

### Phase 093 — Reliability & resilience
**Branch:** `phase-093-reliability`
**Goal:** Graceful degradation and fault tolerance.
1. Add timeouts and retries everywhere
2. Add circuit breakers on dependencies
3. Add graceful degradation modes
4. Add idempotency across pipelines
5. Add chaos/failure injection tests
6. Add health-based traffic shifting
7. Add SLO definitions and tracking
8. Add error-budget policy
9. Tests for resilience
10. Document reliability

### Phase 094 — Deployment & infrastructure as code
**Branch:** `phase-094-iac`
**Goal:** Reproducible deployments across tiers.
1. Add IaC for cloud tier
2. Add IaC for on-prem tier
3. Add IaC for air-gapped tier
4. Add containerization and orchestration manifests
5. Add environment promotion pipeline
6. Add secret injection at deploy
7. Add blue-green/canary support
8. Add deployment verification
9. Tests for IaC
10. Document deployment

### Phase 095 — CI/CD pipeline hardening
**Branch:** `phase-095-cicd-hardening`
**Goal:** Secure, gated delivery pipeline.
1. Enforce full test suite gate
2. Enforce security scan gates
3. Enforce no-squash and branch policy in CI
4. Add signed builds and artifacts
5. Add provenance/attestation for releases
6. Add staged rollout automation
7. Add automated rollback triggers
8. Add release notes automation
9. Tests for pipeline gates
10. Document CI/CD

### Phase 096 — Monitoring & alerting
**Branch:** `phase-096-monitoring-alerting`
**Goal:** Production visibility and on-call alerting.
1. Add service and business metrics
2. Add dashboards for key flows
3. Add SLO-based alerts
4. Add reasoning-quality alerts
5. Add cost and usage alerts
6. Add on-call routing
7. Add alert runbooks
8. Add synthetic monitoring
9. Tests for alert rules
10. Document monitoring

### Phase 097 — End-to-end testing suite
**Branch:** `phase-097-e2e-testing`
**Goal:** Full-journey automated tests across jurisdictions.
1. Define E2E scenarios per case category
2. Add setup-to-opinion journey tests
3. Add multi-jurisdiction variants
4. Add multilingual ingestion tests
5. Add discard-guarantee verification
6. Add sign-off enforcement tests
7. Add data-isolation tests
8. Add CI integration for E2E
9. Add flaky-test controls
10. Document E2E suite

### Phase 098 — Documentation & runbooks
**Branch:** `phase-098-documentation`
**Goal:** Complete operator, admin, and developer docs.
1. Write architecture overview
2. Write deployment guides per tier
3. Write admin and setup guide
4. Write user guide for judges/advocates
5. Write operations runbooks
6. Write incident-response runbook
7. Write API reference
8. Write security and compliance docs
9. Review and link docs
10. Publish documentation site

### Phase 099 — Pilot deployment & feedback loop
**Branch:** `phase-099-pilot`
**Goal:** Controlled pilot with real users and structured feedback.
1. Provision pilot deployment
2. Onboard pilot jurisdiction and corpus
3. Run supervised pilot cases
4. Collect structured expert feedback
5. Measure reasoning quality and trust
6. Triage and prioritize fixes
7. Apply high-priority refinements
8. Validate non-binding workflow in practice
9. Capture pilot report
10. Document pilot findings

### Phase 100 — General availability hardening & release
**Branch:** `phase-100-ga-release`
**Goal:** Final hardening and GA cut.
1. Resolve all critical and high findings
2. Final security and compliance review
3. Final performance and scale validation
4. Freeze and tag release candidate
5. Run full regression and E2E
6. Verify all guardrails and audits
7. Prepare release artifacts and attestation
8. Cut GA release tag
9. Post-release verification
10. Publish release notes and changelog

---

## Cross-Cutting Reminders

- Every reasoning output remains a non-binding draft analysis; the final verdict is always a human judge's.
- The provider abstraction (Phases 11–18) is the only path to any model; no phase bypasses it.
- The transcribe-and-discard guarantee (Phases 19–30) is verified in tests at every stage that touches a binary.
- Jurisdiction parameterization (Phase 58) is the core differentiator: one engine, reweighted by legal family per deployment.
- Maintain the git discipline on every phase: one branch, ≥10 atomic commits, PR, no squash, no direct push to `main`, identity `YASSERRMD`.
