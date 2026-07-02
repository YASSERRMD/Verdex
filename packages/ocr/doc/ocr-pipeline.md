# Verdex OCR & Document Extraction Pipeline

## Overview

`packages/ocr` extracts text from scanned documents and images (filed
exhibits, scanned judgments, photographed evidence) into ordered,
confidence-scored text for the Verdex judicial reasoning platform. Like the
LLM provider abstraction in `packages/provider` and the speech-to-text
pipeline in `packages/stt`, this package never hardcodes an OCR backend: all
extraction is routed through the `OCRProvider` interface so that concrete
adapters (local OCR engines, hosted OCR APIs, or the deterministic
`NoOpOCRProvider` used for tests and air-gapped deployments) can be
registered and swapped without touching business logic.

Source images are never retained beyond the extraction call. Verdex's
transcribe-and-discard guarantee (the same guarantee `packages/intake`
provides for uploaded files and `packages/stt` provides for audio) means the
raw bytes are zeroed immediately after an `ExtractionResult` is produced,
with a SHA-256 provenance hash captured beforehand so downstream systems can
still verify what was processed without ever storing the image itself.

---

## Model-Agnostic Design

### OCRProvider

```go
type OCRProvider interface {
    ID() string
    Capabilities() Capability
    Extract(ctx context.Context, input ImageInput) (*ExtractionResult, error)
}
```

Every adapter — whether it wraps a local engine, a hosted API, or a
deterministic stub — implements this single interface. `Capabilities()`
advertises what the provider supports (supported tasks, max image dimension,
native layout/table detection, supported languages) so callers can select an
appropriate provider without hardcoding assumptions about a specific vendor.

### Registry

`Registry` is a thread-safe map from provider ID to `OCRProvider`, mirroring
`provider.Registry` and `stt.Registry`. `DefaultRegistry` is the
process-wide registry; application startup code registers concrete adapters
against it (or against an isolated `*Registry` for tests), and the rest of
the codebase resolves providers by ID through `OCRService`.

### NoOpOCRProvider

`NoOpOCRProvider` is a deterministic stub: it never inspects the image bytes
and derives its (fixed) output purely from declared metadata (page number,
width/height). It exists for unit tests, CI, and air-gapped deployments
where no real OCR backend is available. Constraint: no code in this package
calls a real external OCR API or SDK — that is left entirely to adapters
implemented outside this package.

---

## Pipeline Stages

`OCRService.Extract` orchestrates the full pipeline:

```
ImageInput
   │
   ▼
Preprocess()            ← deterministic metadata-level deskew/denoise tracking
   │
   ▼
ComputeSourceHash()      ← SHA-256 over the raw bytes, captured before any mutation
   │
   ▼
provider.Extract()       ← produces ordered, confidence-scored TextBlocks
   │
   ▼
LayoutDetector.DetectLayout() ← identifies Regions (paragraph/heading/table/figure)
   │
   ▼
ExtractTablesFromRegions()    ← runs TableExtractor over RegionTypeTable regions
   │
   ▼
Discard()                ← zeroes ImageInput.Data, emits DiscardAuditEvent
   │
   ▼
*ExtractionResult          (SourceHash populated, Regions/Tables attached, source bytes gone)
```

Discard runs unconditionally once the provider call has returned — even on
extraction failure — so source images never outlive an attempt.

### Pre-processing

`Preprocess` operates purely on `ImageInput`'s declared metadata and byte
length, exactly as `packages/stt`'s `Normalize`/`Segment` do for audio. No
real image codec is implemented or required: deskew is tracked when the
caller-supplied `skewAngleDeg` exceeds `DefaultDeskewThresholdDeg`, recording
the corrective rotation angle; denoise is tracked whenever the payload is at
least `DefaultDenoiseMinBytes` long. `PreprocessResult` records which steps
were applied (`Deskewed`, `Denoised`, `RotationCorrectionDeg`) so downstream
code and tests can assert on pipeline behaviour without a real image
decoder.

### Layout and Table Extraction

`LayoutDetector` identifies document `Region`s — bounded, classified areas
(`RegionTypeParagraph`, `RegionTypeHeading`, `RegionTypeTable`,
`RegionTypeFigure`). `NoOpLayoutDetector` is the deterministic default: it
returns no regions, so text extraction proceeds unscoped.

`TableExtractor` is a separate, narrower hook: given a `Region` already
classified as `RegionTypeTable`, it segments that region into a structured
`Table` of rows/columns of cell text. Separating layout detection from table
structure extraction mirrors how `packages/stt` separates transcription from
diarization — two independently pluggable concerns composed by the service.
`NoOpTableExtractor` is the deterministic default: it returns an empty,
zero-dimension `Table` for the region's bounding box.

### Multi-Language and Multi-Script Support

`LanguageHint` is a plain ISO 639-1 string wrapper, mirroring
`stt.LanguageHint`. `LanguageSet` is a minimal, dependency-free mirror of the
language data a `jurisdiction.Jurisdiction` carries (`Languages []string`),
so this package has **no hard module dependency on `packages/jurisdiction`**.

OCR has an additional axis STT does not: writing *script*, independent of
language. `Script` enumerates the scripts Verdex jurisdictions are known to
require (`ScriptLatin`, `ScriptArabic`, `ScriptTamil`, `ScriptUrdu`).
`MultiScriptSupport` declares which scripts a single document set may
contain simultaneously (e.g. an Arabic-script judgment citing an
English-script statute), and `DeriveMultiScriptSupport` derives it from a
jurisdiction's language codes via a small built-in `code -> Script` lookup.

### Confidence and Source-Span Capture

`TextBlock` carries `Confidence` (a float in `[0, 1]`) and a source-span
reference: `Page` plus `BoundingBox` (pixel-coordinate `X`/`Y`/`Width`/
`Height`). `ExtractionResult.SortBlocks` guarantees the final `Blocks` slice
is ordered by page ascending, then reading order (top-to-bottom,
left-to-right) within a page. Helpers in `extraction.go`
(`AverageConfidence`, `LowConfidenceBlocks`) let callers aggregate or flag
low-confidence spans for human review, mirroring `packages/stt`'s
`confidence.go`.

---

## Transcribe-and-Discard Guarantee

1. `ComputeSourceHash` computes the SHA-256 digest of `ImageInput.Data`
   **before** any provider call or mutation.
2. The provider extracts text from the image.
3. `Discard` zeroes every byte of `ImageInput.Data` in place and truncates
   the slice, rendering the source image unrecoverable from the struct.
4. A `DiscardAuditEvent` (event type `ocr.discarded`) carrying the
   pre-computed `SourceHash`, byte count, provider ID, and timestamp is
   emitted to the configured `ImageDiscardSink`.
5. The returned `*ExtractionResult.SourceHash` is set to the same digest, so
   provenance can be verified without ever retaining the image bytes.

This mirrors the guarantee `packages/intake` provides for uploaded binary
artifacts and `packages/stt` provides for audio: hash first, discard
immediately after use, and always emit an audit trail — regardless of
whether the operation that consumed the bytes succeeded or failed.

---

## Testing Strategy

- `registry_test.go` — register/get/list/duplicate/not-found/nil-provider
  behaviour, mirroring `packages/stt`'s and `packages/provider`'s registry
  tests.
- `layout_test.go` — `NoOpLayoutDetector` returns no regions; `Region`
  bounding-box and classification invariants.
- `table_test.go` — table rows/cols consistency (`TableIsConsistent`),
  `NoOpTableExtractor` behaviour, and `ExtractTablesFromRegions` scoping to
  `RegionTypeTable` regions only.
- `service_test.go` — end-to-end pipeline: blocks ordered, confidence in
  `[0, 1]`, error wrapping for missing providers, empty images, and nil
  input.
- `discard_test.go` — source bytes are actually zeroed, discard is
  idempotent, an audit event is emitted with the pre-computed hash, and the
  end-to-end service call discards the caller's `ImageInput`.

No test performs a live network call or decodes a real image; all providers
used in tests are the deterministic `NoOpOCRProvider`, and all test payloads
are synthetic byte slices — no binary/image files are committed to the
repository.
