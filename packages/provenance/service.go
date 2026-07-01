package provenance

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ProvenanceService orchestrates the creation, signing, discarding, and
// verification of provenance records, and exposes the chain-of-custody API.
type ProvenanceService struct {
	store  *InMemoryProvenanceStore
	signer Signer

	// custodyChains maps ProvenanceRecord.ID → *CustodyChain.
	custodyChains map[uuid.UUID]*CustodyChain
}

// NewProvenanceService creates a ProvenanceService backed by the given store
// and signer. If store is nil, a new InMemoryProvenanceStore is allocated.
func NewProvenanceService(store *InMemoryProvenanceStore, signer Signer) *ProvenanceService {
	if store == nil {
		store = NewInMemoryProvenanceStore()
	}
	return &ProvenanceService{
		store:         store,
		signer:        signer,
		custodyChains: make(map[uuid.UUID]*CustodyChain),
	}
}

// CreateRecord creates a new ProvenanceRecord for an uploaded artifact, signs
// it, and appends it to the store. It also starts a custody chain for the
// record with an "uploaded" event.
func (svc *ProvenanceService) CreateRecord(
	ctx context.Context,
	tenantID uuid.UUID,
	caseID *uuid.UUID,
	uploaderID uuid.UUID,
	filename string,
	mimeType string,
	sizeBytes int64,
	contentHash string,
) (*ProvenanceRecord, error) {
	r := NewProvenanceRecord(tenantID, caseID, uploaderID, filename, mimeType, sizeBytes, contentHash)

	if err := SignRecord(ctx, svc.signer, &r); err != nil {
		return nil, fmt.Errorf("provenance service: sign: %w", err)
	}

	if err := svc.store.Append(ctx, r); err != nil {
		return nil, fmt.Errorf("provenance service: append: %w", err)
	}

	// Start a custody chain for this record.
	chain := NewCustodyChain()
	event := CustodyEvent{
		ID:           uuid.New(),
		ProvenanceID: r.ID,
		EventType:    EventUploaded,
		Actor:        uploaderID.String(),
		Details:      fmt.Sprintf("file=%s size=%d", filename, sizeBytes),
		Timestamp:    time.Now().UTC(),
	}
	if err := chain.AddEvent(event); err != nil {
		return nil, fmt.Errorf("provenance service: add custody event: %w", err)
	}
	svc.custodyChains[r.ID] = chain

	cp := r
	return &cp, nil
}

// RecordDiscard marks a record as discarded by setting its DiscardedAt field.
// Returns ErrProvenanceNotFound if the record does not exist, and
// ErrAlreadyDiscarded if it has already been discarded.
func (svc *ProvenanceService) RecordDiscard(ctx context.Context, id uuid.UUID, discardedAt time.Time) error {
	existing, err := svc.store.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("provenance service: get by id: %w", err)
	}
	if existing.DiscardedAt != nil {
		return fmt.Errorf("provenance service: %w", ErrAlreadyDiscarded)
	}

	existing.DiscardedAt = &discardedAt

	// Re-sign to include the DiscardedAt context (via canonicalPayload which
	// does not include DiscardedAt, but we update the store digest).
	if err := svc.store.UpdateRecord(ctx, *existing); err != nil {
		return fmt.Errorf("provenance service: update record: %w", err)
	}

	// Add a custody event.
	if chain, ok := svc.custodyChains[id]; ok {
		event := CustodyEvent{
			ID:           uuid.New(),
			ProvenanceID: id,
			EventType:    EventDiscarded,
			Actor:        "system",
			Details:      fmt.Sprintf("discarded_at=%s", discardedAt.UTC().Format(time.RFC3339)),
			Timestamp:    time.Now().UTC(),
		}
		_ = chain.AddEvent(event)
	}

	return nil
}

// Verify checks the cryptographic signature of a stored record. It returns
// (true, "valid", nil) when the signature is intact.
func (svc *ProvenanceService) Verify(ctx context.Context, id uuid.UUID) (valid bool, reason string, err error) {
	r, err := svc.store.GetByID(ctx, id)
	if err != nil {
		return false, "", fmt.Errorf("provenance service: get by id: %w", err)
	}

	ok, err := VerifyRecord(ctx, svc.signer, *r)
	if err != nil {
		return false, "", fmt.Errorf("provenance service: verify record: %w", err)
	}
	if !ok {
		return false, "signature mismatch", nil
	}
	return true, "valid", nil
}

// GetCustodyChain returns the ordered list of custody events for all records in
// a case (in record insertion order).
func (svc *ProvenanceService) GetCustodyChain(ctx context.Context, caseID uuid.UUID) ([]CustodyEvent, error) {
	records, err := svc.store.GetByCase(ctx, caseID)
	if err != nil {
		return nil, fmt.Errorf("provenance service: get by case: %w", err)
	}

	var all []CustodyEvent
	for _, r := range records {
		if chain, ok := svc.custodyChains[r.ID]; ok {
			all = append(all, chain.GetChain()...)
		}
	}
	return all, nil
}

// VerifyTamperEvidence delegates to the underlying store's tamper-evidence check.
func (svc *ProvenanceService) VerifyTamperEvidence(ctx context.Context, caseID uuid.UUID) (bool, []string, error) {
	return svc.store.VerifyTamperEvidence(ctx, caseID)
}
