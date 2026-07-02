package ocr

// ImageInput describes a raw scanned-document or image payload submitted for
// text extraction.
//
// Verdex never assumes a specific image codec or container: the pipeline
// treats the payload as an opaque byte slice accompanied by declared
// metadata. Real adapters are responsible for decoding whatever
// format/codec the bytes represent; the abstractions in this package
// operate purely on the metadata and byte length.
type ImageInput struct {
	// Data holds the raw image bytes (e.g. a scanned page, a photographed
	// document). It is mutated in place (zeroed) by Discard once extraction
	// has completed; callers must not retain external references to this
	// slice if they need it after discard.
	Data []byte

	// MIMEType is the declared MIME type of the payload (e.g. "image/png",
	// "image/jpeg", "image/tiff", "application/pdf").
	MIMEType string

	// WidthPx is the declared pixel width of the image. A zero value means
	// unknown/unspecified.
	WidthPx int

	// HeightPx is the declared pixel height of the image. A zero value
	// means unknown/unspecified.
	HeightPx int

	// PageNumber is the 1-based page number this image represents within a
	// larger multi-page document, when applicable. A zero value means
	// unknown/unspecified/single-page.
	PageNumber int

	// LanguageHint optionally biases extraction toward a specific
	// language/script. May be the zero value if no hint is available.
	LanguageHint LanguageHint
}

// TaskType classifies the kind of work an OCR provider call performs.
type TaskType string

const (
	// TaskExtractText is a standard text-extraction (OCR) task.
	TaskExtractText TaskType = "extract_text"
	// TaskDetectLayout is a layout/region-detection task.
	TaskDetectLayout TaskType = "detect_layout"
	// TaskExtractTable is a table-structure-extraction task.
	TaskExtractTable TaskType = "extract_table"
)

// Capability describes what a specific OCR provider/model combination can
// do.
type Capability struct {
	// SupportedTasks lists the TaskType values this provider handles.
	SupportedTasks []TaskType
	// MaxImageDimensionPx is the maximum single-request image dimension (the
	// larger of width/height) the provider accepts, in pixels. Zero means
	// unbounded/unspecified.
	MaxImageDimensionPx int
	// SupportsLayoutDetection indicates whether the provider can identify
	// document regions (paragraphs, tables, headings, figures) natively, as
	// opposed to relying on a separate LayoutDetector.
	SupportsLayoutDetection bool
	// SupportsTableExtraction indicates whether the provider can extract
	// structured table cell data natively.
	SupportsTableExtraction bool
	// SupportedLanguages lists ISO 639-1 language codes the provider can
	// extract text in. An empty slice means the provider is
	// language-agnostic or auto-detects.
	SupportedLanguages []string
	// ProviderID identifies the provider (e.g. "noop", "tesseract-local").
	ProviderID string
	// ModelID identifies the specific model, if applicable.
	ModelID string
}
