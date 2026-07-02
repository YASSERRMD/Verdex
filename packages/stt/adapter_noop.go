package stt

import (
	"context"
	"fmt"
	"time"
)

// NoOpSTTProvider is a deterministic stub that implements STTProvider.
//
// It is designed for use in unit tests, CI pipelines, and air-gapped
// deployments where a real transcription backend is unnecessary or
// unavailable. It never inspects input.Data content; output is derived
// solely from declared metadata so behaviour is fully deterministic.
//
// Behaviour:
//   - Transcribe returns a single segment spanning the full declared
//     duration (or a fixed synthetic duration if DurationMS is unset),
//     containing FixedText, with a fixed confidence score.
//   - Transcribe sleeps for SimulatedLatency before returning.
type NoOpSTTProvider struct {
	// SimulatedLatency is the artificial delay added before each response.
	// Zero means no delay.
	SimulatedLatency time.Duration

	// FixedText is the Text returned in the single synthetic segment.
	// Defaults to "noop transcript".
	FixedText string

	// FixedConfidence is the Confidence score attached to the synthetic
	// segment. Defaults to 1.0. Must be in [0, 1] or it is clamped.
	FixedConfidence float64
}

// DefaultNoOpSTTProvider returns a NoOpSTTProvider with sensible defaults.
func DefaultNoOpSTTProvider() *NoOpSTTProvider {
	return &NoOpSTTProvider{
		FixedText:       "noop transcript",
		FixedConfidence: 1.0,
	}
}

// ID returns the stable identifier for the no-op provider.
func (n *NoOpSTTProvider) ID() string { return "noop" }

// Capabilities returns a Capability that advertises transcription support
// for any language.
func (n *NoOpSTTProvider) Capabilities() Capability {
	return Capability{
		SupportedTasks:      []TaskType{TaskTranscribe},
		MaxAudioDurationMS:  0, // unbounded
		SupportsDiarization: false,
		SupportedLanguages:  nil, // language-agnostic
		ProviderID:          "noop",
		ModelID:             "noop-v1",
	}
}

// Transcribe returns a deterministic single-segment Transcript after
// sleeping SimulatedLatency. Returns ErrEmptyAudio if input.Data is empty.
func (n *NoOpSTTProvider) Transcribe(ctx context.Context, input AudioInput) (*Transcript, error) {
	if len(input.Data) == 0 {
		return nil, fmt.Errorf("stt noop: %w", ErrEmptyAudio)
	}
	if err := n.sleep(ctx); err != nil {
		return nil, err
	}

	durationMS := input.DurationMS
	if durationMS <= 0 {
		durationMS = 1000
	}

	segment := TranscriptSegment{
		StartMS:    0,
		EndMS:      durationMS,
		Text:       n.fixedText(),
		Confidence: n.fixedConfidence(),
	}

	language := string(input.LanguageHint)

	return &Transcript{
		ProviderID: n.ID(),
		Language:   language,
		Segments:   []TranscriptSegment{segment},
	}, nil
}

func (n *NoOpSTTProvider) fixedText() string {
	if n.FixedText != "" {
		return n.FixedText
	}
	return "noop transcript"
}

func (n *NoOpSTTProvider) fixedConfidence() float64 {
	c := n.FixedConfidence
	if c == 0 {
		c = 1.0
	}
	if c < 0 {
		c = 0
	}
	if c > 1 {
		c = 1
	}
	return c
}

func (n *NoOpSTTProvider) sleep(ctx context.Context) error {
	if n.SimulatedLatency <= 0 {
		return ctx.Err()
	}
	select {
	case <-time.After(n.SimulatedLatency):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
