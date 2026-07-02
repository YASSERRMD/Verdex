package ingestion

import "time"

// Stage identifies a single step in the ingestion pipeline. The pipeline
// moves a job through these stages strictly in order (with StageExtraction
// branching internally between STT and OCR depending on the job's input
// kind), until it reaches a terminal stage: StageComplete or StageFailed.
type Stage string

const (
	// StageIntake covers upload validation, hashing, virus-scanning, and
	// buffering via packages/intake.
	StageIntake Stage = "intake"

	// StageExtraction covers turning the intake artifact into text: audio
	// input is routed through packages/stt (transcription), image/document
	// input is routed through packages/ocr (text extraction).
	StageExtraction Stage = "extraction"

	// StageNormalize covers Unicode normalization, script/language
	// detection, and legal-term normalization via packages/multilingual.
	StageNormalize Stage = "normalize"

	// StageSegment covers splitting normalized text into segments via
	// packages/segmentation.
	StageSegment Stage = "segment"

	// StageClassify covers evidentiary classification of segments via
	// packages/evidence.
	StageClassify Stage = "classify"

	// StageComplete is the terminal success stage: every prior stage ran
	// and the job's final classified segments are available.
	StageComplete Stage = "complete"

	// StageFailed is the terminal failure stage: the job could not proceed
	// past some stage even after retries were exhausted.
	StageFailed Stage = "failed"
)

// stageOrder is the fixed linear ordering of non-terminal stages. It backs
// NextStage and Stage.Index.
var stageOrder = []Stage{
	StageIntake,
	StageExtraction,
	StageNormalize,
	StageSegment,
	StageClassify,
	StageComplete,
}

// Index returns s's zero-based position in the canonical pipeline order.
// Returns -1 for StageFailed or any unrecognized value.
func (s Stage) Index() int {
	for i, st := range stageOrder {
		if st == s {
			return i
		}
	}
	return -1
}

// IsTerminal reports whether s is a terminal stage (StageComplete or
// StageFailed) that the workflow does not advance past.
func (s Stage) IsTerminal() bool {
	return s == StageComplete || s == StageFailed
}

// NextStage returns the stage that follows s in the canonical pipeline
// order. It returns StageFailed (ok=false) if s is already terminal or
// unrecognized.
func NextStage(s Stage) (next Stage, ok bool) {
	idx := s.Index()
	if idx < 0 || idx >= len(stageOrder)-1 {
		return StageFailed, false
	}
	return stageOrder[idx+1], true
}

// InputKind classifies the kind of source artifact a job carries, which
// determines whether StageExtraction routes through packages/stt or
// packages/ocr.
type InputKind string

const (
	// InputAudio routes StageExtraction through packages/stt.
	InputAudio InputKind = "audio"

	// InputImage routes StageExtraction through packages/ocr.
	InputImage InputKind = "image"
)

// WorkflowState tracks the current stage and terminal outcome for a single
// ingestion job. A *WorkflowState is the unit persisted by IdempotencyStore
// and RecoveryStore, and returned by IngestionStatusAPI.GetStatus.
type WorkflowState struct {
	// JobID identifies the job this state describes.
	JobID string

	// CaseID identifies the case the job's artifact belongs to.
	CaseID string

	// Stage is the job's current stage.
	Stage Stage

	// AttemptsForStage counts retry attempts made for the current Stage
	// (reset to 0 whenever the job advances to a new stage).
	AttemptsForStage int

	// FailureReason holds the last error message recorded for this job, if
	// any. Populated when Stage == StageFailed, and may also be populated
	// transiently while retries are in progress for a non-terminal stage.
	FailureReason string

	// UpdatedAt is the wall-clock time this WorkflowState was last mutated.
	UpdatedAt time.Time

	// Segments holds the final classified segments once Stage ==
	// StageComplete. Nil until then.
	Segments []ClassifiedSegment
}

// Clone returns a deep-enough copy of s suitable for handing to callers
// without exposing the orchestrator's internal state to mutation.
func (s WorkflowState) Clone() WorkflowState {
	cp := s
	if s.Segments != nil {
		cp.Segments = append([]ClassifiedSegment(nil), s.Segments...)
	}
	return cp
}

// Advance returns a copy of s moved to stage, with AttemptsForStage reset
// to 0 and UpdatedAt refreshed. FailureReason is cleared unless stage is
// StageFailed.
func (s WorkflowState) Advance(stage Stage, now time.Time) WorkflowState {
	next := s
	next.Stage = stage
	next.AttemptsForStage = 0
	next.UpdatedAt = now
	if stage != StageFailed {
		next.FailureReason = ""
	}
	return next
}
