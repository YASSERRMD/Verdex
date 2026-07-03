package keymanagement

import (
	"context"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/encryption"
)

// Adapter implements encryption.KeySource (and encryption.Rotator) by
// delegating to a Provider scoped to one tenant — this is the "real"
// KeySource packages/encryption/keysource.go names this phase as
// responsible for. Every packages/encryption.Encrypt/Decrypt call
// site that currently constructs an encryption.LocalKeySource can
// instead construct an Adapter and pass it in unchanged; no change to
// packages/encryption itself is required, exactly as that package's
// doc.go promises.
//
// Adapter is intentionally not permission-gated the way Service is:
// packages/encryption.Encrypt/Decrypt are called from trusted,
// internal server-side code paths (the same posture
// packages/notifications.Service.Notify documents for its own
// event-hook callers), not directly by an end-user request. A caller
// that needs role-gated, audited key operations exposed to an actual
// end user (e.g. an admin "rotate this tenant's key" button) should
// use Service instead — Adapter is the low-level plumbing Service's
// Rotate/CurrentKey/Key ultimately reach the same Provider through.
//
// Every Adapter is bound to exactly one tenant at construction time,
// which is how this package satisfies "per-tenant key isolation"
// (task 4) at the encryption.KeySource boundary: a caller cannot
// accidentally pass the wrong tenant's ID per-call the way a
// tenant-parameterized KeySource method might allow, because
// encryption.KeySource's methods take no tenant argument at all.
type Adapter struct {
	provider Provider
	tenantID uuid.UUID
}

// NewAdapter builds an Adapter bound to tenantID, backed by provider.
// Returns ErrNilProvider if provider is nil, ErrEmptyTenantID if
// tenantID is the zero UUID.
func NewAdapter(provider Provider, tenantID uuid.UUID) (*Adapter, error) {
	if provider == nil {
		return nil, ErrNilProvider
	}
	if tenantID == uuid.Nil {
		return nil, ErrEmptyTenantID
	}
	return &Adapter{provider: provider, tenantID: tenantID}, nil
}

// CurrentKey implements encryption.KeySource.
func (a *Adapter) CurrentKey(ctx context.Context) (encryption.Key, error) {
	material, err := a.provider.CurrentKey(ctx, a.tenantID.String())
	if err != nil {
		return encryption.Key{}, wrapf("Adapter.CurrentKey", err)
	}
	return encryption.Key{ID: material.Metadata.ID, Material: material.Material}, nil
}

// Key implements encryption.KeySource.
func (a *Adapter) Key(ctx context.Context, keyID string) (encryption.Key, error) {
	material, err := a.provider.Key(ctx, a.tenantID.String(), keyID)
	if err != nil {
		return encryption.Key{}, wrapf("Adapter.Key", err)
	}
	return encryption.Key{ID: material.Metadata.ID, Material: material.Material}, nil
}

// Rotate implements encryption.Rotator, so Adapter satisfies both
// encryption.KeySource and the optional Rotator capability
// LocalKeySource also implements — a caller that type-asserts an
// encryption.KeySource for Rotator (as packages/encryption's own doc
// comment on Rotator anticipates callers doing) gets real,
// Provider-backed rotation.
func (a *Adapter) Rotate(ctx context.Context) (string, error) {
	meta, err := a.provider.Rotate(ctx, a.tenantID.String())
	if err != nil {
		return "", wrapf("Adapter.Rotate", err)
	}
	return meta.ID, nil
}

var (
	_ encryption.KeySource = (*Adapter)(nil)
	_ encryption.Rotator   = (*Adapter)(nil)
)
