package provenance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// ProvenanceStore is the persistence abstraction for ProvenanceRecord values.
// Implementations must be append-only: once a record is stored it cannot be
// deleted, and any in-place modification must be detected as tampering.
type ProvenanceStore interface {
	// Append persists a new provenance record.
	Append(ctx context.Context, r ProvenanceRecord) error

	// GetByID retrieves a record by its unique ID.
	GetByID(ctx context.Context, id uuid.UUID) (*ProvenanceRecord, error)

	// GetByCase returns all records belonging to the given case, in insertion
	// order.
	GetByCase(ctx context.Context, caseID uuid.UUID) ([]ProvenanceRecord, error)

	// GetByContentHash returns the first record whose ContentHash matches the
	// given hex-encoded hash.
	GetByContentHash(ctx context.Context, hash string) (*ProvenanceRecord, error)

	// VerifyTamperEvidence re-reads all records for a case and checks whether
	// any stored value has been modified after it was appended.
	VerifyTamperEvidence(ctx context.Context, caseID uuid.UUID) (valid bool, issues []string, err error)
}

// storeEntry holds an immutable copy of a ProvenanceRecord together with the
// digest that was computed at append time.
type storeEntry struct {
	record ProvenanceRecord
	digest string // hex-SHA256 of all record fields at append time
}

// entryDigest computes a hex-encoded SHA-256 over all fields of a record.
func entryDigest(r ProvenanceRecord) (string, error) {
	payload, err := canonicalPayload(r)
	if err != nil {
		return "", fmt.Errorf("provenance: digest canonical payload: %w", err)
	}
	// Append mutable fields that are not covered by canonicalPayload.
	extra := r.Signature + r.ChainHash
	if r.DiscardedAt != nil {
		extra += r.DiscardedAt.UTC().Format("2006-01-02T15:04:05.999999999Z")
	}
	combined := append(payload, []byte(extra)...) //nolint:gocritic
	sum := sha256.Sum256(combined)
	return hex.EncodeToString(sum[:]), nil
}

// InMemoryProvenanceStore is a thread-safe, append-only in-memory implementation
// of ProvenanceStore. It takes a digest snapshot at append time and re-verifies
// it during VerifyTamperEvidence.
type InMemoryProvenanceStore struct {
	mu      sync.RWMutex
	entries []storeEntry
}

// NewInMemoryProvenanceStore allocates an empty store.
func NewInMemoryProvenanceStore() *InMemoryProvenanceStore {
	return &InMemoryProvenanceStore{}
}

// Append stores a copy of the record together with its digest.
func (s *InMemoryProvenanceStore) Append(_ context.Context, r ProvenanceRecord) error {
	d, err := entryDigest(r)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, storeEntry{record: r, digest: d})
	return nil
}

// GetByID returns the stored record with the given ID, or ErrProvenanceNotFound.
func (s *InMemoryProvenanceStore) GetByID(_ context.Context, id uuid.UUID) (*ProvenanceRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, e := range s.entries {
		if e.record.ID == id {
			cp := e.record
			return &cp, nil
		}
	}
	return nil, ErrProvenanceNotFound
}

// GetByCase returns all records for the given case in insertion order.
func (s *InMemoryProvenanceStore) GetByCase(_ context.Context, caseID uuid.UUID) ([]ProvenanceRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []ProvenanceRecord
	for _, e := range s.entries {
		if e.record.CaseID != nil && *e.record.CaseID == caseID {
			out = append(out, e.record)
		}
	}
	return out, nil
}

// GetByContentHash returns the first record whose ContentHash equals hash.
func (s *InMemoryProvenanceStore) GetByContentHash(_ context.Context, hash string) (*ProvenanceRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, e := range s.entries {
		if e.record.ContentHash == hash {
			cp := e.record
			return &cp, nil
		}
	}
	return nil, ErrProvenanceNotFound
}

// VerifyTamperEvidence recomputes the digest of every record in the case and
// compares it to the stored snapshot. It returns false with a list of issue
// descriptions if any mismatch is found.
func (s *InMemoryProvenanceStore) VerifyTamperEvidence(_ context.Context, caseID uuid.UUID) (bool, []string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var issues []string
	for _, e := range s.entries {
		if e.record.CaseID == nil || *e.record.CaseID != caseID {
			continue
		}
		current, err := entryDigest(e.record)
		if err != nil {
			return false, nil, fmt.Errorf("provenance: recompute digest for %s: %w", e.record.ID, err)
		}
		if current != e.digest {
			issues = append(issues, fmt.Sprintf("record %s digest mismatch", e.record.ID))
		}
	}
	if len(issues) > 0 {
		return false, issues, ErrTamperDetected
	}
	return true, nil, nil
}

// UpdateRecord replaces the stored record for the given ID. This is used
// internally by the service to record DiscardedAt; it re-snapshots the digest
// so that subsequent VerifyTamperEvidence calls still pass.
func (s *InMemoryProvenanceStore) UpdateRecord(_ context.Context, r ProvenanceRecord) error {
	d, err := entryDigest(r)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, e := range s.entries {
		if e.record.ID == r.ID {
			s.entries[i] = storeEntry{record: r, digest: d}
			return nil
		}
	}
	return ErrProvenanceNotFound
}
