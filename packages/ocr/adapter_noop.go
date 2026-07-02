package ocr

import (
	"context"
	"fmt"
	"time"
)

// NoOpOCRProvider is a deterministic stub that implements OCRProvider.
//
// It is designed for use in unit tests, CI pipelines, and air-gapped
// deployments where a real OCR backend is unnecessary or unavailable. It
// never inspects input.Data content; output is derived solely from declared
// metadata so behaviour is fully deterministic.
//
// Behaviour:
//   - Extract returns a single text block covering the whole page,
//     containing FixedText, with a fixed confidence score.
//   - Extract sleeps for SimulatedLatency before returning.
type NoOpOCRProvider struct {
	// SimulatedLatency is the artificial delay added before each response.
	// Zero means no delay.
	SimulatedLatency time.Duration

	// FixedText is the Text returned in the single synthetic block. Defaults
	// to "noop extracted text".
	FixedText string

	// FixedConfidence is the Confidence score attached to the synthetic
	// block. Defaults to 1.0. Must be in [0, 1] or it is clamped.
	FixedConfidence float64
}

// DefaultNoOpOCRProvider returns a NoOpOCRProvider with sensible defaults.
func DefaultNoOpOCRProvider() *NoOpOCRProvider {
	return &NoOpOCRProvider{
		FixedText:       "noop extracted text",
		FixedConfidence: 1.0,
	}
}

// ID returns the stable identifier for the no-op provider.
func (n *NoOpOCRProvider) ID() string { return "noop" }

// Capabilities returns a Capability that advertises text extraction support
// for any language.
func (n *NoOpOCRProvider) Capabilities() Capability {
	return Capability{
		SupportedTasks:          []TaskType{TaskExtractText},
		MaxImageDimensionPx:     0, // unbounded
		SupportsLayoutDetection: false,
		SupportsTableExtraction: false,
		SupportedLanguages:      nil, // language-agnostic
		ProviderID:              "noop",
		ModelID:                 "noop-v1",
	}
}

// Extract returns a deterministic single-block ExtractionResult after
// sleeping SimulatedLatency. Returns ErrEmptyImage if input.Data is empty.
func (n *NoOpOCRProvider) Extract(ctx context.Context, input ImageInput) (*ExtractionResult, error) {
	if len(input.Data) == 0 {
		return nil, fmt.Errorf("ocr noop: %w", ErrEmptyImage)
	}
	if err := n.sleep(ctx); err != nil {
		return nil, err
	}

	page := input.PageNumber
	if page <= 0 {
		page = 1
	}

	width := input.WidthPx
	height := input.HeightPx

	block := TextBlock{
		Text:        n.fixedText(),
		Confidence:  n.fixedConfidence(),
		Page:        page,
		BoundingBox: BoundingBox{X: 0, Y: 0, Width: width, Height: height},
	}

	language := string(input.LanguageHint)

	return &ExtractionResult{
		ProviderID: n.ID(),
		Language:   language,
		Blocks:     []TextBlock{block},
	}, nil
}

func (n *NoOpOCRProvider) fixedText() string {
	if n.FixedText != "" {
		return n.FixedText
	}
	return "noop extracted text"
}

func (n *NoOpOCRProvider) fixedConfidence() float64 {
	c := n.FixedConfidence
	if c == 0 {
		c = 1.0
	}
	return ClampConfidence(c)
}

func (n *NoOpOCRProvider) sleep(ctx context.Context) error {
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
