package stt

import "time"

// AudioInput describes a raw audio/video payload submitted for transcription.
//
// Verdex never assumes a specific codec or container: the pipeline treats the
// payload as an opaque byte slice accompanied by declared metadata. Real
// adapters are responsible for decoding whatever container/codec the bytes
// represent; the abstractions in this package operate purely on the
// metadata and byte length.
type AudioInput struct {
	// Data holds the raw audio/video bytes. It is mutated in place (zeroed)
	// by Discard once transcription has completed; callers must not retain
	// external references to this slice if they need it after discard.
	Data []byte

	// MIMEType is the declared MIME type of the payload (e.g. "audio/wav",
	// "audio/mpeg", "video/mp4").
	MIMEType string

	// SampleRateHz is the declared sample rate in Hz (e.g. 16000, 44100).
	// A zero value means unknown/unspecified.
	SampleRateHz int

	// Channels is the declared channel count (1 = mono, 2 = stereo).
	// A zero value means unknown/unspecified.
	Channels int

	// DurationMS is the declared duration of the payload in milliseconds.
	// A zero value means unknown/unspecified.
	DurationMS int64

	// LanguageHint optionally biases transcription toward a specific
	// language. May be the zero value if no hint is available.
	LanguageHint LanguageHint
}

// TaskType classifies the kind of work an STT provider call performs.
type TaskType string

const (
	// TaskTranscribe is a standard speech-to-text transcription task.
	TaskTranscribe TaskType = "transcribe"
	// TaskDiarize is a speaker-diarization task.
	TaskDiarize TaskType = "diarize"
	// TaskTranslate is a speech-to-text task that also translates into a
	// target language.
	TaskTranslate TaskType = "translate"
)

// Capability describes what a specific STT provider/model combination can do.
type Capability struct {
	// SupportedTasks lists the TaskType values this provider handles.
	SupportedTasks []TaskType
	// MaxAudioDurationMS is the maximum single-request audio duration the
	// provider accepts, in milliseconds. Zero means unbounded/unspecified.
	MaxAudioDurationMS int64
	// SupportsDiarization indicates whether the provider can label speakers
	// natively (as opposed to relying on a separate Diarizer).
	SupportsDiarization bool
	// SupportedLanguages lists ISO 639-1 language codes the provider can
	// transcribe. An empty slice means the provider is language-agnostic or
	// auto-detects.
	SupportedLanguages []string
	// ProviderID identifies the provider (e.g. "noop", "whisper-local").
	ProviderID string
	// ModelID identifies the specific model, if applicable.
	ModelID string
}

// TranscribeLatency is a convenience type alias used by adapters to report
// how long a Transcribe call took.
type TranscribeLatency = time.Duration
