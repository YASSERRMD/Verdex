package precedent

import (
	"errors"
	"strings"
	"testing"
)

func TestExtractHoldingAndRatio_HeldAndRatioMarkers(t *testing.T) {
	text := `FACTS: The pursuer drank ginger beer containing a decomposed snail.
HELD: A manufacturer of products owes a duty of care to the ultimate
consumer of those products.
RATIO: The neighbour principle requires reasonable care to avoid acts or
omissions which one can reasonably foresee would injure persons closely
and directly affected.
DISPOSITION: Appeal allowed.`

	result, err := ExtractHoldingAndRatio(text)
	if err != nil {
		t.Fatalf("ExtractHoldingAndRatio() error = %v", err)
	}
	if !strings.Contains(result.Holding, "duty of care") {
		t.Errorf("Holding = %q, want it to contain %q", result.Holding, "duty of care")
	}
	if strings.Contains(result.Holding, "RATIO") || strings.Contains(result.Holding, "neighbour principle") {
		t.Errorf("Holding = %q, should not bleed into the RATIO section", result.Holding)
	}
	if !strings.Contains(result.RatioDecidendi, "neighbour principle") {
		t.Errorf("RatioDecidendi = %q, want it to contain %q", result.RatioDecidendi, "neighbour principle")
	}
	if strings.Contains(result.RatioDecidendi, "DISPOSITION") || strings.Contains(result.RatioDecidendi, "Appeal allowed") {
		t.Errorf("RatioDecidendi = %q, should not bleed into DISPOSITION", result.RatioDecidendi)
	}
}

func TestExtractHoldingAndRatio_HoldingMarkerVariant(t *testing.T) {
	text := `HOLDING: The defendant is liable in negligence.`
	result, err := ExtractHoldingAndRatio(text)
	if err != nil {
		t.Fatalf("ExtractHoldingAndRatio() error = %v", err)
	}
	if !strings.Contains(result.Holding, "liable in negligence") {
		t.Errorf("Holding = %q, want it to contain 'liable in negligence'", result.Holding)
	}
}

func TestExtractHoldingAndRatio_RatioFallback(t *testing.T) {
	// No explicit RATIO/REASONING marker: RatioDecidendi should fall back
	// to the text following the holding section.
	text := `HELD: The claim succeeds.
This is because the duty of care was clearly breached by the defendant's
conduct, and the resulting harm was reasonably foreseeable.`

	result, err := ExtractHoldingAndRatio(text)
	if err != nil {
		t.Fatalf("ExtractHoldingAndRatio() error = %v", err)
	}
	if result.Holding == "" {
		t.Error("Holding should not be empty")
	}
	if result.RatioDecidendi == "" {
		t.Error("RatioDecidendi should not be empty (fallback expected)")
	}
	if !strings.Contains(result.RatioDecidendi, "reasonably foreseeable") {
		t.Errorf("RatioDecidendi = %q, want it to contain fallback text", result.RatioDecidendi)
	}
}

func TestExtractHoldingAndRatio_NotFound(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"empty text", ""},
		{"whitespace only", "   \n\t "},
		{"no marker", "The court considered several arguments but reached no clear determination stated here."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ExtractHoldingAndRatio(tt.text)
			if err == nil {
				t.Fatal("ExtractHoldingAndRatio() error = nil, want error")
			}
			if !errors.Is(err, ErrHoldingNotFound) {
				t.Errorf("errors.Is(err, ErrHoldingNotFound) = false, err = %v", err)
			}
		})
	}
}

func TestExtractHoldingAndRatio_CaseInsensitiveMarker(t *testing.T) {
	text := `held: lowercase marker still recognized.`
	result, err := ExtractHoldingAndRatio(text)
	if err != nil {
		t.Fatalf("ExtractHoldingAndRatio() error = %v", err)
	}
	if !strings.Contains(result.Holding, "lowercase marker") {
		t.Errorf("Holding = %q, want it to contain 'lowercase marker'", result.Holding)
	}
}

func TestExtractorFunc_Pluggable(t *testing.T) {
	var custom ExtractorFunc = func(fullText string) (HoldingExtractionResult, error) {
		return HoldingExtractionResult{Holding: "custom holding", RatioDecidendi: "custom ratio"}, nil
	}
	result, err := custom("irrelevant text")
	if err != nil {
		t.Fatalf("custom extractor error = %v", err)
	}
	if result.Holding != "custom holding" {
		t.Errorf("Holding = %q, want custom holding", result.Holding)
	}
}
