# Verdex Chain-of-Custody Model

## Overview

Every artifact ingested by Verdex receives a **ProvenanceRecord** that
cryptographically binds the artifact's content to a tenant, case, and uploader
at the exact moment of upload. The record persists permanently — even after the
artifact itself is discarded — so courts and auditors can independently verify
the integrity of evidence.

---

## Core Data Structures

### ProvenanceRecord

| Field | Type | Description |
|---|---|---|
| `id` | UUID | Unique identifier of the provenance record |
| `tenant_id` | UUID | Tenant that owns the artifact |
| `case_id` | UUID? | Optional case the artifact belongs to |
| `uploader_id` | UUID | User who uploaded the artifact |
| `filename` | string | Original filename |
| `mime_type` | string | MIME type of the artifact |
| `size_bytes` | int64 | Byte length at upload time |
| `content_hash` | string | Hex-encoded SHA-256 of the artifact bytes |
| `hash_algorithm` | string | Always `"sha256"` |
| `uploaded_at` | time.Time | UTC timestamp of upload |
| `discarded_at` | time.Time? | Set when the artifact is discarded |
| `signature` | string | HMAC-SHA256 hex signature of the canonical payload |
| `chain_hash` | string | SHA-256 linking this record to the preceding one |

### CustodyEvent

| Field | Type | Description |
|---|---|---|
| `id` | UUID | Unique identifier |
| `provenance_id` | UUID | Linked ProvenanceRecord |
| `event_type` | string | One of: `uploaded`, `hashed`, `scanned`, `discarded`, `extracted`, `verified` |
| `actor` | string | Identity that triggered the event |
| `details` | string | Free-text context |
| `timestamp` | time.Time | UTC time of the event |
| `prev_event_hash` | string | EventHash of the preceding CustodyEvent |
| `event_hash` | string | SHA-256(prevEventHash + eventType + actor + timestamp) |

---

## Cryptographic Guarantees

### Content Integrity

The `content_hash` field stores the SHA-256 of the raw artifact bytes computed
at upload time. Downstream processors (virus scanners, text extractors) can
recompute this hash to verify they are working with the correct artifact.

### Record Signing

`SignRecord` serialises a deterministic ("canonical") JSON subset of the
ProvenanceRecord (excluding `signature` and `chain_hash`) and computes
HMAC-SHA256 over it using a server-held key. The hex-encoded MAC is stored in
`signature`. `VerifyRecord` recomputes the MAC and does a constant-time
comparison.

**Canonical payload fields (in JSON key order):**

```
id, tenant_id, case_id, uploader_id, filename, mime_type,
size_bytes, content_hash, hash_algorithm, uploaded_at
```

### Chain Hashing

`ChainBuilder.BuildChain` assigns a `chain_hash` to each record in sequence:

```
chain_hash[i] = SHA-256( chain_hash[i-1] + id[i] + content_hash[i] )
chain_hash[0] = SHA-256( "" + id[0] + content_hash[0] )
```

Any insertion, deletion, or reordering of records will break the chain, which
`VerifyChain` detects by recomputing all hashes and comparing them.

### Custody Event Linking

Each `CustodyEvent` carries the hash of the preceding event:

```
event_hash = SHA-256( prev_event_hash + event_type + actor + timestamp )
```

The first event in a chain uses `prev_event_hash = ""`. This structure means
an adversary cannot insert, remove, or reorder events without invalidating all
subsequent hashes.

---

## Lifecycle

```
Upload
  │
  ▼
NewProvenanceRecord()   ← assigns ID, timestamps, content_hash
  │
  ▼
SignRecord()            ← sets Signature
  │
  ▼
ProvenanceStore.Append()  ← append-only; digest snapshot taken here
  │
  ▼
CustodyChain.AddEvent("uploaded")
  │
  ├── [optional] AddEvent("hashed")
  ├── [optional] AddEvent("scanned")
  ├── [optional] AddEvent("extracted") + ExtractionLinkStore.LinkToExtraction()
  │
  ├── [optional] RecordDiscard()  ← sets DiscardedAt; adds "discarded" event
  │
  └── Verify() / VerifyChain()   ← read-only validation at any time
```

---

## Tamper Evidence

`InMemoryProvenanceStore` stores a SHA-256 digest snapshot of each record at
append time. `VerifyTamperEvidence` recomputes the digest of every stored
record and compares it to the snapshot. Any in-memory mutation (whether
accidental or adversarial) will cause the digest to differ and be reported as
a tamper event.

In a production deployment backed by PostgreSQL, the equivalent guarantee is
provided by:

1. A write-once audit table with `INSERT`-only permissions for the application
   role.
2. A trigger that rejects `UPDATE` and `DELETE` on provenance rows.
3. Periodic background verification that recomputes record signatures and chain
   hashes.

---

## ExtractionLink

When the text-extraction pipeline processes an artifact, it calls
`ExtractionLinkStore.LinkToExtraction(provenanceID, ref)` where `ref` is an
opaque pointer to the extracted text (e.g., an object-store key or database
row ID). This allows auditors to trace from the extracted text back to the
original artifact and verify that nothing was altered during extraction.

---

## API Endpoints

| Method | Path | Description |
|---|---|---|
| `POST` | `/provenance` | Create a provenance record for an uploaded artifact |
| `GET` | `/provenance/{id}` | Fetch a single record by ID |
| `GET` | `/provenance/{id}/verify` | Verify the record's HMAC signature |
| `GET` | `/provenance/case/{caseID}` | List all records for a case |

All responses use JSON. Errors are returned as `{"error": "..."}`.

---

## Security Considerations

- The HMAC key must be kept in a secrets manager (e.g., AWS Secrets Manager,
  HashiCorp Vault). Rotation requires re-signing existing records.
- The chain hash provides ordering integrity but not secrecy. Do not store
  sensitive data in `ContentHash` directly — use opaque identifiers.
- Custody events are advisory and are not signed individually; they derive
  their integrity from the chain-hash linkage.
- `DiscardedAt` is a logical flag only. Physical deletion of artifacts is
  governed by the data-retention policy and is orthogonal to provenance.
