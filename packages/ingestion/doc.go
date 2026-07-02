// Package ingestion coordinates the full document/media ingestion
// pipeline — intake -> STT/OCR extraction -> normalize -> segment ->
// classify — as a resilient async workflow, orchestrating the sibling
// packages built in earlier phases (packages/intake, packages/stt,
// packages/ocr, packages/multilingual, packages/segmentation,
// packages/evidence) rather than reimplementing any of their logic.
//
// Core concepts:
//
//   - Stage / WorkflowState: the fixed pipeline ordering (Intake ->
//     Extraction -> Normalize -> Segment -> Classify -> Complete, with
//     Failed as the terminal error state) and the per-job state tracked
//     as a job moves through it (workflow.go).
//   - JobQueue / InMemoryJobQueue: an asynchronous, channel-backed handoff
//     point between whatever submits ingestion work and the
//     orchestrator's worker loop, with no external broker (queue.go).
//   - IdempotencyStore / RunWithRetry: bounded retries per stage, keyed by
//     (JobID, Stage), so re-running an already-completed stage is a no-op
//     rather than duplicate work (retry.go).
//   - RecoveryStore / PlanResume: checkpoints the last-attempted stage so
//     a mid-pipeline failure can be resumed from that stage rather than
//     restarting the whole pipeline (recovery.go).
//   - ProgressReporter: tracks percent-complete/current stage per job and
//     supports live Subscribe-based updates (progress.go).
//   - VerifyAudioDiscard / VerifyImageDiscard: asserts the
//     transcribe-and-discard guarantee after the extraction stage —
//     packages/stt and packages/ocr must have actually zeroed the source
//     bytes and recorded a provenance hash — failing the stage otherwise
//     (discard_verify.go).
//   - IngestionStatusAPI: a small in-process query surface
//     (GetStatus/GetCaseStatus) over a job's or a case's current
//     WorkflowState, structured so an HTTP handler could wrap it later
//     (status.go).
//   - DeadLetterQueue: holds jobs that exhausted retries at some stage,
//     with their failure reason and attempt count, so they can be listed
//     and inspected independent of the live pipeline (deadletter.go).
//   - IngestionOrchestrator: the top-level coordinator wiring a JobQueue
//     to stage execution (via RunWithRetry), discard verification,
//     progress reporting, recovery checkpointing, and dead-letter handling
//     on exhaustion, producing the final classified segments
//     (orchestrator.go).
//
// Design principles:
//
//   - Coordinate, don't reimplement. This package depends on each sibling
//     package's exported service/interface (stt.STTService,
//     ocr.OCRService, multilingual.NormalizationService,
//     segmentation.SegmentationService, evidence.EvidenceService,
//     intake.IntakeService) rather than duplicating their pipelines.
//   - Resilience by default. Every stage runs through bounded retries with
//     idempotency; a failure at any stage checkpoints recovery state and,
//     once retries are exhausted, lands the job in the dead-letter queue
//     rather than silently dropping it.
//   - Transcribe-and-discard is enforced, not assumed. The orchestrator
//     does not trust that packages/stt/packages/ocr discarded source
//     bytes — it independently verifies the post-extraction AudioInput/
//     ImageInput is zeroed and a provenance hash was recorded, mirroring
//     CONTRIBUTING.md's transcribe-and-discard guardrail.
//   - In-memory by default, swappable everywhere. Every store
//     (IdempotencyStore, RecoveryStore, ProgressReporter, StatusStore,
//     DeadLetterQueue, JobQueue) is an interface with an in-memory
//     implementation; a real deployment can swap in durable
//     implementations without changing IngestionOrchestrator's logic.
//
// See doc/ingestion-workflow.md for a detailed workflow write-up.
package ingestion
