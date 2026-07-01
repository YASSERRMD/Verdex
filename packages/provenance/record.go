package provenance

import (
	"time"

	"github.com/google/uuid"
)

// ProvenanceRecord is an immutable snapshot of an artifact at the time it was
// uploaded into a Verdex case. It captures cryptographic proof of the artifact's
// content and links it to a tenant, case, and uploader.
type ProvenanceRecord struct {
	// ID is the unique identifier of this provenance record.
	ID uuid.UUID `json:"id"`

	// TenantID scopes the record to a specific tenant.
	TenantID uuid.UUID `json:"tenant_id"`

	// CaseID is the optional case this artifact belongs to.
	CaseID *uuid.UUID `json:"case_id,omitempty"`

	// UploaderID identifies the user who uploaded the artifact.
	UploaderID uuid.UUID `json:"uploader_id"`

	// Filename is the original name of the uploaded file.
	Filename string `json:"filename"`

	// MIMEType is the detected or declared MIME type of the artifact.
	MIMEType string `json:"mime_type"`

	// SizeBytes is the byte-length of the artifact.
	SizeBytes int64 `json:"size_bytes"`

	// ContentHash is the hex-encoded cryptographic hash of the artifact's
	// raw bytes, computed using HashAlgorithm.
	ContentHash string `json:"content_hash"`

	// HashAlgorithm names the algorithm used for ContentHash (always "sha256").
	HashAlgorithm string `json:"hash_algorithm"`

	// UploadedAt is the UTC timestamp at which the artifact was received.
	UploadedAt time.Time `json:"uploaded_at"`

	// DiscardedAt, when non-nil, records when the artifact was discarded.
	// A discarded artifact may no longer be accessible, but its provenance
	// record is retained permanently.
	DiscardedAt *time.Time `json:"discarded_at,omitempty"`

	// Signature is the HMAC-SHA256 hex-encoded signature of the canonical JSON
	// of all fields except Signature and ChainHash.
	Signature string `json:"signature"`

	// ChainHash links this record to the previous one in the provenance chain.
	// It is the SHA-256 hex of (prevChainHash + currentID + currentContentHash).
	ChainHash string `json:"chain_hash"`
}

// NewProvenanceRecord constructs a new, unsigned ProvenanceRecord with a fresh
// UUID and the current UTC timestamp. Call signing.SignRecord to populate the
// Signature field before persisting.
func NewProvenanceRecord(
	tenantID uuid.UUID,
	caseID *uuid.UUID,
	uploaderID uuid.UUID,
	filename string,
	mimeType string,
	sizeBytes int64,
	contentHash string,
) ProvenanceRecord {
	return ProvenanceRecord{
		ID:            uuid.New(),
		TenantID:      tenantID,
		CaseID:        caseID,
		UploaderID:    uploaderID,
		Filename:      filename,
		MIMEType:      mimeType,
		SizeBytes:     sizeBytes,
		ContentHash:   contentHash,
		HashAlgorithm: "sha256",
		UploadedAt:    time.Now().UTC(),
	}
}
