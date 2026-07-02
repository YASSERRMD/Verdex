# Verdex Ingestion Orchestration

## Overview

`packages/ingestion` coordinates the full ingestion pipeline built across
phases 019–026 into a single resilient async workflow:

```
intake -> extraction (STT or OCR) -> normalize -> segment -> classify
```

Each stage is implemented by a sibling package this phase depends on but
never reimplements:

| Stage        | Sibling package         | Entry point                                  |
|--------------|--------------------------|-----------------------------------------------|
| Intake       | `packages/intake`        | `IntakeService.Ingest`                        |
| Extraction   | `packages/stt` (audio)   | `STTService.Transcribe`                       |
| Extraction   | `packages/ocr` (image)   | `OCRService.Extract`                          |
| Normalize    | `packages/multilingual`  | `NormalizationService.Normalize`              |
| Segment      | `packages/segmentation`  | `SegmentationService.Segment`                 |
| Classify     | `packages/evidence`      | `EvidenceService.ClassifySegments`             |

`packages/ingestion`'s job is orchestration: sequencing these calls,
retrying transiently-failed stages, verifying the transcribe-and-discard
guarantee, tracking progress, checkpointing recovery state, and routing
permanently-failed jobs to a dead-letter queue — not duplicating any
sibling's internal logic.

---

## Stage / WorkflowState

```go
type Stage string

const (
    StageIntake     Stage = "intake"
    StageExtraction Stage = "extraction"
    StageNormalize  Stage = "normalize"
    StageSegment    Stage = "segment"
    StageClassify   Stage = "classify"
    StageComplete   Stage = "complete"
    StageFailed     Stage = "failed"
)

type WorkflowState struct {
    JobID            string
    CaseID           string
    Stage            Stage
    AttemptsForStage int
    FailureReason    string
    UpdatedAt        time.Time
    Segments         []ClassifiedSegment
}
```

`Stage` follows a fixed linear order (`stageOrder` in `workflow.go`);
`NextStage` walks that order, and `StageFailed` is reachable from any
non-terminal stage once retries are exhausted. `WorkflowState.Advance`
returns a new state moved to the next stage with `AttemptsForStage` reset
and `FailureReason` cleared (unless the target is `StageFailed`).

`Job` (`types.go`) is the unit of work: it carries a `JobID`, `CaseID`,
`InputKind` (`InputAudio` or `InputImage`), the raw `stt.AudioInput` or
`ocr.ImageInput` payload, and an optional `ProviderID` (defaults to
`"noop"`).

---

## Job Queue

```go
type JobQueue interface {
    Enqueue(job Job) error
    Dequeue(ctx context.Context) (*Job, error)
    Close()
}
```

`InMemoryJobQueue` is a buffered-channel implementation with no external
broker. `Close` stops accepting new work but still drains any buffered
jobs to `Dequeue` before returning `ErrQueueClosed`; it never closes the
underlying channel itself, avoiding a send-on-closed-channel panic if
`Enqueue` races with `Close`.

`IngestionOrchestrator.Run(ctx)` is a blocking worker loop that dequeues
and processes jobs one at a time until `ctx` is cancelled or the queue is
closed and drained — intended to be launched in its own goroutine.

---

## Retry and Idempotency

```go
type IdempotencyStore interface {
    Get(key string, stage Stage) (rec IdempotencyRecord, ok bool)
    MarkAttempt(key string, stage Stage) int
    MarkCompleted(key string, stage Stage)
    Reset(key string, stage Stage)
}

func RunWithRetry(ctx context.Context, store IdempotencyStore, policy RetryPolicy,
    job Job, stage Stage, state WorkflowState, fn StageFunc) (WorkflowState, error)
```

`RunWithRetry` is the bounded-retry wrapper every stage runs through:

1. If `(job.JobID, stage)` is already marked `Completed`, `fn` is not
   invoked at all — a true no-op, not just a fast success.
2. Otherwise `fn` is invoked up to `policy.MaxAttempts` times (default
   `DefaultMaxAttempts = 3`). The first successful attempt marks the
   record `Completed` and returns immediately.
3. If every attempt fails, the last error is wrapped in
   `ErrRetriesExhausted` and returned.

`InMemoryIdempotencyStore` is the default map-backed implementation.
`Reset` clears a stage's record so `Resume` can force a fresh attempt
window for the stage that previously failed.

---

## Partial-Failure Recovery

```go
type RecoveryStore interface {
    Checkpoint(state WorkflowState)
    Load(jobID string) (state WorkflowState, ok bool)
    Delete(jobID string)
}

func PlanResume(recovery RecoveryStore, jobID string) (ResumePlan, error)
```

After every stage transition (success or failure), `IngestionOrchestrator`
checkpoints the current `WorkflowState` to `RecoveryStore`. On failure, the
checkpoint records the *actual stage that exhausted retries* (e.g.
`StageExtraction`), separate from the terminal `StageFailed` reported via
`Status`/the returned `WorkflowState` — that distinction is what lets
`PlanResume`/`Resume` know exactly which stage to re-attempt rather than
restarting from `StageIntake`.

`IngestionOrchestrator.Resume(ctx, job)`:

1. Loads the job's checkpoint via `PlanResume` (returns `ErrNotResumable`
   if there is none, or the job already reached `StageComplete`).
2. Resets the failed stage's idempotency record so it gets a fresh
   attempt window.
3. Re-enters `process()`, which picks up from the checkpointed stage.

Resume must be called against the same orchestrator instance that
originally processed the job (or one sharing its idempotency/scratch
state): stage completion is durable via `IdempotencyStore`, but this
package's in-memory intermediate artifacts (extracted text, normalized
text, segments) do not survive a full process restart.

---

## Progress Reporting

```go
type ProgressReporter interface {
    Report(p Progress)
    Get(jobID string) (p Progress, ok bool)
    Subscribe(jobID string) (ch <-chan Progress, unsubscribe func())
}
```

Each stage maps to a fixed percent-complete via `PercentCompleteForStage`
(Intake 20%, Extraction 40%, Normalize 60%, Segment 80%, Classify 95%,
Complete 100%, Failed 0%). `InMemoryProgressReporter.Subscribe` returns a
buffered channel of live updates for one job; `Report` never blocks on a
slow or absent subscriber — it drops the oldest queued update to make room
for the newest rather than stalling the pipeline.

---

## Discard Verification

```go
func VerifyAudioDiscard(jobID string, input stt.AudioInput, sourceHash string) (DiscardVerification, error)
func VerifyImageDiscard(jobID string, input ocr.ImageInput, sourceHash string) (DiscardVerification, error)
```

After `StageExtraction` calls into `packages/stt`/`packages/ocr`, the
orchestrator does not simply trust that the transcribe-and-discard
guarantee held — it independently checks:

1. `sourceHash` (captured by the sibling service before discard) is
   non-empty.
2. The `AudioInput`/`ImageInput`'s `Data` was actually zeroed
   (`stt.IsDiscarded` / `ocr.IsDiscarded`).

If either check fails, the stage fails with `ErrDiscardVerificationFailed`
— mirroring CONTRIBUTING.md's "Transcribe-and-Discard" guardrail and the
same test pattern `packages/stt`/`packages/ocr` use for their own discard
guarantees.

---

## Per-Case Status API

```go
type IngestionStatusAPI struct { /* ... */ }

func (a *IngestionStatusAPI) GetStatus(jobID string) (*WorkflowState, error)
func (a *IngestionStatusAPI) GetCaseStatus(caseID string) []WorkflowState
```

A small in-process query surface over `StatusStore`, intentionally
decoupled from `RecoveryStore` (which is specifically for resuming failed
jobs): `StatusStore` is the "what does this job/case look like right now"
view an HTTP handler could wrap directly.

---

## Dead-Letter Handling

```go
type DeadLetterQueue interface {
    Add(dl DeadLetter)
    Get(jobID string) (dl DeadLetter, ok bool)
    List() []DeadLetter
    Remove(jobID string)
}
```

Once `RunWithRetry` returns `ErrRetriesExhausted` (or a stage otherwise
fails outright, e.g. an invalid `Job`), the orchestrator records a
`DeadLetter` with the failing `Stage`, `Reason`, and `Attempts`, then moves
`WorkflowState.Stage` to `StageFailed`. Dead-lettered jobs remain
inspectable via `List`/`Get` independent of the live pipeline, and their
recovery checkpoint is retained so a later `Resume` can be attempted once
the underlying issue is fixed.

---

## Orchestrator

`IngestionOrchestrator` (`orchestrator.go`) wires everything together:

```go
type IngestionOrchestrator struct {
    Queue       JobQueue
    Idempotency IdempotencyStore
    Recovery    RecoveryStore
    Progress    ProgressReporter
    Status      StatusStore
    DeadLetters DeadLetterQueue
    RetryPolicy RetryPolicy

    Intake     *intake.IntakeService
    STT        *stt.STTService
    OCR        *ocr.OCRService
    Normalizer *multilingual.NormalizationService
    Segmenter  *segmentation.SegmentationService
    Evidence   *evidence.EvidenceService
}
```

`NewIngestionOrchestrator(cfg OrchestratorConfig)` fills in in-memory
defaults for every store left nil; every sibling service left nil is
lazily constructed with its own package defaults (e.g. `stt.NewSTTService`
against `stt.DefaultRegistry`) the first time it's needed.

`Process(ctx, job)` runs a job through every stage in order:

1. **Intake** — runs `packages/intake`'s real validate → hash → scan →
   discard pipeline over a synthetic artifact representing the job's
   uploaded binary, and requires a provenance hash to be returned.
2. **Extraction** — routes to `packages/stt.Transcribe` (audio) or
   `packages/ocr.Extract` (image), then runs `VerifyAudioDiscard`/
   `VerifyImageDiscard` against the result.
3. **Normalize** — runs the extracted text through
   `packages/multilingual.Normalize`.
4. **Segment** — runs the normalized text through
   `packages/segmentation.Segment`.
5. **Classify** — runs the segments through
   `packages/evidence.ClassifySegments`, producing the final
   `[]ClassifiedSegment`.

Every stage is executed via `RunWithRetry`; every stage transition
reports `Progress`, updates `Status`, and checkpoints `Recovery`. On
success, `WorkflowState.Stage == StageComplete` and `.Segments` holds the
classified output. On failure, `.Stage == StageFailed`, `.FailureReason`
explains why, and the job is in `DeadLetters`.

---

## Design Principles

- **Coordinate, don't reimplement.** Every stage delegates to the actual
  sibling service/interface (`stt.STTService`, `ocr.OCRService`,
  `multilingual.NormalizationService`, `segmentation.SegmentationService`,
  `evidence.EvidenceService`, `intake.IntakeService`) rather than
  duplicating pipeline logic in this package.
- **Resilience by default.** Bounded retries with idempotency, recovery
  checkpointing, and dead-letter routing are wired into every stage, not
  bolted on as an afterthought.
- **Transcribe-and-discard is verified, not assumed.** The orchestrator
  independently re-checks the discard guarantee after extraction rather
  than trusting the sibling service's own internal enforcement.
- **In-memory by default, swappable everywhere.** Every store is an
  interface with an in-memory implementation so a real deployment can
  swap in durable implementations without touching
  `IngestionOrchestrator`'s coordination logic.
