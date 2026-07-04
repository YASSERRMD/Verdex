# End-to-end testing suite (Phase 097)

This phase adds `packages/e2e`: a full-journey, jurisdiction-aware,
automated test suite proving that this platform's major phases
genuinely compose end to end -- in process, in memory, with no Docker
or testcontainers, and no real LLM provider call anywhere.

## Goal

Every prior phase in this codebase already has its own unit test
suite. What none of them individually proves is that a real case
actually flows all the way from intake through a draft opinion, that
the pipeline behaves differently (correctly) across jurisdictions,
that multilingual input is handled sensibly, that discarded binaries
stay discarded, that sign-off actually blocks finalization until
recorded, and that one case's facts can never leak into another's
reasoning. This phase closes that gap with real, executing journeys,
not descriptions of them.

## What this package composes from, versus what is new

| Existing piece | What it already provides | What this phase adds |
|---|---|---|
| `packages/ingestion` (Phase 029) | `IngestionOrchestrator.Process`: intake -> STT/OCR extraction -> normalize -> segment -> classify, the exact entry point `packages/perf`'s own benchmark exercises | Every scenario's intake phase (`ingestion_journey.go`), wired identically to `packages/perf/benchmark_ingestion_test.go`'s helper |
| `packages/reasoningorchestration` (Phase 059) | `Run`/`Resume`: issue framing -> first/second-party arguments -> evidence weighing -> law application -> synthesis -> uncertainty surfacing -> guardrail check | Every scenario's reasoning phase (`journey.go`), driven by a deterministic `sequencedProvider` so no real LLM call is ever made |
| `packages/signoff` (Phase 068) | `Service.Approve`/`Reject`, `GateImpl` (a real `guardrail.SignoffGate`) | `signoff_scenario.go`'s enforcement proof: blocked before, approved after |
| `packages/guardrail` (Phase 057) | `CheckText`, `CanFinalize`, `NoSignoffRecordedGate` | Every scenario reads `GuardrailApproved` back from a real `Checkpoint`; `signoff_scenario.go` drives `CanFinalize` directly |
| `packages/knowledgeisolation` (Phase 047) | `CaseScopedStore`: cross-case isolation within a tenant | Every journey's `KnowledgeAPI` is built on it (`fixture.go`); `isolation_scenario.go` re-verifies the guarantee directly against a shared store |
| `packages/category` (Phase 027) | `CategoryCode` taxonomy: civil/criminal/domestic-violence/consumer/... | Every `Scenario.CaseCategory()` (`types.go`) |
| `packages/jurisdiction` (Phase 007) | `LegalFamily` enum, real `SeedData()` jurisdiction catalogue | `jurisdiction_scenario.go`'s multi-jurisdiction variant looks a country up in the real catalogue rather than duplicating it |
| `packages/reasoningprofile` (Phase 058) | `WeightsForFamily`: per-legal-family reasoning weight profile | `jurisdiction_scenario.go` asserts two different families resolve two genuinely distinct profiles |
| `packages/multilingual` (Phase 023) | `NormalizationService.Normalize`: Unicode/script/language/RTL/tokenize pipeline | `multilingual_scenario.go`'s real Arabic/Urdu/Tamil/English samples |
| `packages/intake` (Phase 019) | `IntakeService.Ingest` with TTL-scheduled discard | `discard_scenario.go`'s real post-processing state assertion |
| `packages/securitytesting` (Phase 086) | `Scenario`/`Harness` interface shape (Name/Category/Run) | This package's own `Scenario`/`Suite` mirror the same vocabulary by design, not by import -- see "Suite, not Harness" below |

## The centerpiece: `runFullJourney`

`journey.go`'s `runFullJourney` is what every setup-to-opinion scenario
variant actually drives. It:

1. Builds a `journeyFixture` (`fixture.go`): a real
   `knowledgeapi.KnowledgeAPI` over an isolated, case-scoped
   `graph.InMemoryGraphStore`/`vectorindex` pair (via
   `knowledgeisolation.NewCaseScopedStore`/`NewCaseScopedVectorStore`),
   and a real `ingestion.IngestionOrchestrator` wired with an isolated
   `stt.Registry` carrying `stt.DefaultNoOpSTTProvider` -- mirroring
   `packages/reasoningorchestration`'s own test fixture and
   `packages/perf/benchmark_ingestion_test.go`'s wiring pattern
   line-for-line.
2. Seeds a minimal-but-real reasoning tree: one `irac.IssueNode`, one
   governing `irac.RuleNode`, one supporting `irac.FactNode`.
3. Drives a synthetic exhibit through
   `IngestionOrchestrator.Process` end to end -- the exact same call
   `packages/perf`'s `BenchmarkIngestion_Process` benchmarks.
4. Drives the case through `reasoningorchestration.Run` end to end,
   against a deterministic `sequencedProvider` (a
   `provider.LLMProvider` fake returning one fixed, schema-shaped JSON
   response per call, copied from
   `packages/reasoningorchestration`'s own test fixtures), resolving
   `reasoningprofile.Weights` for the case's legal family alongside it.

**No real LLM provider call happens anywhere in this suite.** Every
reasoning-stage model call is answered by `sequencedProvider`, the same
no-op-provider convention Phase 011 established
(`provider.NoOpProvider`). This suite proves the *pipeline* composes
correctly end to end -- not that any specific model produces good
legal reasoning.

### A real authentication bug this suite caught during development

`reasoningorchestration.Run`'s tree-reading stages fetch through
`knowledgeapi.KnowledgeAPI`, which requires an authenticated
`identity.User` on `ctx`. The first working version of `runFullJourney`
propagated the caller's raw, unauthenticated `ctx` straight into `Run`,
and every journey failed at `StageIssueFraming` with `knowledgeapi:
unauthenticated request`. This was caught by this suite's own tests
during development (see `go test ./packages/e2e/... -v`'s failure
output) and fixed by wrapping the reasoning-phase call in an
authenticated context, mirroring
`packages/reasoningorchestration/helpers_test.go`'s own
`authedContext()` convention. This is exactly the class of
cross-package composition bug a suite like this exists to catch.

## Scenario definitions per case category (task 1)

`Scenario` (`types.go`) is the extension point: `Name() string`,
`CaseCategory() category.CategoryCode`, `Run(ctx) (ScenarioResult,
error)`. `ScenarioFunc` adapts a plain function to it, mirroring
`packages/securitytesting.ScenarioFunc`'s `http.HandlerFunc`-style
adapter.

Four setup-to-opinion scenarios cover the case categories the brief
names by example:

- `civil/setup-to-opinion` -- US-CA, common-law
- `criminal/setup-to-opinion` -- UAE, mixed legal family
- `domestic-violence/setup-to-opinion` -- UK, common-law
- `consumer/setup-to-opinion` -- Egypt, civil-law

Each asserts real properties of the journey it just ran: ingestion
reached `StageComplete`, the reasoning pipeline reached
`TerminationComplete` with all 8 stages recorded, and (since no
sign-off has been recorded for a fresh case) the guardrail check
correctly reports `GuardrailApproved == false` -- the fail-closed
default.

## Multi-jurisdiction variants (task 3)

`jurisdiction_scenario.go`'s `NewMultiJurisdictionScenario(countryCode)`
parameterizes the *same* civil journey by a real seeded
`jurisdiction.Jurisdiction` (looked up in `jurisdiction.SeedData()` by
country code, not duplicated). The critical assertion is
`AssertDistinctJurisdictionProfiles`: two different legal families
(e.g. `US`/common_law and `SA`/islamic_law) must resolve two
*genuinely different* `reasoningprofile.Weights` profiles -- not merely
"both runs completed without erroring." A same-family negative control
(`US`/`GB`, both common_law) proves the assertion correctly rejects a
pairing that should NOT be flagged as distinct.

## Multilingual ingestion (task 4)

`multilingual_scenario.go` feeds real Arabic, Urdu, Tamil, and English
legal prose directly through `multilingual.NormalizationService.Normalize`.
The ingestion pipeline's own `stt.NoOpSTTProvider` ignores audio content
entirely (see its doc comment: "It never inspects input.Data content"),
so a multilingual scenario cannot meaningfully drive different
per-script content through the *ingestion* pipeline's transcription
step -- this scenario instead calls `NormalizationService.Normalize`
directly, which is exactly what the brief specifies. Every sample was
verified to contain the correct disambiguating code points before being
committed (the Urdu sample genuinely contains an Urdu-only Arabic-script
letter per `multilingual`'s own `urduOnlyRanges`, so `DetectLanguage`
correctly distinguishes it from Arabic). The assertion checks detected
`Script`/`Language`/`IsRTL`/token count each differ sensibly, not merely
that the call succeeded.

## Discard-guarantee verification (task 5)

`discard_scenario.go` calls the real `intake.IntakeService.Ingest`
pipeline with a short TTL, then asserts the service's own
post-processing state after the TTL elapses:
`IntakeResult.Status() == StatusDiscarded`, `DiscardedAt()` populated,
and a subsequent `DiscardAll` call remaining a safe no-op --
mirroring `packages/intake/discard_test.go`'s own
`TestIngest_BufferDiscardedAfterTTL` assertion shape exactly, called
through the same service a real deployment uses.

## Sign-off enforcement (task 6)

`signoff_scenario.go` proves, against the real `signoff.Service` and
`guardrail.CanFinalize` gate:

1. A fresh case with no recorded sign-off cannot finalize
   (`CanFinalize` reports `false`, the case is `SignoffPending`).
2. `Approve` rejects an incorrect acknowledgement phrase
   (`ErrAcknowledgementRequired`) -- the requirement is not
   satisfiable by an approximate string.
3. A real, correctly-acknowledged `Approve` decision flips
   `CanFinalize` to `true`.

This package's own `staticCaseVersionReader` is a minimal, local
`signoff.CaseVersionReader` fake (mirroring
`packages/signoff/helpers_test.go`'s own `fakeCaseVersionReader`
convention) rather than importing `packages/caselifecycle`'s full
`Repository` just to satisfy this narrow interface -- this suite's
synthetic cases have no real `caselifecycle.Case` to read a live
`MetadataVersion` from.

## Data-isolation (task 7)

`isolation_scenario.go` seeds two independent synthetic cases, then
wraps case A's *same underlying store* in a `CaseScopedStore` scoped to
case B, and proves `GetNode`/`CreateEdge` both reject cross-case access
with the real, typed `knowledgeisolation.ErrCrossCaseAccess` -- not a
filtered empty result. Every rejected attempt is confirmed recorded via
`AccessAttempts()`. A positive control (case B reading its own,
independently-seeded fact) proves the guard rejects cross-case access
*specifically*, not all access indiscriminately.

The cross-**tenant** axis (`packages/tenancy.WithTenantScope`'s
Postgres-Row-Level-Security guarantee) is deliberately **not**
re-exercised here: that guarantee requires a live Postgres connection
with RLS enabled, exactly the Docker/testcontainers dependency this
phase's brief says to skip, and the same reasoning
`packages/securitytesting`'s own `dataleakage_suite.go` documents at
length for its cross-tenant scenario. `packages/tenancy/integration_test.go`
is the real, Postgres-verified test for that specific axis.

## `Suite`, not `Harness` (design note)

This package defines its own `Suite` (`suite.go`) rather than importing
`packages/securitytesting.Harness`. Both share the same
`Name`/`Category`/`Run` vocabulary deliberately, but
`securitytesting.Harness.RunAll`/`RunOne` are shaped around
`securitytesting.Scenario` (an adversarial `Result` with tenant/
permission bookkeeping and `Engine`-level persistence). This package's
`Scenario` returns a full-journey `ScenarioResult` with no tenant
gating and no persistence -- adapting one shape into the other at
every call site would add an awkward translation layer for no real
benefit. The two types are intentionally structural cousins, not one
wrapping the other.

## Flaky-test controls (task 9)

`flaky.go` defines two honest mechanisms:

- `RetryOnFlake(t, maxAttempts, delay, fn)` retries a **test-only**
  function against genuinely non-deterministic timing (e.g. a
  background-goroutine TTL wait racing a loaded CI runner's scheduler),
  logging every failed attempt via `t.Logf` rather than silently
  hiding it. It is confined to test code (`testing.TB`), never used
  inside a `Scenario.Run` itself -- masking a flaky underlying defect
  behind a retry inside `Run` would defeat the suite's whole purpose.
- `QuarantineList` is an explicit, reviewable list of scenario names to
  skip, each entry requiring a non-blank `Reason`.
  `ActiveQuarantineList` starts (and, as of this phase, remains) empty:
  every scenario this package ships is deterministic as written.
  `RunAllSkippingQuarantined` still returns one `SuiteRecord` per
  scenario, including quarantined ones (as `OutcomeErrored`, never
  silently dropped).

## CI integration (task 8)

`.github/workflows/ci.yml` gains a new `e2e-tests` job running
`go test ./packages/e2e/... -race -count=1 -timeout=5m -v` on the same
Go toolchain setup `build-go` uses (no Docker/Postgres required). It is
wired into the `gate` job's blocking dependencies alongside `build-go`,
`build-ts`, `secrets-scan`, `container-scan`, `branch-policy`, and
`sign-artifacts`.

## What this suite does NOT cover

- **No real LLM provider call, anywhere.** See `sequencedProvider`
  (`fixture.go`) -- a deterministic, provider-agnostic
  `provider.LLMProvider` fake, consistent with Phase 011's
  `provider.NoOpProvider` convention.
- **No Docker, no testcontainers, no live Postgres.** Every store this
  suite touches is in-memory. `packages/tenancy`'s
  Postgres-Row-Level-Security cross-tenant guarantee specifically is
  not re-verified here (see "Data-isolation" above).
- **No real STT/OCR transcription.** `stt.NoOpSTTProvider` ignores
  audio content entirely; the multilingual scenario drives real text
  directly through `multilingual.NormalizationService` instead of
  through the ingestion pipeline's transcription step for this reason.
- **No load, chaos, or long-running soak testing.** See
  `packages/perf` (Phase 091) for benchmarking and
  `packages/reliability` for chaos testing.
- **No durable persistence of run history.** Mirroring
  `packages/perf`'s own Phase 091 in-memory-only precedent: this
  package has no tenant-scoped repository, no `Engine`-style
  permission-gated façade, and no migration. It is a test suite
  invoked by CI and engineers, not a tenant-facing service with
  durable secrets to protect.

## Storage

None. Consistent with `packages/perf`'s Phase 091 precedent (see its
`doc.go`'s "no `packages/identity.Permission` constants" note, and its
"no durable `BenchmarkRun`" section): this package has no per-tenant
durable state, no tenant-scoped repository, and no `Engine`-style
façade gating reads/writes on an authenticated actor. Every store this
suite exercises (`graph.InMemoryGraphStore`,
`vectorindex.NewInMemoryVectorStore`,
`reasoningorchestration.InMemoryCheckpointStore`,
`signoff.InMemoryRepository`, `intake`'s in-process `TempBuffer`) is
in-memory and scoped to a single scenario run. No migration was added
to `packages/persistence/migrations/`.

## Running the suite

```sh
cd packages/e2e
go build ./...
go vet ./...
go test ./... -race -count=1 -timeout=5m -v
golangci-lint run ./...
```
