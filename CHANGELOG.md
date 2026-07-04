# Changelog

All notable changes to Verdex are documented here at a high level. For
a full, commit-by-commit history, see `git log`; for automatically
generated, per-release commit groupings, see
`packages/garelease.BuildReleaseNotes` (composes with
`packages/cicdgate.GenerateReleaseNotes`).

## [1.0.0] - 2026-07-04 -- General availability (100-phase build)

Tagged as `v1.0.0` once `packages/garelease`'s release-readiness gate
and this changelog entry were both in place -- the deliberate last
step of the 100-phase build.

This release is the culmination of a 100-phase build, delivered across
eight parts:

### 1. Foundation & Provisioning

Repository bootstrap, the configuration framework, observability
(structured logging, metrics, health endpoints), a Postgres-backed
persistence layer with migration tooling, and multi-tenancy with
Row-Level-Security enforcement. Includes the first-run deployment
wizard and jurisdiction loading that every later phase builds on.

### 2. Provider Abstraction

The `LLMProvider` interface every phase routes inference calls through
-- no phase hardcodes a specific model vendor. Includes request
routing, prompt templates (with the mandatory non-binding-disclaimer
injection), and the embedding pipeline.

### 3. Ingestion & Transcribe-and-Discard

Speech-to-text and OCR extraction, multilingual normalization and
segmentation, PII detection, and an ingestion orchestrator that
discards binary source material immediately after extracting
provenance-hashed text -- transcribe, extract, discard, never retain
the original binary artifact.

### 4. IRAC Reasoning Tree & Knowledge Layer

The Issue/Rule/Application/Conclusion reasoning-tree model (with its
own non-binding-guardrail primitives), a knowledge graph over the
statute/precedent/ontology corpus, tree assembly/validation/indexing,
and hybrid/adaptive retrieval with citation tracking.

### 5. Reasoning & Adversarial Synthesis

The multi-agent reasoning framework (issue, first-party, second-party,
and synthesis agents), evidence weighing, law application, uncertainty
quantification, and -- critically -- the project-wide non-binding-
analysis guardrail every reasoning output must pass through before it
ever reaches a human. Includes reasoning orchestration, tracing,
grounding, and automated quality evaluation.

### 6. Case Management & Workflow

The case lifecycle state machine, human sign-off gate, case search,
annotations, versioning, notifications, report export, and analytics.

### 7. Security, Compliance & Sovereignty

Encryption and key management, the durable hash-chained audit trail,
data residency and air-gapped deployment support, fine-grained access
governance, data-subject-rights handling, compliance-framework
mapping, threat modeling, adversarial security testing, and
vulnerability/dependency management with SLA-tracked remediation.

### 8. Integration, Operations & Hardening

Backup and disaster recovery, external system integration and bulk
import tooling, localization, performance budgeting and benchmarking,
corpus updates, scalability and reliability engineering, CI/CD
hardening with infrastructure-as-code for cloud/on-prem/air-gapped
deployment tiers, alerting, documentation, full-journey end-to-end
testing, a controlled pilot deployment with a structured feedback
loop, and -- capping the entire build -- a release-readiness gate
(`packages/garelease`) that aggregates every governance, quality, and
security signal produced across all eight parts into one go/no-go
decision before any release is cut.

### What this release does NOT change

The platform's single non-negotiable guarantee is unaffected by GA
status: every reasoning output Verdex produces remains a non-binding
DRAFT ANALYSIS, never a verdict, ruling, or judgment, and never a
substitute for a qualified legal professional or a court. GA hardening
verifies this guarantee holds -- via `packages/garelease`'s live
guardrail-integrity check, among others -- it does not relax it.
