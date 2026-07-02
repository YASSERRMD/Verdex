package ingestion

import (
	"github.com/YASSERRMD/verdex/packages/evidence"
	"github.com/YASSERRMD/verdex/packages/ocr"
	"github.com/YASSERRMD/verdex/packages/segmentation"
	"github.com/YASSERRMD/verdex/packages/stt"
)

// Job describes a single unit of ingestion work: one source artifact (audio
// or image/document) belonging to one case, to be carried through every
// pipeline stage.
//
// Exactly one of Audio or Image should be populated, matching Kind. The
// orchestrator mutates neither field's underlying bytes directly; discard
// is performed by the sibling stt/ocr services themselves, and
// DiscardVerifier asserts it happened.
type Job struct {
	// JobID uniquely identifies this job. Also used as the idempotency key
	// for stage execution (see retry.go).
	JobID string

	// CaseID identifies the case this job's artifact belongs to. Multiple
	// jobs may share a CaseID (e.g. several exhibits in the same case).
	CaseID string

	// Kind selects which extraction path StageExtraction takes.
	Kind InputKind

	// ProviderID is the stt/ocr provider ID to use for extraction (e.g.
	// "noop"). Resolved against the relevant package's DefaultRegistry.
	ProviderID string

	// Audio carries the source payload when Kind == InputAudio.
	Audio *stt.AudioInput

	// Image carries the source payload when Kind == InputImage.
	Image *ocr.ImageInput

	// Language optionally seeds StageNormalize's detected-language
	// fallback and is propagated onto produced segments. May be empty.
	Language string
}

// Validate reports whether j is structurally well-formed enough to enter
// the pipeline.
func (j Job) Validate() error {
	if j.JobID == "" || j.CaseID == "" {
		return ErrInvalidJob
	}
	switch j.Kind {
	case InputAudio:
		if j.Audio == nil {
			return ErrInvalidJob
		}
	case InputImage:
		if j.Image == nil {
			return ErrInvalidJob
		}
	default:
		return ErrInvalidJob
	}
	return nil
}

// ClassifiedSegment pairs a segmentation.Segment with the
// evidence.Classification the pipeline assigned to it — the final output
// unit produced once a job reaches StageComplete.
type ClassifiedSegment struct {
	Segment        segmentation.Segment
	Classification evidence.Classification
}
