package intake

import (
	"time"

	"github.com/google/uuid"
)

// IntakeStatus represents the lifecycle state of an uploaded file within the
// intake pipeline.
type IntakeStatus string

const (
	// StatusReceiving indicates the file is currently being streamed into the
	// TempBuffer.
	StatusReceiving IntakeStatus = "receiving"

	// StatusHashing indicates the SHA-256 provenance hash is being computed.
	StatusHashing IntakeStatus = "hashing"

	// StatusScanning indicates the payload is being passed through the
	// VirusScanHook.
	StatusScanning IntakeStatus = "scanning"

	// StatusReady indicates all checks passed and the caller may read the
	// result; the binary will be discarded after the TTL.
	StatusReady IntakeStatus = "ready"

	// StatusDiscarded indicates the binary has been zeroed and removed from
	// temporary storage.
	StatusDiscarded IntakeStatus = "discarded"

	// StatusFailed indicates the intake pipeline encountered an unrecoverable
	// error; the binary (if any) has been discarded.
	StatusFailed IntakeStatus = "failed"
)

// IntakeRequest carries the metadata supplied by the caller when initiating an
// upload.  The binary payload is delivered separately via an io.Reader so that
// it is never fully buffered before validation.
type IntakeRequest struct {
	// TenantID identifies the tenant performing the upload.
	TenantID uuid.UUID

	// CaseID optionally associates the upload with a specific case.  When nil
	// the upload is treated as a tenant-level artifact.
	CaseID *uuid.UUID

	// UploaderID identifies the authenticated user initiating the upload.
	UploaderID uuid.UUID

	// Filename is the original filename as supplied by the client.
	Filename string

	// MIMEType is the MIME type declared by the client.  DetectMIME is called
	// on the first 512 bytes of the stream to verify this value.
	MIMEType string

	// SizeBytes is the expected payload size in bytes as declared by the
	// client.  The actual number of bytes read is also tracked.
	SizeBytes int64

	// TTL specifies how long the TempBuffer may exist before it must be
	// discarded.  A zero value falls back to a service-level default.
	TTL time.Duration
}

// IntakeResult is returned by IntakeService.Ingest and contains the provenance
// record for the uploaded file.
type IntakeResult struct {
	// IntakeID is a unique identifier for this intake operation.
	IntakeID uuid.UUID

	// ProvisionHash is the hex-encoded SHA-256 digest of the raw payload bytes.
	ProvisionHash string

	// MIMEType is the MIME type detected from the file content (may differ from
	// the client-declared value when sniffing overrides the declaration).
	MIMEType string

	// SizeBytes is the actual number of bytes received.
	SizeBytes int64

	// ReceivedAt is the wall-clock time at which the last byte was written to
	// the TempBuffer.
	ReceivedAt time.Time

	// DiscardedAt is set once the TempBuffer has been successfully discarded.
	DiscardedAt *time.Time

	// Status reflects the final state of the intake pipeline for this upload.
	Status IntakeStatus
}
