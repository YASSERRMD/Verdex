package ocr

import (
	"context"
	"fmt"
)

// OCRService orchestrates the full OCR pipeline:
//
//	preprocess -> provider.Extract -> layout/table extraction -> discard source -> return
//
// It is the primary entry point application code should use rather than
// calling Preprocess, an OCRProvider, LayoutDetector/TableExtractor, and
// Discard directly.
type OCRService struct {
	registry       *Registry
	layoutDetector LayoutDetector
	tableExtractor TableExtractor
	sink           ImageDiscardSink
}

// NewOCRService constructs an OCRService.
//
//   - registry: the Registry used to resolve providers by ID; if nil,
//     DefaultRegistry is used.
//   - layoutDetector: the LayoutDetector applied after extraction; if nil,
//     NoOpLayoutDetector{} is used.
//   - tableExtractor: the TableExtractor applied to any regions classified
//     as RegionTypeTable; if nil, NoOpTableExtractor{} is used.
//   - sink: the ImageDiscardSink that receives discard audit events; if nil,
//     NoOpDiscardSink{} is used.
func NewOCRService(registry *Registry, layoutDetector LayoutDetector, tableExtractor TableExtractor, sink ImageDiscardSink) *OCRService {
	if registry == nil {
		registry = DefaultRegistry
	}
	if layoutDetector == nil {
		layoutDetector = NoOpLayoutDetector{}
	}
	if tableExtractor == nil {
		tableExtractor = NoOpTableExtractor{}
	}
	if sink == nil {
		sink = NoOpDiscardSink{}
	}
	return &OCRService{
		registry:       registry,
		layoutDetector: layoutDetector,
		tableExtractor: tableExtractor,
		sink:           sink,
	}
}

// Extract runs the full pipeline for a single ImageInput using the provider
// registered under providerID:
//
//  1. Preprocess applies deterministic metadata-level deskew/denoise
//     corrections (skewAngleDeg biases the deskew step; pass 0 if unknown).
//  2. ComputeSourceHash captures the provenance hash before any mutation.
//  3. The provider extracts text into an ExtractionResult.
//  4. The configured LayoutDetector identifies regions.
//  5. The configured TableExtractor extracts Table data from any
//     RegionTypeTable regions.
//  6. Discard zeroes the source bytes and emits an audit event.
//
// The returned *ExtractionResult has SourceHash, Regions, and Tables
// populated. input is mutated in place: after Extract returns (success or
// failure past step 2), input.Data has been discarded (zeroed and
// truncated) via Discard.
func (s *OCRService) Extract(ctx context.Context, providerID string, input *ImageInput, skewAngleDeg float64) (*ExtractionResult, error) {
	if input == nil {
		return nil, fmt.Errorf("ocr service: %w: input must not be nil", ErrInvalidRequest)
	}

	p, err := s.registry.Get(providerID)
	if err != nil {
		return nil, fmt.Errorf("ocr service: %w", err)
	}

	pre, err := Preprocess(*input, skewAngleDeg)
	if err != nil {
		return nil, fmt.Errorf("ocr service: preprocess: %w", err)
	}
	normalized := pre.Image

	sourceHash := ComputeSourceHash(normalized.Data)

	result, extractErr := s.extractAndAnnotate(ctx, p, normalized)

	// Discard always runs, even on extraction failure, to uphold the
	// transcribe-and-discard guarantee: source bytes must not outlive the
	// attempt.
	discardErr := Discard(ctx, input, sourceHash, providerID, s.sink)

	if extractErr != nil {
		return nil, fmt.Errorf("ocr service: extract: %w", extractErr)
	}
	if discardErr != nil {
		return nil, fmt.Errorf("ocr service: discard: %w", discardErr)
	}

	result.SourceHash = sourceHash
	return result, nil
}

// extractAndAnnotate performs steps 3-5 of the pipeline.
func (s *OCRService) extractAndAnnotate(ctx context.Context, p OCRProvider, input ImageInput) (*ExtractionResult, error) {
	result, err := p.Extract(ctx, input)
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = &ExtractionResult{ProviderID: p.ID()}
	}

	result.SortBlocks()

	regions, err := s.layoutDetector.DetectLayout(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("layout: %w", err)
	}
	result.Regions = regions

	tables, err := ExtractTablesFromRegions(ctx, s.tableExtractor, input, regions)
	if err != nil {
		return nil, fmt.Errorf("table: %w", err)
	}
	result.Tables = tables

	return result, nil
}
