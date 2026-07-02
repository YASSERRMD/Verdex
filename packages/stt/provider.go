package stt

import "context"

// STTProvider is the contract every concrete speech-to-text adapter must
// satisfy.
//
// Verdex routes all audio/video transcription calls through this interface
// so that adapters (local Whisper-style models, hosted STT APIs, air-gapped
// no-op stubs, etc.) can be registered and swapped without touching business
// logic. This mirrors provider.LLMProvider in packages/provider.
//
// Implementations MUST be safe for concurrent use from multiple goroutines.
type STTProvider interface {
	// ID returns the stable, unique identifier for this provider instance
	// (e.g. "noop", "whisper-local"). The value must be non-empty and match
	// the key used when registering with the Registry.
	ID() string

	// Capabilities returns the static capability descriptor for this
	// provider/model pair. The returned value must not be mutated by the
	// caller.
	Capabilities() Capability

	// Transcribe converts the audio described by input into a timestamped
	// Transcript. Implementations should honour ctx.Done() and return a
	// wrapped context error promptly.
	//
	// Transcribe must not itself discard or mutate input.Data; discard is
	// the caller's (STTService's) responsibility so that provenance hashing
	// can occur around the provider call.
	Transcribe(ctx context.Context, input AudioInput) (*Transcript, error)
}
