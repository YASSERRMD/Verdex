// Package provenance implements provenance tracking and chain-of-custody for
// Verdex judicial artifacts.
//
// Every artifact uploaded into a Verdex case receives a ProvenanceRecord that
// captures its cryptographic fingerprint (SHA-256 content hash), a digital
// signature (HMAC-SHA256 over canonical JSON of the record), and a chain hash
// linking it to the previous record so that any tampering or gap is immediately
// detectable.
//
// # Core concepts
//
//   - ProvenanceRecord: immutable snapshot of an artifact at upload time.
//   - CustodyEvent:     ordered event in the artifact's chain of custody
//     (uploaded, hashed, scanned, discarded, …).
//   - Signing:          HMAC-SHA256 signatures make each record self-verifying.
//   - ChainBuilder:     links records into an ordered, tamper-evident chain.
//   - InMemoryProvenanceStore: append-only store with tamper-evidence detection.
//   - ProvenanceService: orchestrates creation, discarding and verification.
//   - ExtractionLink:   ties a provenance record to the location of its
//     extracted text representation.
//
// The package is intentionally free of external persistence dependencies so
// that it can be embedded in any Verdex service. Swap InMemoryProvenanceStore
// for a PostgreSQL-backed implementation when persistence is required.
package provenance
