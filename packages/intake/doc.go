// Package intake implements the upload intake service for the Verdex judicial
// reasoning platform.
//
// The intake service accepts files, audio, and video uploads from authenticated
// tenants, computes a provenance hash over the raw bytes, validates MIME types
// and file sizes, optionally virus-scans the payload, emits a structured audit
// event, and then discards the binary from temporary storage after a
// configurable TTL.
//
// Design invariants enforced by this package:
//
//   - No binary payload is persisted beyond its TTL.  The TempBuffer is zeroed
//     and deleted by Discard(); the IntakeService schedules that call
//     automatically.
//   - The provenance hash (SHA-256) is computed in a single streaming pass;
//     the full file is never held entirely in memory.
//   - Every intake operation produces an IntakeAuditEvent regardless of
//     success or failure, allowing downstream forensic reconstruction.
//   - Quota limits (file size, daily uploads, concurrent uploads) are checked
//     before any bytes are written to disk.
//
// Typical call sequence:
//
//	svc := intake.NewIntakeService(scanner, quota, auditSink, 5*time.Minute)
//	result, err := svc.Ingest(ctx, req, body)
package intake
