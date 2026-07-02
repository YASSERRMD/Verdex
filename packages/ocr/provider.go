package ocr

import "context"

// OCRProvider is the contract every concrete optical-character-recognition
// adapter must satisfy.
//
// Verdex routes all image/scanned-document text-extraction calls through
// this interface so that adapters (local OCR engines, hosted OCR APIs,
// air-gapped no-op stubs, etc.) can be registered and swapped without
// touching business logic. This mirrors provider.LLMProvider in
// packages/provider and STTProvider in packages/stt.
//
// Implementations MUST be safe for concurrent use from multiple goroutines.
type OCRProvider interface {
	// ID returns the stable, unique identifier for this provider instance
	// (e.g. "noop", "tesseract-local"). The value must be non-empty and
	// match the key used when registering with the Registry.
	ID() string

	// Capabilities returns the static capability descriptor for this
	// provider/model pair. The returned value must not be mutated by the
	// caller.
	Capabilities() Capability

	// Extract converts the image described by input into an
	// ExtractionResult containing ordered, confidence-scored text blocks.
	// Implementations should honour ctx.Done() and return a wrapped context
	// error promptly.
	//
	// Extract must not itself discard or mutate input.Data; discard is the
	// caller's (OCRService's) responsibility so that provenance hashing can
	// occur around the provider call.
	Extract(ctx context.Context, input ImageInput) (*ExtractionResult, error)
}
