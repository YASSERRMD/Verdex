// Package stt implements the model-agnostic speech-to-text pipeline used
// throughout the Verdex judicial reasoning platform.
//
// All audio/video transcription inside Verdex is routed through the
// STTProvider interface defined in this package so that concrete adapters
// (local models, hosted STT APIs, air-gapped no-op stubs, etc.) can be
// registered and swapped without touching business logic. This mirrors the
// provider.LLMProvider abstraction in packages/provider.
//
// Core concepts:
//
//   - STTProvider: the interface every adapter must implement.
//   - Registry: a process-level map from provider IDs to STTProvider
//     instances.
//   - AudioInput: a provider-neutral description of a raw audio/video
//     payload (bytes plus declared metadata); no specific codec is assumed.
//   - Normalize / Segment: deterministic metadata normalization and
//     bounded-duration chunking, operating purely on declared metadata.
//   - Diarizer: an interface for assigning SpeakerLabel values to transcript
//     segments, with NoOpDiarizer as the deterministic default.
//   - Transcript / TranscriptSegment: the timestamped, confidence-scored
//     output of transcription.
//   - STTService: orchestrates normalize -> provider.Transcribe -> diarize
//     -> discard source -> return *Transcript.
//   - Discard / DiscardAuditEvent: the transcribe-and-discard guarantee —
//     source audio bytes are zeroed immediately after transcription
//     completes, with the SHA-256 provenance hash captured beforehand and a
//     discard audit event emitted, mirroring packages/intake's discard
//     guarantee for uploaded artifacts.
//   - NoOpSTTProvider: a deterministic stub useful in tests, CI, and
//     air-gapped deployments.
//
// See doc/stt-pipeline.md for a detailed design write-up.
package stt
