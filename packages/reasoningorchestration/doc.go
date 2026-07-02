// Package reasoningorchestration coordinates the entire Part 5 reasoning
// stack — packages/issueagent, packages/firstpartyagent,
// packages/secondpartyagent, packages/evidenceweighing,
// packages/lawapplication, packages/synthesisagent,
// packages/uncertainty, and packages/guardrail — into one end-to-end
// pipeline for a single case, per the implementation plan's Phase 059
// "Reasoning orchestration pipeline" goal: "Coordinate the full agent
// sequence end to end."
//
// # Composes with, does not duplicate
//
// Every unit of actual reasoning work — issue framing, argument
// construction, evidence weighing, law application, synthesis,
// uncertainty surfacing, and the non-binding guardrail check — already
// exists as an independent, fully tested package (Phases 050-057). This
// package does not reimplement, wrap with new logic, or extend any of
// them: Run calls each package's own sanctioned entrypoint
// (issueagent.Analyze, firstpartyagent.Argue, secondpartyagent.Argue,
// evidenceweighing.Weigh, lawapplication.Apply,
// synthesisagent.Synthesize, uncertainty.Surface, guardrail.CheckText/
// CanFinalize) in the dependency order those packages' own designs
// require, and nothing more. packages/reasoningprofile (Phase 058) is
// consulted only for its Weights/Family resolution, run concurrently
// with issue framing (see "Concurrency" below) — this package does not
// duplicate reasoningprofile's own weighting logic either.
//
// # Primary types
//
// Stage is the eight-value (plus a terminal StageComplete marker) enum
// naming each pipeline step. RunState is the persistable state of one
// case's run: its CurrentStage, CompletedStages, and (if halted short of
// completion) which stage failed or that the budget was exhausted.
// Run(ctx, caseID, cfg) RunResult drives a case through every stage from
// scratch; Resume(ctx, caseID, cfg) RunResult continues a previously
// checkpointed, partially-completed run, skipping every stage already
// present in CompletedStages.
//
// # Why (mostly) sequential
//
// Every stage but one strictly requires the prior stage's typed output:
// argument construction needs framed issues, second-party rebuttal needs
// the first party's ArgumentSet, evidence weighing needs both
// ArgumentSets, law application needs the issues/ArgumentSets/evidence
// weights together, synthesis needs every one of those, and uncertainty
// surfacing needs the synthesized Opinion. This is not an
// implementation shortcut this package could parallelize away — it is
// each upstream package's own documented design (see e.g.
// packages/secondpartyagent's doc.go: rebuttal requires the first
// party's arguments to exist first). See doc/reasoning-orchestration.md
// for the full stage-by-stage dependency table.
//
// # Concurrency
//
// Exactly one genuine concurrency opportunity exists in this chain:
// resolving this case's reasoningprofile.Family/Weights depends only on
// the case's jurisdiction/legal-family context, not on issue framing's
// output, and issue framing does not depend on it either. runIssueFraming
// therefore starts that resolution on its own goroutine before calling
// issueagent.Analyze and joins it before returning. This is safe because
// the two computations share no mutable state and have no ordering
// requirement between them. Separately, checkpoint persistence
// (SaveCheckpoint) is fire-and-forget on its own goroutine per stage,
// since the next stage always reads from the in-process pipelineContext,
// never from the CheckpointStore — see doc/reasoning-orchestration.md.
//
// # What this package deliberately does not do
//
// It does not call any provider.LLMProvider directly (every LLM call is
// reached through the same *router.Router each Part-5 package already
// requires); it does not implement its own retry/backoff policy beyond
// what agentframework.Runner already provides per stage; it does not
// implement a durable CheckpointStore backend (InMemoryCheckpointStore is
// the only implementation here, mirroring every sibling package's
// in-memory-first convention); and it does not implement human sign-off
// itself — StageGuardrailCheck calls guardrail.CanFinalize against
// whatever guardrail.SignoffGate the caller supplies (defaulting to the
// fail-closed guardrail.NoSignoffRecordedGate), the same extension point
// Phase 068 is expected to implement.
package reasoningorchestration
