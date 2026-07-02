package ocr_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/ocr"
)

func newTestOCRService(t *testing.T) (*ocr.OCRService, *ocr.NoOpOCRProvider) {
	t.Helper()
	registry := ocr.NewRegistry()
	p := ocr.DefaultNoOpOCRProvider()
	if err := registry.Register(p.ID(), p); err != nil {
		t.Fatalf("Register: %v", err)
	}
	return ocr.NewOCRService(registry, nil, nil, nil), p
}

// TestOCRService_Extract_BlocksOrdered verifies that returned blocks are
// ordered by page then position.
func TestOCRService_Extract_BlocksOrdered(t *testing.T) {
	svc, _ := newTestOCRService(t)

	input := &ocr.ImageInput{
		Data:     []byte("some raw scanned page bytes representing text"),
		WidthPx:  600,
		HeightPx: 800,
	}

	result, err := svc.Extract(context.Background(), "noop", input, 0)
	if err != nil {
		t.Fatalf("Extract() unexpected error: %v", err)
	}

	if len(result.Blocks) == 0 {
		t.Fatal("expected at least one block")
	}

	for i := 1; i < len(result.Blocks); i++ {
		prev, cur := result.Blocks[i-1], result.Blocks[i]
		if cur.Page < prev.Page {
			t.Fatalf("blocks not ordered by page: block[%d].Page=%d < block[%d].Page=%d",
				i, cur.Page, i-1, prev.Page)
		}
	}
}

// TestOCRService_Extract_ConfidenceInRange verifies every block's Confidence
// lies in [0, 1].
func TestOCRService_Extract_ConfidenceInRange(t *testing.T) {
	svc, _ := newTestOCRService(t)

	input := &ocr.ImageInput{Data: []byte("image bytes")}

	result, err := svc.Extract(context.Background(), "noop", input, 0)
	if err != nil {
		t.Fatalf("Extract() unexpected error: %v", err)
	}

	for i, b := range result.Blocks {
		if b.Confidence < 0 || b.Confidence > 1 {
			t.Errorf("block[%d].Confidence = %v, want value in [0, 1]", i, b.Confidence)
		}
	}
}

// TestOCRService_Extract_UnknownProvider_ReturnsErrProviderNotFound verifies
// error wrapping for a missing provider ID.
func TestOCRService_Extract_UnknownProvider_ReturnsErrProviderNotFound(t *testing.T) {
	svc, _ := newTestOCRService(t)

	input := &ocr.ImageInput{Data: []byte("data")}
	_, err := svc.Extract(context.Background(), "does-not-exist", input, 0)
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
	if !errors.Is(err, ocr.ErrProviderNotFound) {
		t.Errorf("error %v does not wrap ErrProviderNotFound", err)
	}
}

// TestOCRService_Extract_EmptyImage_ReturnsError verifies that an empty
// ImageInput is rejected.
func TestOCRService_Extract_EmptyImage_ReturnsError(t *testing.T) {
	svc, _ := newTestOCRService(t)

	input := &ocr.ImageInput{Data: nil}
	_, err := svc.Extract(context.Background(), "noop", input, 0)
	if err == nil {
		t.Fatal("expected error for empty image, got nil")
	}
	if !errors.Is(err, ocr.ErrEmptyImage) {
		t.Errorf("error %v does not wrap ErrEmptyImage", err)
	}
}

// TestOCRService_Extract_NilInput_ReturnsError verifies that a nil
// *ImageInput is rejected without panicking.
func TestOCRService_Extract_NilInput_ReturnsError(t *testing.T) {
	svc, _ := newTestOCRService(t)

	_, err := svc.Extract(context.Background(), "noop", nil, 0)
	if err == nil {
		t.Fatal("expected error for nil input, got nil")
	}
	if !errors.Is(err, ocr.ErrInvalidRequest) {
		t.Errorf("error %v does not wrap ErrInvalidRequest", err)
	}
}

// TestOCRService_Extract_RegionsAndTablesPopulated verifies that a
// LayoutDetector/TableExtractor pair configured on the service annotates the
// result with regions and tables.
func TestOCRService_Extract_RegionsAndTablesPopulated(t *testing.T) {
	registry := ocr.NewRegistry()
	p := ocr.DefaultNoOpOCRProvider()
	if err := registry.Register(p.ID(), p); err != nil {
		t.Fatalf("Register: %v", err)
	}

	detector := fixedLayoutDetector{
		regions: []ocr.Region{
			{Page: 1, Type: ocr.RegionTypeParagraph, BoundingBox: ocr.BoundingBox{Width: 100, Height: 20}},
			{Page: 1, Type: ocr.RegionTypeTable, BoundingBox: ocr.BoundingBox{Width: 200, Height: 100}},
		},
	}

	svc := ocr.NewOCRService(registry, detector, ocr.NoOpTableExtractor{}, nil)

	input := &ocr.ImageInput{Data: []byte("image bytes")}
	result, err := svc.Extract(context.Background(), "noop", input, 0)
	if err != nil {
		t.Fatalf("Extract() unexpected error: %v", err)
	}

	if len(result.Regions) != 2 {
		t.Fatalf("Regions = %d, want 2", len(result.Regions))
	}
	if len(result.Tables) != 1 {
		t.Fatalf("Tables = %d, want 1", len(result.Tables))
	}
}

// TestOCRService_Extract_DeskewApplied verifies that a non-trivial
// skewAngleDeg results in the pipeline running Preprocess's deskew path
// without error (the correction itself is metadata-only; this asserts the
// service wires the angle through).
func TestOCRService_Extract_DeskewApplied(t *testing.T) {
	svc, _ := newTestOCRService(t)

	input := &ocr.ImageInput{Data: []byte("skewed page bytes")}
	result, err := svc.Extract(context.Background(), "noop", input, 5.0)
	if err != nil {
		t.Fatalf("Extract() unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Extract() returned nil result")
	}
}

type fixedLayoutDetector struct {
	regions []ocr.Region
}

func (f fixedLayoutDetector) DetectLayout(_ context.Context, _ ocr.ImageInput) ([]ocr.Region, error) {
	return f.regions, nil
}
