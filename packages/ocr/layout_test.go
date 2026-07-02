package ocr_test

import (
	"context"
	"testing"

	"github.com/YASSERRMD/verdex/packages/ocr"
)

func TestNoOpLayoutDetector_ReturnsNoRegions(t *testing.T) {
	d := ocr.NoOpLayoutDetector{}
	input := ocr.ImageInput{Data: []byte("synthetic page bytes")}

	regions, err := d.DetectLayout(context.Background(), input)
	if err != nil {
		t.Fatalf("DetectLayout() unexpected error: %v", err)
	}
	if len(regions) != 0 {
		t.Errorf("DetectLayout() returned %d regions, want 0", len(regions))
	}
}

func TestRegion_TypeEnumValues(t *testing.T) {
	tests := []struct {
		name string
		typ  ocr.RegionType
	}{
		{"paragraph", ocr.RegionTypeParagraph},
		{"heading", ocr.RegionTypeHeading},
		{"table", ocr.RegionTypeTable},
		{"figure", ocr.RegionTypeFigure},
	}

	seen := make(map[ocr.RegionType]bool)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.typ == "" {
				t.Errorf("RegionType %s must not be empty", tt.name)
			}
			if seen[tt.typ] {
				t.Errorf("RegionType %s duplicates another region type value %q", tt.name, tt.typ)
			}
			seen[tt.typ] = true
		})
	}
}

func TestRegion_BoundingBoxRoundTrip(t *testing.T) {
	r := ocr.Region{
		Page:        2,
		BoundingBox: ocr.BoundingBox{X: 10, Y: 20, Width: 300, Height: 40},
		Type:        ocr.RegionTypeHeading,
		Confidence:  0.87,
	}

	if r.Page != 2 {
		t.Errorf("Page = %d, want 2", r.Page)
	}
	if r.BoundingBox.X != 10 || r.BoundingBox.Y != 20 || r.BoundingBox.Width != 300 || r.BoundingBox.Height != 40 {
		t.Errorf("BoundingBox = %+v, want {10 20 300 40}", r.BoundingBox)
	}
	if r.Type != ocr.RegionTypeHeading {
		t.Errorf("Type = %q, want %q", r.Type, ocr.RegionTypeHeading)
	}
	if r.Confidence < 0 || r.Confidence > 1 {
		t.Errorf("Confidence = %v, want value in [0, 1]", r.Confidence)
	}
}
