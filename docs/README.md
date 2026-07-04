# Verdex documentation

This is the documentation index: every operator, admin, developer, and
practitioner document in this repository, in one navigable page. It is
the "doc site" — a real index tying together the ~90 packages' own
design docs, not a hosted-site build.

> **Non-binding, everywhere.** Every reasoning surface this platform
> produces is draft analysis only — never a verdict, ruling, or legal
> advice. See [`README.md`](../README.md)'s Non-Binding Disclaimer and
> [`docs/security-compliance/overview.md`](security-compliance/overview.md)
> for how this is enforced in code and audited.

## Start here

| If you are... | Start with |
|---|---|
| New to the codebase | [`docs/architecture/overview.md`](architecture/overview.md) |
| Deploying a new tenant | [`docs/admin/setup-guide.md`](admin/setup-guide.md), then the matching [`docs/deployment/`](deployment/) guide |
| A judge or advocate using the platform | [`docs/user-guide/judges-advocates.md`](user-guide/judges-advocates.md) |
| Operating the platform day to day | [`docs/operations/runbooks.md`](operations/runbooks.md) |
| Responding to an incident right now | [`docs/operations/incident-response.md`](operations/incident-response.md) |
| Integrating against Verdex's APIs | [`docs/api/reference.md`](api/reference.md) |
| Reviewing security/compliance posture | [`docs/security-compliance/overview.md`](security-compliance/overview.md) |

## Higher-level documents (this phase)

| Document | Covers |
|---|---|
| [`docs/architecture/overview.md`](architecture/overview.md) | The 8-part system architecture, naming every real package by phase. |
| [`docs/deployment/cloud.md`](deployment/cloud.md) | Cloud-tier deployment (`packages/iac`, Phase 094). |
| [`docs/deployment/onprem.md`](deployment/onprem.md) | On-premises deployment. |
| [`docs/deployment/airgapped.md`](deployment/airgapped.md) | Fully offline deployment for sensitive courts (`packages/airgapped`, Phase 079). |
| [`docs/admin/setup-guide.md`](admin/setup-guide.md) | First-run tenant provisioning (`packages/setup`, Phase 008; `packages/jurisdiction`, Phase 007). |
| [`docs/user-guide/judges-advocates.md`](user-guide/judges-advocates.md) | The practitioner-facing case-workspace walkthrough. |
| [`docs/operations/runbooks.md`](operations/runbooks.md) | Reliability posture and post-deploy verification. |
| [`docs/operations/incident-response.md`](operations/incident-response.md) | General incident-response runbook. |
| [`docs/api/reference.md`](api/reference.md) | API conventions and the knowledge-layer facade index. |
| [`docs/security-compliance/overview.md`](security-compliance/overview.md) | Part 7 of the architecture, indexed by phase. |
| [`packages/docsite/doc/docsite.md`](../packages/docsite/doc/docsite.md) | This doc site's structure and the link checker that validates it. |

## Every package's own documentation, by architecture part

The tables below are the same Part 1-8 grouping
[`docs/architecture/overview.md`](architecture/overview.md) uses, this
time linking directly to each package's own `doc/*.md` (its
authoritative design doc) rather than summarizing it again. Four
foundational packages —
[`packages/config`](../packages/config),
[`packages/observability`](../packages/observability),
[`packages/persistence`](../packages/persistence), and
[`packages/tenancy`](../packages/tenancy) — are documented via their
own `doc.go` package comment only, with no separate `doc/*.md` file.

### Part 1 — Foundation

| Package | Doc |
|---|---|
| `packages/provider` | [`doc/provider-contract.md`](../packages/provider/doc/provider-contract.md) |
| `packages/router` | [`doc/routing-policy.md`](../packages/router/doc/routing-policy.md) |
| `packages/adapters` | [`doc/adapter-setup.md`](../packages/adapters/doc/adapter-setup.md) |
| `packages/jurisdiction` | [`doc/jurisdiction-schema.md`](../packages/jurisdiction/doc/jurisdiction-schema.md) |
| `packages/setup` | [`doc/setup-api-contract.md`](../packages/setup/doc/setup-api-contract.md) |
| `packages/gateway` | [`doc/api-conventions.md`](../packages/gateway/doc/api-conventions.md) |
| `packages/identity` | [`doc/rbac-matrix.md`](../packages/identity/doc/rbac-matrix.md) |
| `packages/intake` | [`doc/intake-contract.md`](../packages/intake/doc/intake-contract.md) |
| `packages/prompts` | [`doc/template-authoring.md`](../packages/prompts/doc/template-authoring.md) |
| `packages/eval` | [`doc/evaluation-process.md`](../packages/eval/doc/evaluation-process.md) |

### Part 2 — Ingest: transcribe-and-discard

| Package | Doc |
|---|---|
| `packages/stt` | [`doc/stt-pipeline.md`](../packages/stt/doc/stt-pipeline.md) |
| `packages/ocr` | [`doc/ocr-pipeline.md`](../packages/ocr/doc/ocr-pipeline.md) |
| `packages/multilingual` | [`doc/normalization-rules.md`](../packages/multilingual/doc/normalization-rules.md) |
| `packages/segmentation` | [`doc/segmentation-model.md`](../packages/segmentation/doc/segmentation-model.md) |
| `packages/pii` | [`doc/pii-governance.md`](../packages/pii/doc/pii-governance.md) |
| `packages/evidence` | [`doc/evidence-taxonomy.md`](../packages/evidence/doc/evidence-taxonomy.md) |
| `packages/category` | [`doc/category-model.md`](../packages/category/doc/category-model.md) |
| `packages/timeline` | [`doc/party-timeline-model.md`](../packages/timeline/doc/party-timeline-model.md) |
| `packages/ingestion` | [`doc/ingestion-workflow.md`](../packages/ingestion/doc/ingestion-workflow.md) |
| `packages/provenance` | [`doc/custody-model.md`](../packages/provenance/doc/custody-model.md) |

### Part 3 — IRAC reasoning tree schema

| Package | Doc |
|---|---|
| `packages/irac` | [`doc/irac-schema.md`](../packages/irac/doc/irac-schema.md) |
| `packages/issue` | [`doc/issue-extraction.md`](../packages/issue/doc/issue-extraction.md) |
| `packages/fact` | [`doc/fact-model.md`](../packages/fact/doc/fact-model.md) |
| `packages/statute` | [`doc/statute-model.md`](../packages/statute/doc/statute-model.md) |
| `packages/precedent` | [`doc/precedent-model.md`](../packages/precedent/doc/precedent-model.md) |
| `packages/application` | [`doc/application-model.md`](../packages/application/doc/application-model.md) |
| `packages/ontology` | [`doc/legal-ontology.md`](../packages/ontology/doc/legal-ontology.md) |

### Part 4 — IRAC Reasoning Tree & Knowledge Layer

| Package | Doc |
|---|---|
| `packages/graph` | [`doc/graph-layer.md`](../packages/graph/doc/graph-layer.md) |
| `packages/treeassembly` | [`doc/tree-assembly.md`](../packages/treeassembly/doc/tree-assembly.md) |
| `packages/treevalidation` | [`doc/integrity-guarantees.md`](../packages/treevalidation/doc/integrity-guarantees.md) |
| `packages/vectorindex` | [`doc/vector-index.md`](../packages/vectorindex/doc/vector-index.md) |
| `packages/treeindex` | [`doc/tree-indexing.md`](../packages/treeindex/doc/tree-indexing.md) |
| `packages/traversal` | [`doc/graph-traversal.md`](../packages/traversal/doc/graph-traversal.md) |
| `packages/hybridretrieval` | [`doc/hybrid-retrieval.md`](../packages/hybridretrieval/doc/hybrid-retrieval.md) |
| `packages/adaptiveretrieval` | [`doc/adaptive-retrieval.md`](../packages/adaptiveretrieval/doc/adaptive-retrieval.md) |
| `packages/citation` | [`doc/citation-fidelity.md`](../packages/citation/doc/citation-fidelity.md) |
| `packages/knowledgeisolation` | [`doc/knowledge-isolation.md`](../packages/knowledgeisolation/doc/knowledge-isolation.md) |
| `packages/knowledgeapi` | [`doc/knowledge-api.md`](../packages/knowledgeapi/doc/knowledge-api.md) |

### Part 5 — Adversarial reasoning agents and synthesis

| Package | Doc |
|---|---|
| `packages/agentframework` | [`doc/agent-framework.md`](../packages/agentframework/doc/agent-framework.md) |
| `packages/issueagent` | [`doc/issue-agent.md`](../packages/issueagent/doc/issue-agent.md) |
| `packages/firstpartyagent` | [`doc/first-party-agent.md`](../packages/firstpartyagent/doc/first-party-agent.md) |
| `packages/secondpartyagent` | [`doc/second-party-agent.md`](../packages/secondpartyagent/doc/second-party-agent.md) |
| `packages/evidenceweighing` | [`doc/evidence-weighing.md`](../packages/evidenceweighing/doc/evidence-weighing.md) |
| `packages/lawapplication` | [`doc/law-application.md`](../packages/lawapplication/doc/law-application.md) |
| `packages/synthesisagent` | [`doc/synthesis-agent.md`](../packages/synthesisagent/doc/synthesis-agent.md) |
| `packages/uncertainty` | [`doc/uncertainty-surfacing.md`](../packages/uncertainty/doc/uncertainty-surfacing.md) |
| `packages/guardrail` | [`doc/guardrail-policy.md`](../packages/guardrail/doc/guardrail-policy.md) |
| `packages/reasoningprofile` | [`doc/jurisdiction-reasoning.md`](../packages/reasoningprofile/doc/jurisdiction-reasoning.md) |
| `packages/reasoningorchestration` | [`doc/reasoning-orchestration.md`](../packages/reasoningorchestration/doc/reasoning-orchestration.md) |
| `packages/reasoningtrace` | [`doc/reasoning-trace.md`](../packages/reasoningtrace/doc/reasoning-trace.md) |
| `packages/grounding` | [`doc/grounding.md`](../packages/grounding/doc/grounding.md) |
| `packages/reasoningeval` | [`doc/reasoning-quality-evaluation.md`](../packages/reasoningeval/doc/reasoning-quality-evaluation.md) |

### Part 6 — Case lifecycle, judicial workspace UI, and sign-off

| Package / app | Doc |
|---|---|
| `packages/caselifecycle` | [`doc/case-lifecycle.md`](../packages/caselifecycle/doc/case-lifecycle.md) |
| `apps/web` frontend architecture | [`apps/web/docs/frontend-architecture.md`](../apps/web/docs/frontend-architecture.md) |
| `apps/web` ingestion UX | [`apps/web/docs/ingestion-ux.md`](../apps/web/docs/ingestion-ux.md) |
| `apps/web` case workspace | [`apps/web/docs/case-workspace-ux.md`](../apps/web/docs/case-workspace-ux.md) |
| `apps/web` reasoning tree visualization | [`apps/web/docs/tree-visualization.md`](../apps/web/docs/tree-visualization.md) |
| `apps/web` evidence review | [`apps/web/docs/evidence-review.md`](../apps/web/docs/evidence-review.md) |
| `apps/web` draft opinion review | [`apps/web/docs/opinion-review.md`](../apps/web/docs/opinion-review.md) |
| `packages/signoff` | [`doc/signoff-workflow.md`](../packages/signoff/doc/signoff-workflow.md) |
| `apps/web` case search | [`apps/web/docs/case-search-ux.md`](../apps/web/docs/case-search-ux.md) |
| `packages/casesearch` | [`doc/case-search.md`](../packages/casesearch/doc/case-search.md) |
| `apps/web` annotations (discussion) | [`apps/web/docs/annotations-ui.md`](../apps/web/docs/annotations-ui.md) |
| `packages/annotations` | [`doc/annotations.md`](../packages/annotations/doc/annotations.md) |
| `apps/web` case history | [`apps/web/docs/case-history-ui.md`](../apps/web/docs/case-history-ui.md) |
| `packages/caseversioning` | [`doc/case-versioning.md`](../packages/caseversioning/doc/case-versioning.md) |
| `packages/notifications` | [`doc/notifications.md`](../packages/notifications/doc/notifications.md) |
| `packages/reportexport` | [`doc/report-export.md`](../packages/reportexport/doc/report-export.md) |
| `packages/analytics` | [`doc/analytics.md`](../packages/analytics/doc/analytics.md) |

### Part 7 — Security, privacy, and compliance

See [`docs/security-compliance/overview.md`](security-compliance/overview.md)
for the full index with one-line summaries. Direct links:

| Package | Doc |
|---|---|
| `packages/encryption` | [`doc/encryption.md`](../packages/encryption/doc/encryption.md) |
| `packages/keymanagement` | [`doc/key-management.md`](../packages/keymanagement/doc/key-management.md) |
| `packages/auditlog` | [`doc/audit-trail.md`](../packages/auditlog/doc/audit-trail.md) |
| `packages/dataresidency` | [`doc/data-residency.md`](../packages/dataresidency/doc/data-residency.md) |
| `packages/airgapped` | [`doc/airgapped-tier.md`](../packages/airgapped/doc/airgapped-tier.md) |
| `packages/accessgovernance` | [`doc/access-governance.md`](../packages/accessgovernance/doc/access-governance.md) |
| `packages/privacy` | [`doc/privacy.md`](../packages/privacy/doc/privacy.md) |
| `packages/compliance` | [`doc/compliance.md`](../packages/compliance/doc/compliance.md) |
| `packages/threatmodel` | [`doc/threat-model.md`](../packages/threatmodel/doc/threat-model.md) |
| `packages/vulnmanagement` | [`doc/vuln-management.md`](../packages/vulnmanagement/doc/vuln-management.md) |
| `packages/backupdr` | [`doc/backup-dr.md`](../packages/backupdr/doc/backup-dr.md) / [`doc/dr-runbook.md`](../packages/backupdr/doc/dr-runbook.md) |
| `packages/securitytesting` | [`doc/security-testing.md`](../packages/securitytesting/doc/security-testing.md) |

### Part 8 — Platform hardening

| Package | Doc |
|---|---|
| `packages/integration` | [`doc/integration-framework.md`](../packages/integration/doc/integration-framework.md) |
| `packages/bulkimport` | [`doc/bulk-import.md`](../packages/bulkimport/doc/bulk-import.md) |
| `packages/corpusupdater` | [`doc/corpus-updater.md`](../packages/corpusupdater/doc/corpus-updater.md) |
| `packages/localization` | [`doc/localization.md`](../packages/localization/doc/localization.md) |
| `packages/perf` | [`doc/performance.md`](../packages/perf/doc/performance.md) / [`doc/graph-optimization-checklist.md`](../packages/perf/doc/graph-optimization-checklist.md) |
| `packages/scalability` | [`doc/scalability.md`](../packages/scalability/doc/scalability.md) |
| `packages/reliability` | [`doc/reliability.md`](../packages/reliability/doc/reliability.md) |
| `packages/iac` | [`doc/deployment.md`](../packages/iac/doc/deployment.md) |
| `packages/cicdgate` | [`doc/cicd.md`](../packages/cicdgate/doc/cicd.md) |
| `packages/docsite` | [`doc/docsite.md`](../packages/docsite/doc/docsite.md) |

## Keeping this index honest

Every relative link on this page (and every link in every document it
points to, and every existing package's own `doc/*.md`) is checked by
[`packages/docsite`](../packages/docsite)'s `CheckLinks` — run it
yourself with:

```
go run ./packages/docsite/cmd/checklinks .
```

This same command runs as a CI job on every pull request (see
[`.github/workflows/ci.yml`](../.github/workflows/ci.yml)), so a broken
link on this page or anywhere it points to fails the build rather than
sitting silently stale.

## Project-level references

- [`README.md`](../README.md) — the platform's own top-level overview and non-binding disclaimer.
- [`CONTRIBUTING.md`](../CONTRIBUTING.md) — branching, commit, and PR conventions.
- [`.github/pull_request_template.md`](../.github/pull_request_template.md) — the PR template every phase follows.
