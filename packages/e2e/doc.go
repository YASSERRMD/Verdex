// Package e2e is Phase 097: a full-journey, jurisdiction-aware
// automated test suite proving this platform's major phases actually
// compose end to end, in process, with no Docker/testcontainers and no
// real LLM provider call anywhere. It draws on the ingestion
// orchestrator added in Phase 029 (packages/ingestion), the reasoning
// pipeline orchestrator added in Phase 059 (packages/reasoningorchestration),
// the mandatory human sign-off workflow added in Phase 068
// (packages/signoff), the non-binding guardrail added in Phase 057
// (packages/guardrail), the cross-case isolation guard added in Phase
// 047 (packages/knowledgeisolation), the case-category taxonomy added
// in Phase 027 (packages/category), the jurisdiction/legal-family model
// added in Phase 007 (packages/jurisdiction), the per-family reasoning
// weights added in Phase 058 (packages/reasoningprofile), the
// multilingual normalization pipeline added in Phase 023
// (packages/multilingual), and the adversarial test-harness shape
// established in Phase 086 (packages/securitytesting, referenced by
// design, not imported), composing them into one end-to-end test suite
// rather than duplicating any of them.
//
// # This package IS the composition, not a description of it
//
// Every scenario in this package genuinely executes real code: real
// calls through ingestion.IngestionOrchestrator.Process (the exact
// entry point packages/perf's own BenchmarkIngestion_Process
// exercises), real calls through reasoningorchestration.Run (the exact
// entry point that package's own test suite exercises), and real calls
// through signoff.Service.Approve / guardrail.CanFinalize. No scenario
// in this package is a stub, a mock of another phase's behavior, or a
// prose description of what a journey "should" do — see
// journey.go's runFullJourney for the shared composition every
// setup-to-opinion scenario variant drives.
//
// # Composition table
//
//	| Prior phase                         | What it already provides                                    | How this suite uses it |
//	|--------------------------------------|--------------------------------------------------------------|------------------------|
//	| Phase 029 packages/ingestion          | IngestionOrchestrator.Process: intake->STT/OCR->normalize->segment->classify | Every scenario's intake phase (ingestion_journey.go), wired exactly like packages/perf's benchmark helper |
//	| Phase 059 packages/reasoningorchestration | Run/Resume: issue framing->arguments->weighing->application->synthesis->uncertainty->guardrail | Every scenario's reasoning phase (journey.go), driven by a deterministic sequencedProvider |
//	| Phase 068 packages/signoff             | Service.Approve/Reject, GateImpl (a real guardrail.SignoffGate) | signoff_scenario.go's enforcement proof |
//	| Phase 057 packages/guardrail           | CheckText, CanFinalize, NoSignoffRecordedGate                | Every scenario reads GuardrailApproved back from a real Checkpoint; signoff_scenario.go drives CanFinalize directly |
//	| Phase 047 packages/knowledgeisolation   | CaseScopedStore: cross-case isolation within a tenant          | Every journey's KnowledgeAPI is built on it (fixture.go); isolation_scenario.go re-verifies the guarantee directly |
//	| Phase 027 packages/category             | CategoryCode taxonomy (civil/criminal/domestic-violence/consumer/...) | Every Scenario's CaseCategory() (types.go) |
//	| Phase 007 packages/jurisdiction         | LegalFamily enum, SeedData() real jurisdiction catalogue        | jurisdiction_scenario.go's multi-jurisdiction variant |
//	| Phase 058 packages/reasoningprofile     | WeightsForFamily: per-legal-family reasoning weight profile     | jurisdiction_scenario.go asserts two families resolve two distinct profiles |
//	| Phase 023 packages/multilingual         | NormalizationService.Normalize: Unicode/script/language/RTL/tokenize | multilingual_scenario.go's real Arabic/Urdu/Tamil/English samples |
//	| Phase 019 packages/intake               | IntakeService.Ingest with TTL-scheduled discard                | discard_scenario.go's real post-processing state assertion |
//	| Phase 086 packages/securitytesting      | Scenario/Harness interface shape (referenced by design only)   | This package's own Scenario/Suite mirror the same Name/Category/Run vocabulary without importing it (see suite.go) |
//
// # Primary types
//
//   - Scenario / ScenarioFunc (types.go): the named,
//     case-category-classified journey interface every scenario in
//     this package implements — Name/CaseCategory/Run, mirroring
//     packages/securitytesting.Scenario's shape by design, not by
//     import.
//   - Suite (suite.go): a lightweight local runner over a fixed
//     Scenario collection, built local to this package rather than
//     importing packages/securitytesting.Harness (see suite.go's own
//     doc comment for exactly why that import does not fit cleanly).
//   - runFullJourney (journey.go): the shared setup-to-opinion journey
//     composition every civil/criminal/domestic-violence/consumer
//     scenario variant (task 2) and the multi-jurisdiction variant
//     (task 3) drives, parameterized by jurisdiction and sign-off gate.
//   - RetryOnFlake / QuarantineList (flaky.go): honest flaky-test
//     controls (task 9) — retries only genuinely non-deterministic
//     timing, never masks a real defect.
//
// # What is new here
//
//   - Scenario definitions per case category (task 1, types.go,
//     civil_scenario.go, criminal_scenario.go,
//     domesticviolence_scenario.go, consumer_scenario.go).
//   - Setup-to-opinion journey tests (task 2, journey.go plus the four
//     category-specific scenario files above): real, in-process,
//     no-Docker journeys through ingestion, reasoning, and the
//     guardrail check.
//   - Multi-jurisdiction variants (task 3, jurisdiction_scenario.go):
//     the same scenario, parameterized by
//     jurisdiction.LegalFamily, asserting two different families
//     resolve two different reasoningprofile.Weights profiles.
//   - Multilingual ingestion tests (task 4, multilingual_scenario.go):
//     real Arabic/Urdu/Tamil/English text through
//     multilingual.NormalizationService.Normalize.
//   - Discard-guarantee verification (task 5, discard_scenario.go):
//     a real intake.IntakeService.Ingest call, asserting
//     IntakeResult.Status()/DiscardedAt() after the TTL elapses.
//   - Sign-off enforcement tests (task 6, signoff_scenario.go): real
//     signoff.Service.Approve calls gating a real
//     guardrail.CanFinalize check.
//   - Data-isolation tests (task 7, isolation_scenario.go): a real
//     knowledgeisolation.CaseScopedStore cross-case rejection proof.
//   - CI integration (task 8, .github/workflows/ci.yml): an additive
//     `go test ./packages/e2e/...` job feeding the existing gate job.
//   - Flaky-test controls (task 9, flaky.go): RetryOnFlake and
//     QuarantineList.
//   - This doc.go and doc/e2e-suite.md (task 10): the full write-up.
//
// # What is explicitly reused, not duplicated
//
//   - ingestion.IngestionOrchestrator, reasoningorchestration.Run/Resume,
//     signoff.Service, guardrail.CanFinalize/CheckText,
//     knowledgeisolation.CaseScopedStore, multilingual.NormalizationService,
//     and intake.IntakeService remain the only implementations of their
//     respective pipelines in this codebase; every scenario in this
//     package calls through their existing public constructors and
//     methods rather than reimplementing, wrapping with new logic, or
//     forking any of them.
//   - packages/reasoningprofile's WeightsForFamily/Family remain the
//     only per-legal-family reasoning-weight resolution in this
//     codebase; jurisdiction_scenario.go calls it directly rather than
//     recomputing weights.
//   - packages/jurisdiction's SeedData() remains the only jurisdiction
//     catalogue in this codebase; jurisdiction_scenario.go looks a
//     country code up in it rather than hardcoding a second copy of
//     the legal-family mapping.
//   - packages/securitytesting's Scenario/Harness naming vocabulary
//     (Name/Category/Run, RunAll/RunOne, FailedRecords/AllPassed) is
//     the structural precedent this package's Scenario/Suite follow —
//     deliberately mirrored, not imported (see suite.go's doc comment
//     for the specific reason the import does not fit).
//   - packages/tenancy.WithTenantScope's Postgres-Row-Level-Security
//     cross-tenant guarantee is documented by name in
//     isolation_scenario.go rather than re-exercised here: that
//     guarantee requires a live Postgres connection with RLS enabled,
//     exactly the Docker/testcontainers dependency this phase's brief
//     says to skip. packages/tenancy/integration_test.go is the real,
//     Postgres-verified test for that specific axis.
//
// # What this suite does NOT cover
//
//   - No real LLM provider call, anywhere. Every reasoning-stage model
//     call in this suite is answered by a deterministic,
//     provider-agnostic sequencedProvider (fixture.go) implementing
//     provider.LLMProvider — the same no-op-provider convention Phase
//     011 established (provider.NoOpProvider). This suite proves the
//     PIPELINE composes correctly, not that any specific model
//     produces good legal reasoning.
//   - No Docker, no testcontainers, no live Postgres. Every store this
//     suite touches is an in-memory implementation
//     (graph.InMemoryGraphStore, vectorindex.NewInMemoryVectorStore,
//     reasoningorchestration.InMemoryCheckpointStore,
//     signoff.InMemoryRepository, intake's in-process TempBuffer).
//     packages/tenancy's Postgres-Row-Level-Security cross-tenant
//     guarantee specifically is NOT re-verified here (see above) — its
//     own integration test is the real verification for that axis.
//   - No real STT/OCR transcription. packages/stt's NoOpSTTProvider
//     ignores audio content entirely (see fixture.go); this suite's
//     multilingual scenario therefore drives real text directly
//     through packages/multilingual.NormalizationService rather than
//     through the ingestion pipeline's transcription step, since the
//     no-op STT provider cannot carry real per-script content through
//     that path (see multilingual_scenario.go's own doc comment).
//   - No real STT/OCR/embedding vendor integration testing of any
//     kind — every extraction/embedding call in this suite's ingestion
//     phase resolves to a "noop" provider, consistent with every prior
//     phase's own no-Docker test convention.
//   - No load, chaos, or long-running soak testing. See packages/perf
//     (Phase 091) for benchmarking and packages/reliability for chaos
//     testing; this suite is functional correctness across a full
//     journey, not a performance or resilience test.
//   - No durable persistence of run history. Mirroring
//     packages/perf's own Phase 091 in-memory-only precedent (see its
//     doc.go's "no packages/identity.Permission constants" note): this
//     package has no tenant-scoped repository, no Engine-style
//     permission-gated façade, and no migration — it is a test suite
//     invoked by CI and engineers, not a tenant-facing service with
//     durable secrets to protect. Should a future phase want durable,
//     queryable E2E run history, that phase should add the
//     PermView*/PermManage* pair and migration then, following the
//     exact precedent packages/compliance/packages/securitytesting/etc.
//     already established.
//
// See doc/e2e-suite.md for the full write-up.
package e2e
