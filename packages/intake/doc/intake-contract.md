# Intake API Contract

## Overview

The intake service accepts file, audio, and video uploads from authenticated
tenants, computes a SHA-256 provenance hash, enforces strict MIME-type and
quota policies, optionally virus-scans the payload, emits structured audit
events, and discards the binary after a configurable TTL.

No binary payload persists beyond the TTL. Downstream services must extract
all required content before the TTL elapses.

---

## Endpoints

### POST /intake/upload

Accepts a `multipart/form-data` body containing the file to upload.

**Request**

| Field       | Type   | Required | Description                                              |
|-------------|--------|----------|----------------------------------------------------------|
| `file`      | file   | yes      | Binary payload. Max 100 MB per request.                  |
| `case_id`   | string | no       | UUID of the case to associate with the upload.           |
| `mime_type` | string | no       | Client-declared MIME type. Overrides the part header.    |

**Headers**

| Header          | Value                          |
|-----------------|-------------------------------|
| `Content-Type`  | `multipart/form-data; boundary=...` |

The tenant identity is resolved from the request context (populated by the
tenancy middleware). An unauthenticated request returns `401 Unauthorized`.

**Successful response — 202 Accepted**

```json
{
  "IntakeID":      "550e8400-e29b-41d4-a716-446655440000",
  "ProvisionHash": "2cf24dba5fb0a30e...",
  "MIMEType":      "application/pdf",
  "SizeBytes":     204800,
  "ReceivedAt":    "2026-07-01T12:00:00Z",
  "DiscardedAt":   null,
  "Status":        "ready"
}
```

**Error responses**

| Status | Condition                                           |
|--------|-----------------------------------------------------|
| 400    | Malformed multipart body or invalid `case_id`.      |
| 401    | No tenant in request context.                       |
| 413    | Body exceeds 100 MB.                                |
| 415    | MIME type not in the allowlist or content mismatch. |
| 429    | Quota exceeded (size, daily uploads, concurrent).   |
| 500    | Internal error (scan infrastructure, disk, etc.).   |

---

### GET /intake/{id}/status

Polls the current status of an intake operation.

> **Note:** The intake service does not persist results after the binary is
> discarded. Callers should use the `IntakeResult` returned from
> `POST /intake/upload` directly. This endpoint returns `404 Not Found` once
> the buffer has been discarded or was never registered.

**Path parameters**

| Parameter | Type   | Description              |
|-----------|--------|--------------------------|
| `id`      | string | UUID of the intake operation. |

**Successful response — 200 OK** (while buffer is live)

```json
{
  "IntakeID":      "550e8400-e29b-41d4-a716-446655440000",
  "Status":        "ready"
}
```

**Error responses**

| Status | Condition                              |
|--------|----------------------------------------|
| 400    | `id` is not a valid UUID.              |
| 401    | No tenant in request context.          |
| 404    | Intake not found or already discarded. |

---

## Allowed MIME Types

| Category  | MIME Type                                                                  |
|-----------|----------------------------------------------------------------------------|
| Document  | `application/pdf`                                                          |
| Document  | `application/vnd.openxmlformats-officedocument.wordprocessingml.document` |
| Document  | `text/plain`                                                               |
| Audio     | `audio/mpeg`, `audio/wav`, `audio/ogg`                                     |
| Video     | `video/mp4`                                                                |
| Image     | `image/png`, `image/jpeg`, `image/tiff`, `image/webp`                      |

Any other MIME type is rejected with `415 Unsupported Media Type`.

MIME detection uses the first 512 bytes of the payload (identical to browser
content sniffing per WHATWG). If the detected type differs from the declared
type and the detected type is not in the allowlist, the upload is rejected.

---

## Provenance Hash

The `ProvisionHash` field in `IntakeResult` is the lowercase hex-encoded
SHA-256 digest of the raw payload bytes, computed in a single streaming pass
without buffering the entire file in memory.

The hash is computed over the original bytes as received, before any
transcoding or extraction. It uniquely identifies the binary and can be used
for:

- Deduplication across tenants (if cross-tenant comparison is authorised).
- Forensic reconstruction: "did this exact file enter the system?"
- Chain-of-custody documentation for judicial proceedings.

---

## TTL and Discard Guarantee

Every upload is written to an OS temporary file (the `TempBuffer`). The buffer:

1. Is created with a TTL (default 5 minutes, configurable per request).
2. Is automatically zeroed and deleted when the TTL elapses, even if the
   caller never explicitly calls `DiscardAll`.
3. Emits an `intake.discarded` audit event when discarded.

**No binary payload persists beyond the TTL.** Downstream extraction
(transcription, OCR, embedding) must complete within the TTL window, or the
consumer must copy the bytes to its own durable store before discard.

---

## Quota

Three independent quota dimensions are enforced before any bytes are written:

| Dimension                | Config field                   | Default  |
|--------------------------|-------------------------------|----------|
| Maximum file size        | `MaxFileSizeMB`               | 0 (off)  |
| Daily uploads per tenant | `MaxDailyUploadsPerTenant`    | 0 (off)  |
| Concurrent uploads       | `MaxConcurrentPerTenant`      | 0 (off)  |

A value of `0` disables the corresponding check. All three checks run before
the TempBuffer is created.

---

## Audit Events

Every intake operation emits a sequence of `IntakeAuditEvent` records to the
configured `AuditSink`:

| Event type         | When emitted                                     |
|--------------------|--------------------------------------------------|
| `intake.started`   | Immediately on `Ingest` entry, before any checks.|
| `intake.hashing`   | After quota checks, before streaming to buffer.  |
| `intake.scanning`  | After hashing, before virus scan.                |
| `intake.ready`     | After a successful scan.                         |
| `intake.failed`    | When any step fails.                             |
| `intake.discarded` | When the TempBuffer is zeroed and removed.       |

Events are emitted on a best-effort basis; a sink error does not abort the
pipeline.

---

## Virus Scanning

The `VirusScanHook` interface allows plugging in any AV backend:

```go
type VirusScanHook interface {
    Scan(ctx context.Context, r io.Reader, filename string) (clean bool, details string, err error)
}
```

The `NoOpVirusScanHook` (always clean) is provided for development and test
environments. The `LoggingVirusScanHook` wraps any hook and logs every scan
invocation.

A scan error (non-nil `err`) causes the upload to fail with a 500 error and
the buffer to be immediately discarded.

---

## Security Considerations

- **No binary persistence.** The TempBuffer is the only location where the
  payload exists; it is zeroed before deletion.
- **Streaming hashing.** The full file is never held in memory; the SHA-256
  state is updated in 32 KB chunks via `StreamingHashReader`.
- **MIME sniffing.** The declared MIME type is verified against the detected
  type to prevent extension spoofing.
- **Quota guards.** All three quota dimensions are checked atomically under a
  mutex before the buffer is allocated.
- **Audit trail.** `intake.started` is emitted before any validation so the
  audit trail begins even for requests that are immediately rejected.
