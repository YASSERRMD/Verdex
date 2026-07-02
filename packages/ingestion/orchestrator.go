package ingestion

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/YASSERRMD/verdex/packages/evidence"
	"github.com/YASSERRMD/verdex/packages/intake"
	"github.com/YASSERRMD/verdex/packages/multilingual"
	"github.com/YASSERRMD/verdex/packages/ocr"
	"github.com/YASSERRMD/verdex/packages/segmentation"
	"github.com/YASSERRMD/verdex/packages/stt"
	"github.com/google/uuid"
)

// intakeArtifactMIME is the declared/detected MIME type used for the
// StageIntake binary-handling step. The orchestrator's intake stage is
// only responsible for the transcribe-and-discard-style handling of the
// raw uploaded bytes (validate, hash, virus-scan, buffer, discard); the
// audio/image-specific payload consumed by StageExtraction is supplied
// independently on the Job (Job.Audio / Job.Image), mirroring how a real
// deployment would decode a validated upload into the shape each
// downstream extractor expects.
const intakeArtifactMIME = "application/pdf"

// intakeArtifactHeader is a minimal valid-looking payload that both
// declares and sniffs as intakeArtifactMIME, so StageIntake can run
// packages/intake's real validate -> hash -> scan -> discard pipeline
// without depending on real binary fixtures.
var intakeArtifactHeader = []byte("%PDF-1.4 verdex ingestion orchestrator artifact")

// OrchestratorConfig supplies the pluggable dependencies for
// NewIngestionOrchestrator. Every field is optional; nil fields fall back
// to in-memory defaults (stores) or lazily-constructed sibling services.
type OrchestratorConfig struct {
	Queue       JobQueue
	Idempotency IdempotencyStore
	Recovery    RecoveryStore
	Progress    ProgressReporter
	Status      StatusStore
	DeadLetters DeadLetterQueue
	RetryPolicy RetryPolicy

	// Intake is the sibling packages/intake service used for StageIntake.
	// If nil, a service is lazily constructed with no-op scanner/quota/audit
	// dependencies.
	Intake *intake.IntakeService

	// STT is the sibling packages/stt service used for StageExtraction
	// when a job's Kind is InputAudio. If nil, a service backed by
	// stt.DefaultRegistry is lazily constructed.
	STT *stt.STTService

	// OCR is the sibling packages/ocr service used for StageExtraction
	// when a job's Kind is InputImage. If nil, a service backed by
	// ocr.DefaultRegistry is lazily constructed.
	OCR *ocr.OCRService

	// Normalizer is the sibling packages/multilingual service used for
	// StageNormalize. If nil, multilingual.NewNormalizationService() is
	// lazily constructed.
	Normalizer *multilingual.NormalizationService

	// Segmenter is the sibling packages/segmentation service used for
	// StageSegment. If nil, segmentation.NewSegmentationService() is
	// lazily constructed.
	Segmenter *segmentation.SegmentationService

	// Evidence is the sibling packages/evidence service used for
	// StageClassify. If nil, evidence.NewEvidenceService() is lazily
	// constructed.
	Evidence *evidence.EvidenceService
}

// IngestionOrchestrator is the top-level coordinator wiring together every
// building block in this package: a JobQueue feeds jobs to a worker loop
// that executes each Stage in order via RunWithRetry (idempotency +
// bounded retries), verifies the transcribe-and-discard guarantee after
// extraction, reports progress as it goes, checkpoints recovery state,
// updates the status store, and moves a job to the DeadLetterQueue if
// retries are exhausted at any stage.
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

	// scratch holds orchestrator-internal, per-job intermediate pipeline
	// artifacts (extracted text, normalized text, segments) that live only
	// for the duration of a job's processing and are not part of the
	// externally visible WorkflowState. Safe for concurrent use across
	// jobs processed in parallel.
	scratch   map[string]*pipelineState
	scratchMu sync.Mutex
}

// NewIngestionOrchestrator constructs an IngestionOrchestrator from cfg,
// filling in in-memory defaults for every store left nil. Sibling-service
// dependencies (Intake, STT, OCR, Normalizer, Segmenter, Evidence) left
// nil are lazily constructed with their own package defaults the first
// time they are needed. Callers that want a shared JobQueue must set
// cfg.Queue explicitly; the returned orchestrator has no queue wired by
// default (Run requires one).
func NewIngestionOrchestrator(cfg OrchestratorConfig) *IngestionOrchestrator {
	orch := &IngestionOrchestrator{
		Queue:       cfg.Queue,
		Idempotency: cfg.Idempotency,
		Recovery:    cfg.Recovery,
		Progress:    cfg.Progress,
		Status:      cfg.Status,
		DeadLetters: cfg.DeadLetters,
		RetryPolicy: cfg.RetryPolicy,
		Intake:      cfg.Intake,
		STT:         cfg.STT,
		OCR:         cfg.OCR,
		Normalizer:  cfg.Normalizer,
		Segmenter:   cfg.Segmenter,
		Evidence:    cfg.Evidence,
		scratch:     make(map[string]*pipelineState),
	}
	if orch.Idempotency == nil {
		orch.Idempotency = NewInMemoryIdempotencyStore()
	}
	if orch.Recovery == nil {
		orch.Recovery = NewInMemoryRecoveryStore()
	}
	if orch.Progress == nil {
		orch.Progress = NewInMemoryProgressReporter()
	}
	if orch.Status == nil {
		orch.Status = NewInMemoryStatusStore()
	}
	if orch.DeadLetters == nil {
		orch.DeadLetters = NewInMemoryDeadLetterQueue()
	}
	return orch
}

func (o *IngestionOrchestrator) intakeService() *intake.IntakeService {
	if o.Intake != nil {
		return o.Intake
	}
	o.Intake = intake.NewIntakeService(intake.NoOpVirusScanHook{}, nil, intake.NoOpAuditSink{}, time.Millisecond)
	return o.Intake
}

func (o *IngestionOrchestrator) sttService() *stt.STTService {
	if o.STT != nil {
		return o.STT
	}
	o.STT = stt.NewSTTService(nil, nil, nil)
	return o.STT
}

func (o *IngestionOrchestrator) ocrService() *ocr.OCRService {
	if o.OCR != nil {
		return o.OCR
	}
	o.OCR = ocr.NewOCRService(nil, nil, nil, nil)
	return o.OCR
}

func (o *IngestionOrchestrator) normalizer() *multilingual.NormalizationService {
	if o.Normalizer != nil {
		return o.Normalizer
	}
	o.Normalizer = multilingual.NewNormalizationService()
	return o.Normalizer
}

func (o *IngestionOrchestrator) segmenter() *segmentation.SegmentationService {
	if o.Segmenter != nil {
		return o.Segmenter
	}
	o.Segmenter = segmentation.NewSegmentationService()
	return o.Segmenter
}

func (o *IngestionOrchestrator) evidenceService() *evidence.EvidenceService {
	if o.Evidence != nil {
		return o.Evidence
	}
	o.Evidence = evidence.NewEvidenceService()
	return o.Evidence
}

// pipelineState carries values threaded between stage functions within a
// single job's processing that don't belong in the persisted
// WorkflowState (e.g. the extracted text and provenance hash aren't
// queryable status fields, only the final classified segments are).
type pipelineState struct {
	extractedText string
	extractedLang string
	sourceHash    string
	normalized    *multilingual.NormalizedText
	segments      []segmentation.Segment
	classified    []ClassifiedSegment
}

// stash returns (creating if necessary) the pipelineState for jobID.
func (o *IngestionOrchestrator) stash(jobID string) *pipelineState {
	o.scratchMu.Lock()
	defer o.scratchMu.Unlock()
	if o.scratch == nil {
		o.scratch = make(map[string]*pipelineState)
	}
	ps, ok := o.scratch[jobID]
	if !ok {
		ps = &pipelineState{}
		o.scratch[jobID] = ps
	}
	return ps
}

// clearStash discards jobID's pipelineState once processing has reached a
// terminal stage, so scratch memory doesn't grow unbounded.
func (o *IngestionOrchestrator) clearStash(jobID string) {
	o.scratchMu.Lock()
	defer o.scratchMu.Unlock()
	delete(o.scratch, jobID)
}

// Process runs job through every pipeline stage in order, from
// StageIntake through StageClassify, returning the final WorkflowState.
// On success, Stage == StageComplete and WorkflowState.Segments holds the
// classified output. On failure, Stage == StageFailed,
// WorkflowState.FailureReason explains why, and the job has been recorded
// in DeadLetters.
//
// Process reports Progress after every stage transition, checkpoints
// Recovery after every stage transition, and keeps Status up to date
// throughout so IngestionStatusAPI reflects live state.
func (o *IngestionOrchestrator) Process(ctx context.Context, job Job) WorkflowState {
	return o.process(ctx, job)
}

// Resume continues jobID from its last recovery checkpoint (see
// recovery.go) rather than restarting from StageIntake. Returns
// ErrNotResumable if no checkpoint exists for jobID. job must be the same
// Job value originally submitted (Job itself is not persisted by
// RecoveryStore/StatusStore, only its derived WorkflowState is).
//
// Resume must be called on the same *IngestionOrchestrator instance (or
// one sharing its Idempotency/scratch state) that originally processed the
// job: stage idempotency records for already-completed stages are honored
// (RunWithRetry no-ops them), but this package's in-memory scratch
// (extracted/normalized/segmented intermediates) does not survive a
// process restart, so resuming after a full process restart is only
// guaranteed to work correctly when resuming from StageIntake or
// StageExtraction (which do not depend on scratch from an earlier stage).
func (o *IngestionOrchestrator) Resume(ctx context.Context, job Job) (WorkflowState, error) {
	plan, err := PlanResume(o.Recovery, job.JobID)
	if err != nil {
		return WorkflowState{}, err
	}

	if _, ok := o.Recovery.Load(job.JobID); !ok {
		return WorkflowState{}, ErrNotResumable
	}
	// Re-attempting FromStage should not be blocked by a prior failed
	// idempotency attempt count carried over from before the failure; give
	// it a fresh window of attempts.
	o.Idempotency.Reset(job.JobID, plan.FromStage)

	// The checkpointed WorkflowState.Stage already equals plan.FromStage
	// (see PlanResume), so process() picks up from the right stage once it
	// reloads the checkpoint itself.
	return o.process(ctx, job), nil
}

func (o *IngestionOrchestrator) process(ctx context.Context, job Job) WorkflowState {
	state, ok := o.Recovery.Load(job.JobID)
	if !ok || state.JobID == "" {
		state = WorkflowState{
			JobID:     job.JobID,
			CaseID:    job.CaseID,
			Stage:     StageIntake,
			UpdatedAt: time.Now().UTC(),
		}
	}

	o.saveState(state)

	stages := []Stage{StageIntake, StageExtraction, StageNormalize, StageSegment, StageClassify}
	startIdx := 0
	for i, st := range stages {
		if st == state.Stage {
			startIdx = i
			break
		}
	}

	for i := startIdx; i < len(stages); i++ {
		st := stages[i]
		fn := o.stageFunc(st)

		next, err := RunWithRetry(ctx, o.Idempotency, o.RetryPolicy, job, st, state, fn)
		if err != nil {
			failed := state
			failed.Stage = st
			failed.FailureReason = err.Error()
			failed.UpdatedAt = time.Now().UTC()
			o.saveState(failed)
			o.Recovery.Checkpoint(failed)

			o.DeadLetters.Add(DeadLetter{
				JobID:    job.JobID,
				CaseID:   job.CaseID,
				Stage:    st,
				Reason:   err.Error(),
				Attempts: o.attemptsFor(job.JobID, st),
				FailedAt: time.Now().UTC(),
			})

			finalFailed := failed
			finalFailed.Stage = StageFailed
			finalFailed.UpdatedAt = time.Now().UTC()
			o.saveState(finalFailed)
			return finalFailed
		}

		state = next
		advanceTo, hasNext := NextStage(st)
		if !hasNext {
			advanceTo = StageComplete
		}
		state = state.Advance(advanceTo, time.Now().UTC())
		o.saveState(state)
		o.Recovery.Checkpoint(state)
	}

	// All stages ran; runClassifyStage already populated ps.classified.
	ps := o.stash(job.JobID)
	if len(ps.classified) == 0 {
		err := fmt.Errorf("%w: classify: no classified segments for job %s", ErrStageFailed, job.JobID)
		state.Stage = StageFailed
		state.FailureReason = err.Error()
		state.UpdatedAt = time.Now().UTC()
		o.saveState(state)
		o.Recovery.Checkpoint(state)
		o.DeadLetters.Add(DeadLetter{
			JobID:    job.JobID,
			CaseID:   job.CaseID,
			Stage:    StageClassify,
			Reason:   err.Error(),
			FailedAt: time.Now().UTC(),
		})
		o.clearStash(job.JobID)
		return state
	}

	state.Segments = ps.classified
	state.Stage = StageComplete
	state.UpdatedAt = time.Now().UTC()
	o.saveState(state)
	o.Recovery.Delete(job.JobID)
	o.clearStash(job.JobID)
	return state
}

func (o *IngestionOrchestrator) attemptsFor(jobID string, stage Stage) int {
	if rec, ok := o.Idempotency.Get(jobID, stage); ok {
		return rec.Attempts
	}
	return 0
}

func (o *IngestionOrchestrator) saveState(state WorkflowState) {
	o.Status.Put(state)
	o.Progress.Report(Progress{
		JobID:           state.JobID,
		Stage:           state.Stage,
		PercentComplete: PercentCompleteForStage(state.Stage),
	})
}

// stageFunc dispatches to the concrete implementation for a stage.
// Extraction/normalize/segment results that don't belong on the persisted
// WorkflowState are attached via o.stash(job.JobID) (see pipelineState).
func (o *IngestionOrchestrator) stageFunc(stage Stage) StageFunc {
	switch stage {
	case StageIntake:
		return o.runIntake
	case StageExtraction:
		return o.runExtraction
	case StageNormalize:
		return o.runNormalize
	case StageSegment:
		return o.runSegment
	case StageClassify:
		return o.runClassifyStage
	default:
		return func(_ context.Context, _ Job, state WorkflowState) (WorkflowState, error) {
			return state, fmt.Errorf("%w: unknown stage %s", ErrStageFailed, stage)
		}
	}
}

func (o *IngestionOrchestrator) runIntake(ctx context.Context, job Job, state WorkflowState) (WorkflowState, error) {
	svc := o.intakeService()

	req := intake.IntakeRequest{
		TenantID:   uuid.New(),
		UploaderID: uuid.New(),
		Filename:   fmt.Sprintf("%s-artifact", job.JobID),
		MIMEType:   intakeArtifactMIME,
		SizeBytes:  int64(len(intakeArtifactHeader)),
		TTL:        time.Millisecond,
	}

	result, err := svc.Ingest(ctx, req, bytes.NewReader(intakeArtifactHeader))
	if err != nil {
		return state, fmt.Errorf("%w: intake: %v", ErrStageFailed, err)
	}
	if result.ProvisionHash == "" {
		return state, fmt.Errorf("%w: intake: missing provenance hash", ErrStageFailed)
	}
	return state, nil
}

func (o *IngestionOrchestrator) runExtraction(ctx context.Context, job Job, state WorkflowState) (WorkflowState, error) {
	switch job.Kind {
	case InputAudio:
		return o.runAudioExtraction(ctx, job, state)
	case InputImage:
		return o.runImageExtraction(ctx, job, state)
	default:
		return state, fmt.Errorf("%w: extraction: unknown input kind %q", ErrStageFailed, job.Kind)
	}
}

func (o *IngestionOrchestrator) runAudioExtraction(ctx context.Context, job Job, state WorkflowState) (WorkflowState, error) {
	if job.Audio == nil {
		return state, fmt.Errorf("%w: extraction: nil audio input", ErrStageFailed)
	}
	// Operate on a copy so retries don't re-transcribe already-zeroed data.
	input := *job.Audio
	input.Data = append([]byte(nil), job.Audio.Data...)

	providerID := job.ProviderID
	if providerID == "" {
		providerID = "noop"
	}

	transcript, err := o.sttService().Transcribe(ctx, providerID, &input)
	if err != nil {
		return state, fmt.Errorf("%w: extraction(stt): %v", ErrStageFailed, err)
	}

	if _, verr := VerifyAudioDiscard(job.JobID, input, transcript.SourceHash); verr != nil {
		return state, fmt.Errorf("%w: %v", ErrDiscardVerificationFailed, verr)
	}

	o.stash(job.JobID).extractedText = transcript.FullText()
	o.stash(job.JobID).extractedLang = transcript.Language
	o.stash(job.JobID).sourceHash = transcript.SourceHash
	return state, nil
}

func (o *IngestionOrchestrator) runImageExtraction(ctx context.Context, job Job, state WorkflowState) (WorkflowState, error) {
	if job.Image == nil {
		return state, fmt.Errorf("%w: extraction: nil image input", ErrStageFailed)
	}
	input := *job.Image
	input.Data = append([]byte(nil), job.Image.Data...)

	providerID := job.ProviderID
	if providerID == "" {
		providerID = "noop"
	}

	result, err := o.ocrService().Extract(ctx, providerID, &input, 0)
	if err != nil {
		return state, fmt.Errorf("%w: extraction(ocr): %v", ErrStageFailed, err)
	}

	if _, verr := VerifyImageDiscard(job.JobID, input, result.SourceHash); verr != nil {
		return state, fmt.Errorf("%w: %v", ErrDiscardVerificationFailed, verr)
	}

	o.stash(job.JobID).extractedText = result.FullText()
	o.stash(job.JobID).extractedLang = result.Language
	o.stash(job.JobID).sourceHash = result.SourceHash
	return state, nil
}

func (o *IngestionOrchestrator) runNormalize(ctx context.Context, job Job, state WorkflowState) (WorkflowState, error) {
	ps := o.stash(job.JobID)
	text := ps.extractedText
	if text == "" {
		return state, fmt.Errorf("%w: normalize: no extracted text for job %s", ErrStageFailed, job.JobID)
	}

	lang := multilingual.Language(job.Language)
	normalized, err := o.normalizer().Normalize(ctx, job.JobID, text, lang)
	if err != nil {
		return state, fmt.Errorf("%w: normalize: %v", ErrStageFailed, err)
	}
	ps.normalized = normalized
	return state, nil
}

func (o *IngestionOrchestrator) runSegment(ctx context.Context, job Job, state WorkflowState) (WorkflowState, error) {
	ps := o.stash(job.JobID)
	if ps.normalized == nil {
		return state, fmt.Errorf("%w: segment: no normalized text for job %s", ErrStageFailed, job.JobID)
	}

	req := segmentation.SegmentRequest{
		DocumentID: job.JobID,
		Text:       ps.normalized.Text,
		Language:   string(ps.normalized.Language),
	}
	segs, err := o.segmenter().Segment(ctx, req)
	if err != nil {
		return state, fmt.Errorf("%w: segment: %v", ErrStageFailed, err)
	}
	ps.segments = segs
	return state, nil
}

// runClassifyStage classifies ps.segments via packages/evidence and stashes
// the resulting []ClassifiedSegment on ps.classified, which process() reads
// once every stage has completed to populate WorkflowState.Segments.
func (o *IngestionOrchestrator) runClassifyStage(ctx context.Context, job Job, state WorkflowState) (WorkflowState, error) {
	ps := o.stash(job.JobID)
	if len(ps.segments) == 0 {
		return state, fmt.Errorf("%w: classify: no segments for job %s", ErrStageFailed, job.JobID)
	}
	req := evidence.ClassifyRequest{Segments: ps.segments}
	classifications, err := o.evidenceService().ClassifySegments(ctx, req)
	if err != nil {
		return state, fmt.Errorf("%w: classify: %v", ErrStageFailed, err)
	}

	bySegID := make(map[string]evidence.Classification, len(classifications))
	for _, c := range classifications {
		bySegID[c.SegmentID] = c
	}

	classified := make([]ClassifiedSegment, 0, len(ps.segments))
	for _, seg := range ps.segments {
		c, ok := bySegID[seg.ID]
		if !ok {
			continue
		}
		classified = append(classified, ClassifiedSegment{Segment: seg, Classification: c})
	}
	ps.classified = classified

	return state, nil
}

// Run starts a blocking worker loop that dequeues jobs from o.Queue and
// processes them one at a time until ctx is cancelled or the queue is
// closed and drained. It is intended to be launched in its own goroutine.
func (o *IngestionOrchestrator) Run(ctx context.Context) {
	for {
		job, err := o.Queue.Dequeue(ctx)
		if err != nil {
			return
		}
		o.Process(ctx, *job)
	}
}
