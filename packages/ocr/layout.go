package ocr

import "context"

// BoundingBox is an axis-aligned pixel-coordinate box locating a region or
// text block on a page, with the origin (0,0) at the top-left corner.
type BoundingBox struct {
	// X is the horizontal offset of the box's top-left corner, in pixels.
	X int
	// Y is the vertical offset of the box's top-left corner, in pixels.
	Y int
	// Width is the box width, in pixels.
	Width int
	// Height is the box height, in pixels.
	Height int
}

// RegionType classifies the kind of content a detected Region contains.
type RegionType string

const (
	// RegionTypeParagraph is a block of body text.
	RegionTypeParagraph RegionType = "paragraph"
	// RegionTypeHeading is a title or section heading.
	RegionTypeHeading RegionType = "heading"
	// RegionTypeTable is a tabular grid of cells.
	RegionTypeTable RegionType = "table"
	// RegionTypeFigure is a non-text visual element (image, diagram, seal,
	// signature block).
	RegionTypeFigure RegionType = "figure"
)

// Region describes one detected layout region on a page: a classified,
// bounded area that downstream extraction (text or table) can be scoped to.
type Region struct {
	// Page is the 1-based page number this region was detected on.
	Page int

	// BoundingBox locates the region on the page, in pixel coordinates.
	BoundingBox BoundingBox

	// Type classifies the region's content.
	Type RegionType

	// Confidence is the detector's confidence in this region's classification
	// and boundaries, in the closed interval [0, 1].
	Confidence float64
}

// LayoutDetector identifies the layout regions present in an image.
//
// Layout detection may be performed natively by an OCRProvider (see
// Capability.SupportsLayoutDetection) or as a separate pre/post-processing
// step applied by OCRService. Implementations MUST be deterministic for a
// given input so tests can assert exact output.
type LayoutDetector interface {
	// DetectLayout returns the ordered list of Regions found in input. It
	// must not mutate input.Data.
	DetectLayout(ctx context.Context, input ImageInput) ([]Region, error)
}

// NoOpLayoutDetector is a LayoutDetector that returns no regions.
//
// Use this when no real layout-detection backend is configured; text
// extraction then proceeds without region scoping, and table extraction
// (which depends on RegionTypeTable regions) yields no tables. This is the
// default LayoutDetector for OCRService.
type NoOpLayoutDetector struct{}

// DetectLayout implements LayoutDetector. It is a deterministic no-op that
// always returns an empty, nil-error result.
func (NoOpLayoutDetector) DetectLayout(_ context.Context, _ ImageInput) ([]Region, error) {
	return nil, nil
}
