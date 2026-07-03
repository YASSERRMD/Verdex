package keymanagement_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/keymanagement"
)

// inMemoryKeyBytes mirrors encryption.KeyBytes without importing
// packages/encryption from this test-only fixture.
const inMemoryKeyBytes = 32

// inMemoryProvider is a minimal, dependency-free keymanagement.Provider
// test fixture: key material lives only in a process-local map, and
// KeyMetadata bookkeeping is delegated to a real
// keymanagement.Repository (typically InMemoryRepository), mirroring
// how FileProvider splits "metadata" from "material" but without
// touching the filesystem. Used by every keymanagement_test test that
// does not specifically need FileProvider's on-disk behavior.
type inMemoryProvider struct {
	mu       sync.Mutex
	repo     keymanagement.Repository
	material map[string][]byte // keyed by "<tenantID>/<keyID>"
}

func newInMemoryProvider(repo keymanagement.Repository) *inMemoryProvider {
	return &inMemoryProvider{repo: repo, material: make(map[string][]byte)}
}

func (p *inMemoryProvider) materialKey(tenantID, keyID string) string {
	return tenantID + "/" + keyID
}

func (p *inMemoryProvider) CurrentKey(ctx context.Context, tenantID string) (keymanagement.KeyMaterial, error) {
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return keymanagement.KeyMaterial{}, err
	}
	meta, err := p.repo.GetActive(ctx, tid)
	if err != nil {
		return keymanagement.KeyMaterial{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	material, ok := p.material[p.materialKey(tenantID, meta.ID)]
	if !ok {
		return keymanagement.KeyMaterial{}, keymanagement.ErrInvalidKeyMaterial
	}
	return keymanagement.KeyMaterial{Metadata: *meta, Material: material}, nil
}

func (p *inMemoryProvider) Key(ctx context.Context, tenantID, keyID string) (keymanagement.KeyMaterial, error) {
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return keymanagement.KeyMaterial{}, err
	}
	meta, err := p.repo.Get(ctx, tid, keyID)
	if err != nil {
		return keymanagement.KeyMaterial{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	material, ok := p.material[p.materialKey(tenantID, keyID)]
	if !ok {
		return keymanagement.KeyMaterial{}, keymanagement.ErrInvalidKeyMaterial
	}
	return keymanagement.KeyMaterial{Metadata: *meta, Material: material}, nil
}

func (p *inMemoryProvider) Rotate(ctx context.Context, tenantID string) (keymanagement.KeyMetadata, error) {
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return keymanagement.KeyMetadata{}, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	prevVersion, err := p.repo.MaxVersion(ctx, tid)
	if err != nil {
		return keymanagement.KeyMetadata{}, err
	}
	newVersion := prevVersion + 1

	material := make([]byte, inMemoryKeyBytes)
	if _, err := rand.Read(material); err != nil {
		return keymanagement.KeyMetadata{}, err
	}
	newID := fmt.Sprintf("%s-v%d", tenantID, newVersion)

	if prev, err := p.repo.GetActive(ctx, tid); err == nil {
		if err := p.repo.UpdateState(ctx, tid, prev.ID, keymanagement.KeyStateRetired); err != nil {
			return keymanagement.KeyMetadata{}, err
		}
	}

	newMeta := &keymanagement.KeyMetadata{
		ID:        newID,
		TenantID:  tid,
		Version:   newVersion,
		State:     keymanagement.KeyStateActive,
		CreatedAt: time.Now().UTC(),
	}
	if err := p.repo.Create(ctx, tid, newMeta); err != nil {
		return keymanagement.KeyMetadata{}, err
	}
	p.material[p.materialKey(tenantID, newID)] = material

	return *newMeta, nil
}

var _ keymanagement.Provider = (*inMemoryProvider)(nil)

// seedActiveKey directly registers tenantID's first Active key
// version in both repo and provider, bypassing Rotate, for tests that
// want to start from an already-provisioned tenant rather than
// exercising Rotate itself.
func seedActiveKey(ctx context.Context, p *inMemoryProvider, tenantID uuid.UUID) (string, error) {
	meta, err := p.Rotate(ctx, tenantID.String())
	if err != nil {
		return "", err
	}
	return meta.ID, nil
}
