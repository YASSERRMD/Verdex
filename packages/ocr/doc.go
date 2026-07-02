// Package ocr implements the model-agnostic optical-character-recognition
// (OCR) and document-extraction pipeline used throughout the Verdex judicial
// reasoning platform.
//
// All text extraction from scanned documents and images inside Verdex is
// routed through the OCRProvider interface defined in this package so that
// concrete adapters (local OCR engines, hosted OCR APIs, air-gapped no-op
// stubs, etc.) can be registered and swapped without touching business
// logic. This mirrors the provider.LLMProvider abstraction in
// packages/provider and the STTProvider abstraction in packages/stt.
//
// Core concepts:
//
//   - OCRProvider: the interface every adapter must implement.
//   - Registry: a process-level map from provider IDs to OCRProvider
//     instances.
//   - ImageInput: a provider-neutral description of a raw image/scanned-page
//     payload (bytes plus declared metadata); no specific codec is assumed.
//   - Preprocess: deterministic, metadata-level deskew/denoise tracking,
//     operating purely on declared metadata and byte length.
//   - LayoutDetector: an interface for identifying document Regions
//     (paragraph, heading, table, figure), with NoOpLayoutDetector as the
//     deterministic default.
//   - TableExtractor: an interface for extracting structured Table data from
//     regions classified as tables, with NoOpTableExtractor as the
//     deterministic default.
//   - LanguageHint / Script / MultiScriptSupport: multi-language and
//     multi-script (Latin/Arabic/Tamil/Urdu) hinting and detection support.
//   - ExtractionResult / TextBlock: the ordered, confidence-scored,
//     source-span-annotated output of extraction.
//   - OCRService: orchestrates preprocess -> provider.Extract ->
//     layout/table extraction -> discard source -> return
//     *ExtractionResult.
//   - Discard / DiscardAuditEvent: the transcribe-and-discard guarantee —
//     source image bytes are zeroed immediately after extraction completes,
//     with the SHA-256 provenance hash captured beforehand and a discard
//     audit event emitted, mirroring packages/stt's and packages/intake's
//     discard guarantees for other binary ingestion artifacts.
//   - NoOpOCRProvider: a deterministic stub useful in tests, CI, and
//     air-gapped deployments.
//
// See doc/ocr-pipeline.md for a detailed design write-up.
package ocr
