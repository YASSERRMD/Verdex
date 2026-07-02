package ocr_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/ocr"
)

func TestNoOpOCRProvider_Extract_EmptyImage_ReturnsError(t *testing.T) {
	p := ocr.DefaultNoOpOCRProvider()

	_, err := p.Extract(context.Background(), ocr.ImageInput{})
	if !errors.Is(err, ocr.ErrEmptyImage) {
		t.Errorf("Extract() error = %v, want ErrEmptyImage", err)
	}
}

func TestNoOpOCRProvider_Extract_DefaultsAppliedWhenZero(t *testing.T) {
	p := &ocr.NoOpOCRProvider{}

	result, err := p.Extract(context.Background(), ocr.ImageInput{Data: []byte("page")})
	if err != nil {
		t.Fatalf("Extract() unexpected error: %v", err)
	}
	if len(result.Blocks) != 1 {
		t.Fatalf("Blocks = %d, want 1", len(result.Blocks))
	}
	if result.Blocks[0].Text != "noop extracted text" {
		t.Errorf("Text = %q, want default", result.Blocks[0].Text)
	}
	if result.Blocks[0].Confidence != 1.0 {
		t.Errorf("Confidence = %v, want 1.0", result.Blocks[0].Confidence)
	}
	if result.Blocks[0].Page != 1 {
		t.Errorf("Page = %d, want 1 (default)", result.Blocks[0].Page)
	}
}

func TestNoOpOCRProvider_Extract_UsesConfiguredFields(t *testing.T) {
	p := &ocr.NoOpOCRProvider{FixedText: "custom", FixedConfidence: 0.42}

	result, err := p.Extract(context.Background(), ocr.ImageInput{
		Data:       []byte("page"),
		PageNumber: 3,
		WidthPx:    1200,
		HeightPx:   1600,
	})
	if err != nil {
		t.Fatalf("Extract() unexpected error: %v", err)
	}
	block := result.Blocks[0]
	if block.Text != "custom" {
		t.Errorf("Text = %q, want %q", block.Text, "custom")
	}
	if block.Confidence != 0.42 {
		t.Errorf("Confidence = %v, want 0.42", block.Confidence)
	}
	if block.Page != 3 {
		t.Errorf("Page = %d, want 3", block.Page)
	}
	if block.BoundingBox.Width != 1200 || block.BoundingBox.Height != 1600 {
		t.Errorf("BoundingBox = %+v, want width 1200 height 1600", block.BoundingBox)
	}
}

func TestNoOpOCRProvider_Capabilities(t *testing.T) {
	p := ocr.DefaultNoOpOCRProvider()
	caps := p.Capabilities()

	if caps.ProviderID != "noop" {
		t.Errorf("ProviderID = %q, want %q", caps.ProviderID, "noop")
	}
	if len(caps.SupportedTasks) != 1 || caps.SupportedTasks[0] != ocr.TaskExtractText {
		t.Errorf("SupportedTasks = %v, want [%v]", caps.SupportedTasks, ocr.TaskExtractText)
	}
	if caps.SupportsLayoutDetection {
		t.Error("SupportsLayoutDetection = true, want false")
	}
	if caps.SupportsTableExtraction {
		t.Error("SupportsTableExtraction = true, want false")
	}
}

func TestNoOpOCRProvider_ID(t *testing.T) {
	p := &ocr.NoOpOCRProvider{}
	if p.ID() != "noop" {
		t.Errorf("ID() = %q, want %q", p.ID(), "noop")
	}
}
