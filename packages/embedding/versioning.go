package embedding

import (
	"context"
	"sync"
	"time"
)

// EmbeddingVersion captures the full identity of an embedding schema.  When
// any field changes, existing embeddings may no longer be comparable to newly
// computed ones and should be invalidated.
type EmbeddingVersion struct {
	// ModelID is the embedding model identifier (e.g. "text-embedding-3-small").
	ModelID string

	// ProviderID identifies the LLM provider (e.g. "openai").
	ProviderID string

	// Dimensions is the length of embedding vectors produced by this model.
	Dimensions int

	// Version is the application-level schema version, incremented whenever
	// the combination of (ModelID, ProviderID, Dimensions) changes.
	Version int

	// CreatedAt is when this version entry was first recorded.
	CreatedAt time.Time
}

// VersionRegistry tracks the current [EmbeddingVersion] and records
// historical versions so callers can detect when re-embedding is required.
//
// Implementations MUST be safe for concurrent use.
type VersionRegistry interface {
	// CurrentVersion returns the most recently recorded version, or nil if
	// no version has been recorded yet.
	CurrentVersion(ctx context.Context) (*EmbeddingVersion, error)

	// RecordVersion stores v as the new current version.  If the incoming
	// version differs from the previously stored one (by ModelID, ProviderID,
	// or Dimensions), Version is incremented automatically.
	RecordVersion(ctx context.Context, v EmbeddingVersion) error
}

// InMemoryVersionRegistry is a simple, non-persistent [VersionRegistry]
// suitable for testing and single-binary deployments.
type InMemoryVersionRegistry struct {
	mu      sync.RWMutex
	current *EmbeddingVersion
	history []EmbeddingVersion
}

// NewInMemoryVersionRegistry returns a ready-to-use [InMemoryVersionRegistry].
func NewInMemoryVersionRegistry() *InMemoryVersionRegistry {
	return &InMemoryVersionRegistry{}
}

// CurrentVersion implements [VersionRegistry].
func (r *InMemoryVersionRegistry) CurrentVersion(_ context.Context) (*EmbeddingVersion, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.current == nil {
		return nil, nil
	}
	copy := *r.current
	return &copy, nil
}

// RecordVersion implements [VersionRegistry].
//
// Re-embedding is triggered when the new version's (ModelID, ProviderID,
// Dimensions) triple differs from the stored current version.  In that case
// the Version counter is incremented.
func (r *InMemoryVersionRegistry) RecordVersion(_ context.Context, v EmbeddingVersion) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.current == nil {
		// First registration.
		v.Version = 1
		if v.CreatedAt.IsZero() {
			v.CreatedAt = time.Now().UTC()
		}
		cp := v
		r.current = &cp
		r.history = append(r.history, cp)
		return nil
	}

	// Check whether the schema identity has changed.
	schemaChanged := r.current.ModelID != v.ModelID ||
		r.current.ProviderID != v.ProviderID ||
		r.current.Dimensions != v.Dimensions

	if schemaChanged {
		v.Version = r.current.Version + 1
	} else {
		v.Version = r.current.Version
	}
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now().UTC()
	}

	cp := v
	r.current = &cp
	r.history = append(r.history, cp)
	return nil
}

// History returns a snapshot of all recorded versions in registration order.
func (r *InMemoryVersionRegistry) History() []EmbeddingVersion {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]EmbeddingVersion, len(r.history))
	copy(out, r.history)
	return out
}

// NeedsReEmbed reports whether the current registry version differs from the
// version embedded in a stored [EmbeddedText], indicating that the text
// should be re-embedded.
func NeedsReEmbed(ctx context.Context, reg VersionRegistry, et EmbeddedText) (bool, error) {
	cur, err := reg.CurrentVersion(ctx)
	if err != nil {
		return false, err
	}
	if cur == nil {
		return false, nil
	}
	return cur.Version != et.Version, nil
}
