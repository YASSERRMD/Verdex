package stt

import (
	"context"
	"fmt"
)

// STTService orchestrates the full speech-to-text pipeline:
//
//	normalize -> provider.Transcribe -> diarize -> discard source -> return
//
// It is the primary entry point application code should use rather than
// calling Normalize, an STTProvider, and Discard directly.
type STTService struct {
	registry *Registry
	diarizer Diarizer
	sink     AudioDiscardSink

	// MaxChunkMS bounds how long a single segment fed to the provider may
	// be. Zero disables chunking (the whole AudioInput is sent as one
	// request).
	MaxChunkMS int64
}

// NewSTTService constructs an STTService.
//
//   - registry: the Registry used to resolve providers by ID; if nil,
//     DefaultRegistry is used.
//   - diarizer: the Diarizer applied to the assembled transcript; if nil,
//     NoOpDiarizer{} is used.
//   - sink: the AudioDiscardSink that receives discard audit events; if nil,
//     NoOpDiscardSink{} is used.
func NewSTTService(registry *Registry, diarizer Diarizer, sink AudioDiscardSink) *STTService {
	if registry == nil {
		registry = DefaultRegistry
	}
	if diarizer == nil {
		diarizer = NoOpDiarizer{}
	}
	if sink == nil {
		sink = NoOpDiscardSink{}
	}
	return &STTService{
		registry: registry,
		diarizer: diarizer,
		sink:     sink,
	}
}

// Transcribe runs the full pipeline for a single AudioInput using the
// provider registered under providerID:
//
//  1. Normalize fills in sample-rate/channel defaults.
//  2. ComputeSourceHash captures the provenance hash before any mutation.
//  3. The provider transcribes the (optionally chunked) audio.
//  4. The configured Diarizer assigns speaker labels.
//  5. Discard zeroes the source bytes and emits an audit event.
//
// The returned *Transcript has SourceHash populated. input is mutated in
// place: after Transcribe returns (success or failure past step 2),
// input.Data has been discarded (zeroed and truncated) via Discard.
func (s *STTService) Transcribe(ctx context.Context, providerID string, input *AudioInput) (*Transcript, error) {
	if input == nil {
		return nil, fmt.Errorf("stt service: %w: input must not be nil", ErrInvalidRequest)
	}

	p, err := s.registry.Get(providerID)
	if err != nil {
		return nil, fmt.Errorf("stt service: %w", err)
	}

	normalized, err := Normalize(*input)
	if err != nil {
		return nil, fmt.Errorf("stt service: normalize: %w", err)
	}

	sourceHash := ComputeSourceHash(normalized.Data)

	transcript, transcribeErr := s.transcribe(ctx, p, normalized)

	// Discard always runs, even on transcription failure, to uphold the
	// transcribe-and-discard guarantee: source bytes must not outlive the
	// attempt.
	discardErr := Discard(ctx, input, sourceHash, providerID, s.sink)

	if transcribeErr != nil {
		return nil, fmt.Errorf("stt service: transcribe: %w", transcribeErr)
	}
	if discardErr != nil {
		return nil, fmt.Errorf("stt service: discard: %w", discardErr)
	}

	transcript.SourceHash = sourceHash

	diarized, err := s.diarizer.Diarize(ctx, transcript.Segments)
	if err != nil {
		return nil, fmt.Errorf("stt service: diarize: %w", err)
	}
	transcript.Segments = diarized

	return transcript, nil
}

// transcribe performs steps 3 of the pipeline, optionally chunking the audio
// when MaxChunkMS is configured and the audio exceeds it.
func (s *STTService) transcribe(ctx context.Context, p STTProvider, input AudioInput) (*Transcript, error) {
	if s.MaxChunkMS <= 0 || input.DurationMS <= s.MaxChunkMS {
		return p.Transcribe(ctx, input)
	}

	chunks, err := Segment(input, s.MaxChunkMS)
	if err != nil {
		return nil, fmt.Errorf("segment: %w", err)
	}

	chunkSegments := make([][]TranscriptSegment, len(chunks))
	offsets := make([]int64, len(chunks))
	var language string
	var providerID string

	for i, c := range chunks {
		chunkInput := AudioInput{
			Data:         c.Data,
			MIMEType:     input.MIMEType,
			SampleRateHz: input.SampleRateHz,
			Channels:     input.Channels,
			DurationMS:   c.DurationMS,
			LanguageHint: input.LanguageHint,
		}
		t, err := p.Transcribe(ctx, chunkInput)
		if err != nil {
			return nil, fmt.Errorf("transcribe chunk %d: %w", i, err)
		}
		chunkSegments[i] = t.Segments
		offsets[i] = c.OffsetMS
		if language == "" {
			language = t.Language
		}
		if providerID == "" {
			providerID = t.ProviderID
		}
	}

	return AssembleTranscript(providerID, language, chunkSegments, offsets), nil
}
