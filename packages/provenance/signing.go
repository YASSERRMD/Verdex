package provenance

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// Signer produces and verifies digital signatures over arbitrary byte slices.
type Signer interface {
	// Sign returns a hex-encoded signature of data.
	Sign(ctx context.Context, data []byte) (signature string, err error)

	// Verify returns true when signature is a valid signature of data.
	Verify(ctx context.Context, data []byte, signature string) (bool, error)
}

// HMACSigner implements Signer using HMAC-SHA256 with a symmetric key.
type HMACSigner struct {
	key []byte
}

// NewHMACSigner creates an HMACSigner using the provided key bytes.
func NewHMACSigner(key []byte) *HMACSigner {
	return &HMACSigner{key: key}
}

// Sign computes HMAC-SHA256(key, data) and returns it hex-encoded.
func (h *HMACSigner) Sign(_ context.Context, data []byte) (string, error) {
	mac := hmac.New(sha256.New, h.key)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// Verify recomputes the HMAC and does a constant-time comparison.
func (h *HMACSigner) Verify(_ context.Context, data []byte, signature string) (bool, error) {
	expected, err := h.Sign(context.Background(), data)
	if err != nil {
		return false, err
	}
	sigBytes, err := hex.DecodeString(signature)
	if err != nil {
		return false, fmt.Errorf("provenance: invalid signature encoding: %w", err)
	}
	expectedBytes, _ := hex.DecodeString(expected)
	return hmac.Equal(sigBytes, expectedBytes), nil
}

// canonicalPayload returns the deterministic JSON encoding of the record fields
// that are included in the signature. Signature and ChainHash are excluded so
// they can be computed after signing.
func canonicalPayload(r ProvenanceRecord) ([]byte, error) {
	type sigPayload struct {
		ID            string  `json:"id"`
		TenantID      string  `json:"tenant_id"`
		CaseID        *string `json:"case_id,omitempty"`
		UploaderID    string  `json:"uploader_id"`
		Filename      string  `json:"filename"`
		MIMEType      string  `json:"mime_type"`
		SizeBytes     int64   `json:"size_bytes"`
		ContentHash   string  `json:"content_hash"`
		HashAlgorithm string  `json:"hash_algorithm"`
		UploadedAt    string  `json:"uploaded_at"`
	}

	var caseIDStr *string
	if r.CaseID != nil {
		s := r.CaseID.String()
		caseIDStr = &s
	}

	p := sigPayload{
		ID:            r.ID.String(),
		TenantID:      r.TenantID.String(),
		CaseID:        caseIDStr,
		UploaderID:    r.UploaderID.String(),
		Filename:      r.Filename,
		MIMEType:      r.MIMEType,
		SizeBytes:     r.SizeBytes,
		ContentHash:   r.ContentHash,
		HashAlgorithm: r.HashAlgorithm,
		UploadedAt:    r.UploadedAt.UTC().Format("2006-01-02T15:04:05.999999999Z"),
	}

	return json.Marshal(p)
}

// SignRecord computes the canonical JSON of r (excluding Signature and
// ChainHash), signs it with s, and sets r.Signature.
func SignRecord(ctx context.Context, s Signer, r *ProvenanceRecord) error {
	payload, err := canonicalPayload(*r)
	if err != nil {
		return fmt.Errorf("provenance: marshal canonical payload: %w", err)
	}

	sig, err := s.Sign(ctx, payload)
	if err != nil {
		return fmt.Errorf("provenance: sign record: %w", err)
	}
	r.Signature = sig
	return nil
}

// VerifyRecord recomputes the canonical payload of r and verifies the stored
// Signature. Returns (false, nil) when the signature is wrong (not an error),
// and (false, err) only when a technical failure occurs.
func VerifyRecord(ctx context.Context, s Signer, r ProvenanceRecord) (bool, error) {
	payload, err := canonicalPayload(r)
	if err != nil {
		return false, fmt.Errorf("provenance: marshal canonical payload: %w", err)
	}
	return s.Verify(ctx, payload, r.Signature)
}
