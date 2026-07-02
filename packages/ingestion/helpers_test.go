package ingestion

import (
	"github.com/YASSERRMD/verdex/packages/ocr"
	"github.com/YASSERRMD/verdex/packages/stt"
)

// dummyAudio is a minimal non-empty stt.AudioInput used by tests that only
// need a structurally valid Job, not a real transcription pipeline run.
var dummyAudio = stt.AudioInput{
	Data:       []byte("dummy audio bytes"),
	MIMEType:   "audio/wav",
	DurationMS: 1000,
}

// dummyImage is a minimal non-empty ocr.ImageInput used by tests that only
// need a structurally valid Job, not a real extraction pipeline run.
var dummyImage = ocr.ImageInput{
	Data:     []byte("dummy image bytes"),
	MIMEType: "image/png",
}

// newAudioJob returns a well-formed audio Job with a fresh copy of
// dummyAudio's bytes (so tests that mutate/discard Data don't interfere
// with one another).
func newAudioJob(jobID, caseID string) Job {
	data := append([]byte(nil), dummyAudio.Data...)
	return Job{
		JobID:      jobID,
		CaseID:     caseID,
		Kind:       InputAudio,
		ProviderID: "noop",
		Audio: &stt.AudioInput{
			Data:       data,
			MIMEType:   dummyAudio.MIMEType,
			DurationMS: dummyAudio.DurationMS,
		},
	}
}

// newImageJob returns a well-formed image Job with a fresh copy of
// dummyImage's bytes.
func newImageJob(jobID, caseID string) Job {
	data := append([]byte(nil), dummyImage.Data...)
	return Job{
		JobID:      jobID,
		CaseID:     caseID,
		Kind:       InputImage,
		ProviderID: "noop",
		Image: &ocr.ImageInput{
			Data:     data,
			MIMEType: dummyImage.MIMEType,
		},
	}
}
