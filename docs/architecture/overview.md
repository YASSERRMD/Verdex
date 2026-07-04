# Verdex Architecture Overview

Verdex is a jurisdiction-aware judicial reasoning platform. It ingests case
materials, assembles a structured IRAC (Issue / Rule / Application /
Conclusion) reasoning tree over a retrieval-augmented knowledge layer,
synthesizes an adversarial, evidence-weighed draft opinion, and requires a
qualified human practitioner's sign-off before any output leaves the
system. This document is the single map of how the ~90 packages under
[`packages/`](../../packages) fit together. It does not restate any
package's own design — every section links out to the authoritative
per-package `doc/*.md` file that owns those details.

> **Non-binding, everywhere.** Every part of this architecture below
> Part 1 produces *draft analysis*, never a verdict, ruling, or legal
> advice. This is enforced in code (see [`packages/guardrail`](../../packages/guardrail),
> Phase 057) — it is a hard gate, not a documentation convention a
> deployment can configure away. See
> [`docs/security-compliance/overview.md`](../security-compliance/overview.md)
> for how this guarantee is audited and mapped to compliance controls.

## How to read this document

Verdex was built in eight parts, corresponding to the phased
implementation plan referenced in [`CONTRIBUTING.md`](../../CONTRIBUTING.md).
Each part below names the packages it introduced, in the order they were
built, with a one-line description and a link to that package's own
`doc/*.md`. Where a package's exact phase number is confirmed in its own
`doc.go` or `doc/*.md` header, it is cited; where a package predates that
convention (early Part 1/6 packages), it is described without an invented
number.

---

## Part 1 — Foundation: config, tenancy, identity, and the provider abstraction

The base every later part composes over: multi-tenant scoping, RBAC,
observability, persistence, and the model-agnostic provider layer that
guarantees no phase ever hardcodes an LLM vendor.

| Package | Role |
|---|---|
| [`packages/config`](../../packages/config) | `VERDEX_`-prefixed environment configuration and per-deployment secret-reference schemes (`env://`, `vault://`) that every later manifest and package reuses. |
| [`packages/observability`](../../packages/observability) | Structured logging, metrics, liveness/readiness health endpoints (Phase 003), and the `AuditEvent` concept later durably persisted by `packages/auditlog`. |
| [`packages/persistence`](../../packages/persistence) | Migration runner and storage primitives shared by every stateful package. |
| [`packages/tenancy`](../../packages/tenancy) | Tenant identity and scoping, the axis every cross-tenant-isolation guarantee in this platform is measured against. |
| [`packages/identity`](../../packages/identity) | Role-based access control (RBAC): `Role`/`Permission`/`HasPermission`, the authorization gate almost every later package's `Engine` calls through. |
| [`packages/provider`](../../packages/provider) | The `LLMProvider` interface — see [`doc/provider-contract.md`](../../packages/provider/doc/provider-contract.md). No phase may call a model except through this abstraction (`CONTRIBUTING.md`'s "Provider Abstraction" rule). |
| [`packages/router`](../../packages/router) | Routes requests across configured providers; owns the provider-specific circuit breaker later generalized by `packages/reliability` (Phase 093). |
| [`packages/adapters`](../../packages/adapters) | Concrete `LLMProvider` implementations, including the offline `local` adapter `packages/airgapped` depends on. |
| [`packages/jurisdiction`](../../packages/jurisdiction) | Phase 007: the jurisdiction/court/legal-family registry — see [`doc/jurisdiction-schema.md`](../../packages/jurisdiction/doc/jurisdiction-schema.md). |
| [`packages/setup`](../../packages/setup) | Phase 008: the first-run deployment provisioning wizard — see [`doc/setup-api-contract.md`](../../packages/setup/doc/setup-api-contract.md) and [`docs/admin/setup-guide.md`](../admin/setup-guide.md). |
| [`packages/gateway`](../../packages/gateway) | Phase 009: the HTTP API gateway — versioning, envelopes, rate limiting, CORS — see [`doc/api-conventions.md`](../../packages/gateway/doc/api-conventions.md) and [`docs/api/reference.md`](../api/reference.md). |

## Part 2 — Ingest: transcribe-and-discard

Binary case materials (audio, scanned documents) are transcribed or
extracted, hashed for provenance, and the binary is discarded — only
extracted text ever persists. This is enforced and tested, not merely
documented (`CONTRIBUTING.md`'s "Transcribe-and-Discard" rule).

| Package | Role |
|---|---|
| [`packages/stt`](../../packages/stt) | Speech-to-text pipeline — see [`doc/stt-pipeline.md`](../../packages/stt/doc/stt-pipeline.md). |
| [`packages/ocr`](../../packages/ocr) | Optical character recognition for scanned documents — see [`doc/ocr-pipeline.md`](../../packages/ocr/doc/ocr-pipeline.md). |
| [`packages/multilingual`](../../packages/multilingual) | Multilingual text normalization — see [`doc/normalization-rules.md`](../../packages/multilingual/doc/normalization-rules.md). |
| [`packages/segmentation`](../../packages/segmentation) | Splits normalized transcripts/documents into structured segments — see [`doc/segmentation-model.md`](../../packages/segmentation/doc/segmentation-model.md). |
| [`packages/pii`](../../packages/pii) | PII detection and redaction primitives, reused later by `packages/privacy`'s anonymization — see [`doc/pii-governance.md`](../../packages/pii/doc/pii-governance.md). |
| [`packages/evidence`](../../packages/evidence) | Evidence taxonomy and classification — see [`doc/evidence-taxonomy.md`](../../packages/evidence/doc/evidence-taxonomy.md). |
| [`packages/category`](../../packages/category) | Case category classification. |
| [`packages/timeline`](../../packages/timeline) | Party/event timeline model — see [`doc/party-timeline-model.md`](../../packages/timeline/doc/party-timeline-model.md). |
| [`packages/ingestion`](../../packages/ingestion) | Orchestrates the ingest pipeline end to end, including the per-stage idempotency ledger later generalized by `packages/reliability` — see [`doc/ingestion-workflow.md`](../../packages/ingestion/doc/ingestion-workflow.md). |
| [`packages/provenance`](../../packages/provenance) | Phase 020: hash + chain-of-custody model for every discarded artifact — see [`doc/custody-model.md`](../../packages/provenance/doc/custody-model.md). |

## Part 3 — IRAC reasoning tree schema

The Issue / Rule / Fact / Application / Conclusion node schema — established
once (Phase 031) and never modified afterward — plus the domain models
(statute, precedent, legal ontology) that populate `Rule` nodes.

| Package | Role |
|---|---|
| [`packages/irac`](../../packages/irac) | Phase 031: the IRAC node-type schema itself — see [`doc/irac-schema.md`](../../packages/irac/doc/irac-schema.md). Validation only; no storage backend (that is Part 4). |
| [`packages/issue`](../../packages/issue) | Phase 033: issue-node extraction — see [`doc/issue-extraction.md`](../../packages/issue/doc/issue-extraction.md). |
| [`packages/fact`](../../packages/fact) | Phase 034: fact-node construction — see [`doc/fact-model.md`](../../packages/fact/doc/fact-model.md). |
| [`packages/statute`](../../packages/statute) | Phase 035: statute-model representation — see [`doc/statute-model.md`](../../packages/statute/doc/statute-model.md). |
| [`packages/precedent`](../../packages/precedent) | Phase 036: precedent/holding-node model — see [`doc/precedent-model.md`](../../packages/precedent/doc/precedent-model.md). |
| [`packages/application`](../../packages/application) | Phase 037: `Origin`/`OriginatedRule`/`WeightByLegalFamily` — how a rule's legal-family origin weights its application — see [`doc/application-model.md`](../../packages/application/doc/application-model.md). |
| [`packages/ontology`](../../packages/ontology) | Legal concept ontology underlying rule/statute/precedent cross-referencing — see [`doc/legal-ontology.md`](../../packages/ontology/doc/legal-ontology.md). |

## Part 4 — IRAC Reasoning Tree & Knowledge Layer

The graph/vector storage, assembly, indexing, and retrieval stack over the
Part 3 schema, capped by `packages/knowledgeapi`'s single stable internal
contract (Phases 041-048 per that package's own doc).

| Package | Role |
|---|---|
| [`packages/graph`](../../packages/graph) | Phase 032: graph-database storage for IRAC nodes/edges — see [`doc/graph-layer.md`](../../packages/graph/doc/graph-layer.md). |
| [`packages/treeassembly`](../../packages/treeassembly) | Phase 039: composes issue/fact/rule/application nodes into one assembled tree per case — see [`doc/tree-assembly.md`](../../packages/treeassembly/doc/tree-assembly.md). |
| [`packages/treevalidation`](../../packages/treevalidation) | Phase 040: the capstone integrity/validation gate for an assembled tree — see [`doc/integrity-guarantees.md`](../../packages/treevalidation/doc/integrity-guarantees.md). |
| [`packages/vectorindex`](../../packages/vectorindex) | Phase 041: semantic-recall vector index over assembled trees — see [`doc/vector-index.md`](../../packages/vectorindex/doc/vector-index.md). |
| [`packages/treeindex`](../../packages/treeindex) | Structured path lookups over an assembled tree. |
| [`packages/traversal`](../../packages/traversal) | Phase 043: dynamic multi-hop graph query layer — see [`doc/graph-traversal.md`](../../packages/traversal/doc/graph-traversal.md). |
| [`packages/hybridretrieval`](../../packages/hybridretrieval) | Phase 044: fused graph + vector retrieval entry point — see [`doc/hybrid-retrieval.md`](../../packages/hybridretrieval/doc/hybrid-retrieval.md). |
| [`packages/adaptiveretrieval`](../../packages/adaptiveretrieval) | Phase 045: cost-bounded, on-demand retrieval over the same fused stack — see [`doc/adaptive-retrieval.md`](../../packages/adaptiveretrieval/doc/adaptive-retrieval.md). |
| [`packages/citation`](../../packages/citation) | Phase 046: citation-fidelity guarantee — every retrieved unit of law traces to a verifiable source — see [`doc/citation-fidelity.md`](../../packages/citation/doc/citation-fidelity.md). |
| [`packages/knowledgeisolation`](../../packages/knowledgeisolation) | Phase 047: cross-case isolation enforcement — a case's retrieval never leaks another case's facts — see [`doc/knowledge-isolation.md`](../../packages/knowledgeisolation/doc/knowledge-isolation.md). |
| [`packages/knowledgeapi`](../../packages/knowledgeapi) | Phase 048: the capstone stable internal facade over every package above — see [`doc/knowledge-api.md`](../../packages/knowledgeapi/doc/knowledge-api.md) and [`docs/api/reference.md`](../api/reference.md). |

## Part 5 — Adversarial reasoning agents and draft-opinion synthesis

Issue-scoped agents construct both parties' strongest arguments, evidence
is weighed, uncertainty is surfaced rather than hidden, and a synthesis
agent composes the final draft opinion — always behind the guardrail from
Phase 057.

| Package | Role |
|---|---|
| [`packages/agentframework`](../../packages/agentframework) | Phase 049: shared scaffolding every reasoning agent below is built on. |
| [`packages/issueagent`](../../packages/issueagent) | Phase 050: per-issue reasoning agent, reading `irac.IssueNode`s. |
| [`packages/firstpartyagent`](../../packages/firstpartyagent) | Phase 051: constructs the first party's strongest argument. |
| [`packages/secondpartyagent`](../../packages/secondpartyagent) | Phase 051: constructs the second party's strongest argument, consuming the same `issueagent` output. |
| [`packages/evidenceweighing`](../../packages/evidenceweighing) | Phase 053: weighs evidence by kind and legal-family-adjusted authority — see [`doc/evidence-weighing.md`](../../packages/evidenceweighing/doc/evidence-weighing.md). |
| [`packages/lawapplication`](../../packages/lawapplication) | Phase 054: applies weighed rules to weighed evidence — see [`doc/law-application.md`](../../packages/lawapplication/doc/law-application.md). |
| [`packages/synthesisagent`](../../packages/synthesisagent) | Phase 055: composes the final draft reasoned opinion — see [`doc/synthesis-agent.md`](../../packages/synthesisagent/doc/synthesis-agent.md). Never itself emits verdict language; that is the guardrail's job. |
| [`packages/uncertainty`](../../packages/uncertainty) | Surfaces reasoning uncertainty/weakest-link callouts rather than hiding them. |
| [`packages/guardrail`](../../packages/guardrail) | Phase 057: the non-binding guardrail — blocks verdict/directive language in code, defines `SignoffGate`'s seam (filled by Phase 068) — see [`doc/guardrail-policy.md`](../../packages/guardrail/doc/guardrail-policy.md). |
| [`packages/reasoningprofile`](../../packages/reasoningprofile) | Phase 058: per-jurisdiction reasoning-weight profiles — see [`doc/jurisdiction-reasoning.md`](../../packages/reasoningprofile/doc/jurisdiction-reasoning.md). |
| [`packages/reasoningorchestration`](../../packages/reasoningorchestration) | Phase 059: orchestrates the full per-case reasoning pipeline across the agents above. |
| [`packages/reasoningtrace`](../../packages/reasoningtrace) | Phase 060: traces every conclusion back through the reasoning pipeline that produced it — see [`doc/reasoning-trace.md`](../../packages/reasoningtrace/doc/reasoning-trace.md). |
| [`packages/grounding`](../../packages/grounding) | Verifies every synthesized claim grounds back to a retrieved source node. |
| [`packages/reasoningeval`](../../packages/reasoningeval) | Phase 062: reasoning-quality evaluation harness — see [`doc/reasoning-quality-evaluation.md`](../../packages/reasoningeval/doc/reasoning-quality-evaluation.md). |

## Part 6 — Case lifecycle, judicial workspace UI, and human sign-off

The case-management backend and the judicial-facing UI
([`apps/web`](../../apps/web)) that makes the reasoning output above
reviewable, annotatable, and — critically — never usable without a
recorded human sign-off.

| Package / app | Role |
|---|---|
| [`packages/caselifecycle`](../../packages/caselifecycle) | Phase 062: case state machine (draft → active → under_review → closed → archived). |
| [`apps/web` — case workspace](../../apps/web/docs/case-workspace-ux.md) | Phase 064: the unified judicial-facing case view (`/cases/[caseId]`) every other UI panel below mounts into. |
| [`apps/web` — reasoning tree visualization](../../apps/web/docs/tree-visualization.md) | Phase 065: interactive IRAC tree graph rendering. |
| [`apps/web` — evidence review](../../apps/web/docs/evidence-review.md) | Phase 066: evidence classification/correction UI. |
| [`apps/web` — draft opinion review](../../apps/web/docs/opinion-review.md) | Phase 067: full per-issue draft-opinion reviewer UI, always rendering the disclaimer first. |
| [`packages/signoff`](../../packages/signoff) | Phase 068: the mandatory human sign-off workflow — the first real, persisted implementation of `guardrail.SignoffGate` — see [`doc/signoff-workflow.md`](../../packages/signoff/doc/signoff-workflow.md). No case output is usable without it. |
| [`apps/web` — case search](../../apps/web/docs/case-search-ux.md) | Phase 069: cross-case search UI — see also [`packages/casesearch`](../../packages/casesearch). |
| [`apps/web` — annotations (discussion)](../../apps/web/docs/annotations-ui.md) | Multi-user notes/threaded discussion on a case — see [`packages/annotations`](../../packages/annotations). |
| [`apps/web` — case history](../../apps/web/docs/case-history-ui.md) | Case-level version history and diff/restore — see [`packages/caseversioning`](../../packages/caseversioning). |
| [`packages/notifications`](../../packages/notifications) | Persisted notification inbox for case/review events. |
| [`packages/reportexport`](../../packages/reportexport) | Exports a signed-off case's draft opinion and supporting record. |
| [`packages/analytics`](../../packages/analytics) | Cross-case operational analytics and dashboards. |

See [`docs/user-guide/judges-advocates.md`](../user-guide/judges-advocates.md)
for the practitioner-facing walkthrough of this part.

## Part 7 — Security, privacy, and compliance

Encryption, key management, the durable audit trail, data-subject rights,
access governance, and the compliance-control mapping that ties them all
to named regulatory frameworks — plus the offline/air-gapped deployment
tier for the most sensitive courts.

| Package | Role |
|---|---|
| [`packages/encryption`](../../packages/encryption) | Phase 075: encryption at rest and in transit — see [`doc/encryption.md`](../../packages/encryption/doc/encryption.md). |
| [`packages/keymanagement`](../../packages/keymanagement) | Phase 076: mandatory, centralized secret and key handling — see [`doc/key-management.md`](../../packages/keymanagement/doc/key-management.md). |
| [`packages/auditlog`](../../packages/auditlog) | Phase 077: the durable, hash-chained, immutable audit trail every later security package writes to. |
| [`packages/dataresidency`](../../packages/dataresidency) | Phase 078: enforces that a deployment's data stays within its declared jurisdiction boundary. |
| [`packages/airgapped`](../../packages/airgapped) | Phase 079: the fully offline deployment tier for sensitive courts — see [`docs/deployment/airgapped.md`](../deployment/airgapped.md). |
| [`packages/accessgovernance`](../../packages/accessgovernance) | Phase 080: certification/reporting capstone drawing together RBAC, case lifecycle, key-management break-glass, and the audit trail. |
| [`packages/privacy`](../../packages/privacy) | Phase 081: data-subject rights (SAR, erasure, consent, retention) — see [`doc/privacy.md`](../../packages/privacy/doc/privacy.md). |
| [`packages/compliance`](../../packages/compliance) | Phase 082: maps platform controls to named regulatory frameworks — see [`doc/compliance.md`](../../packages/compliance/doc/compliance.md) and [`docs/security-compliance/overview.md`](../security-compliance/overview.md). |
| [`packages/threatmodel`](../../packages/threatmodel) | Phase 083: structured STRIDE-style threat catalogue per platform component. |
| [`packages/vulnmanagement`](../../packages/vulnmanagement) | Phase 084: continuous SCA/SAST/container vulnerability detection and remediation-SLA tracking, wired into CI. |
| [`packages/backupdr`](../../packages/backupdr) | Phase 085: backup and disaster recovery — see [`doc/dr-runbook.md`](../../packages/backupdr/doc/dr-runbook.md) and [`docs/operations/incident-response.md`](../operations/incident-response.md). |
| [`packages/securitytesting`](../../packages/securitytesting) | Phase 086: adversarial validation harness proving defenses hold under attack. |

See [`docs/security-compliance/overview.md`](../security-compliance/overview.md)
for the full index of this part's packages by phase.

## Part 8 — Platform hardening: integration, scale, reliability, delivery

Operational maturity: external-system integration, localization,
bulk-migration tooling, performance budgets, horizontal scalability,
graceful degradation, deployment automation, and CI/CD hardening.

| Package | Role |
|---|---|
| [`packages/integration`](../../packages/integration) | Phase 087: external system integration framework. |
| [`packages/bulkimport`](../../packages/bulkimport) | Phase 088: bulk import and migration tooling. |
| [`packages/corpusupdater`](../../packages/corpusupdater) | Phase 089: keeps the statute/precedent corpus current post-launch. |
| [`packages/localization`](../../packages/localization) | Phase 090: localization and i18n. |
| [`packages/perf`](../../packages/perf) | Phase 091: performance budgets and regression tracking. |
| [`packages/scalability`](../../packages/scalability) | Phase 092: horizontal scaling. |
| [`packages/reliability`](../../packages/reliability) | Phase 093: graceful degradation and fault tolerance (retry, circuit breaker, degradation, idempotency, traffic shifting, SLO/error-budget) — see [`docs/operations/runbooks.md`](../operations/runbooks.md). |
| [`packages/iac`](../../packages/iac) | Phase 094: reproducible deployments across cloud/on-prem/air-gapped tiers, plus real manifests under [`infra/`](../../infra) — see [`docs/deployment/`](../deployment/). |
| [`packages/cicdgate`](../../packages/cicdgate) | Phase 095: delivery-pipeline hardening — branch/commit policy, signed builds, staged rollout — wired into [`.github/workflows/ci.yml`](../../.github/workflows/ci.yml). |
| [`packages/docsite`](../../packages/docsite) | Phase 098 (this phase): the internal-link checker validating every document referenced from this page actually resolves — see [`doc/docsite.md`](../../packages/docsite/doc/docsite.md). |

---

## Where to go next

- Deploying a new tenant: [`docs/deployment/`](../deployment/) and [`docs/admin/setup-guide.md`](../admin/setup-guide.md)
- Running the platform day to day: [`docs/operations/runbooks.md`](../operations/runbooks.md)
- Responding to an incident: [`docs/operations/incident-response.md`](../operations/incident-response.md)
- Judges and advocates using the case workspace: [`docs/user-guide/judges-advocates.md`](../user-guide/judges-advocates.md)
- Integrating against Verdex's APIs: [`docs/api/reference.md`](../api/reference.md)
- Security, privacy, and compliance posture: [`docs/security-compliance/overview.md`](../security-compliance/overview.md)
- The full documentation index: [`docs/README.md`](../README.md)
