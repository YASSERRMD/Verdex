package ingestion

import (
	"fmt"

	"github.com/YASSERRMD/verdex/packages/ocr"
	"github.com/YASSERRMD/verdex/packages/stt"
)

// DiscardVerification is the result of checking whether a job's source
// binary was actually discarded by the underlying stt/ocr service after
// extraction.
type DiscardVerification struct {
	// JobID identifies the job whose source was checked.
	JobID string

	// SourceHash is the provenance hash recorded before discard (from
	// Transcript.SourceHash / ExtractionResult.SourceHash).
	SourceHash string

	// BytesZeroed reports whether the source buffer is now empty, matching
	// the stt/ocr transcribe-and-discard guarantee (Data truncated to zero
	// length after zeroing).
	BytesZeroed bool

	// Verified reports whether verification passed: SourceHash is
	// non-empty AND BytesZeroed is true.
	Verified bool
}

// VerifyAudioDiscard asserts the transcribe-and-discard guarantee for an
// audio job: sourceHash (captured by packages/stt.STTService.Transcribe
// before discard) must be non-empty, and input.Data must have been zeroed
// (stt.IsDiscarded(input) == true).
//
// Returns ErrDiscardVerificationFailed if either condition does not hold.
func VerifyAudioDiscard(jobID string, input stt.AudioInput, sourceHash string) (DiscardVerification, error) {
	v := DiscardVerification{
		JobID:       jobID,
		SourceHash:  sourceHash,
		BytesZeroed: stt.IsDiscarded(input),
	}
	v.Verified = v.SourceHash != "" && v.BytesZeroed
	if !v.Verified {
		return v, fmt.Errorf("%w: job=%s hash_set=%t bytes_zeroed=%t", ErrDiscardVerificationFailed, jobID, v.SourceHash != "", v.BytesZeroed)
	}
	return v, nil
}

// VerifyImageDiscard asserts the transcribe-and-discard guarantee for an
// image/document job: sourceHash (captured by
// packages/ocr.OCRService.Extract before discard) must be non-empty, and
// input.Data must have been zeroed (ocr.IsDiscarded(input) == true).
//
// Returns ErrDiscardVerificationFailed if either condition does not hold.
func VerifyImageDiscard(jobID string, input ocr.ImageInput, sourceHash string) (DiscardVerification, error) {
	v := DiscardVerification{
		JobID:       jobID,
		SourceHash:  sourceHash,
		BytesZeroed: ocr.IsDiscarded(input),
	}
	v.Verified = v.SourceHash != "" && v.BytesZeroed
	if !v.Verified {
		return v, fmt.Errorf("%w: job=%s hash_set=%t bytes_zeroed=%t", ErrDiscardVerificationFailed, jobID, v.SourceHash != "", v.BytesZeroed)
	}
	return v, nil
}
