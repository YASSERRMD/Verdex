package keymanagement

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// keyMaterialBytes is the required length, in bytes, of a key
// version's material for AES-256: 256 bits. Mirrors
// packages/encryption.KeyBytes exactly (kept as a distinct constant
// so this file has no import-time dependency on packages/encryption;
// adapter.go is the only file in this package that imports it).
const keyMaterialBytes = 32

// FileProvider is a real, file-backed Provider implementation
// suitable for air-gapped deployments (task 8): every operation reads
// and writes local files under a root directory with zero network
// calls, which is exactly what an air-gapped deployment tier (Phase
// 079) needs from an "offline key store".
//
// # Layout
//
// Key material is partitioned per tenant, implementing per-tenant key
// isolation at the storage layer, not just the API layer:
//
//	<root>/<tenantID>/<keyID>.key
//
// Each .key file holds the key version's raw material, itself
// wrapped (AES-256-GCM) under a master key so key material is never
// written to disk in plaintext — see NewFileProvider's masterKey
// parameter. KeyMetadata for these key versions is tracked separately
// via a Repository (typically InMemoryRepository or
// TenantScopedRepository), matching every other Provider's split
// between "where metadata lives" and "where material lives"; see
// doc/key-management.md, "Offline key store wiring".
type FileProvider struct {
	root      string
	masterKey [32]byte
	repo      Repository

	mu sync.Mutex
}

// NewFileProvider builds a FileProvider rooted at root (created if it
// does not exist) and backed by repo for KeyMetadata bookkeeping.
// masterKey wraps every key version's material at rest and must be
// exactly 32 bytes — see DeriveMasterKey for turning an arbitrary
// passphrase (e.g. from an offline, out-of-band-distributed
// passphrase for an air-gapped deployment) into a valid masterKey.
func NewFileProvider(root string, masterKey []byte, repo Repository) (*FileProvider, error) {
	if root == "" {
		return nil, wrapf("NewFileProvider", errors.New("root path is required"))
	}
	if len(masterKey) != keyMaterialBytes {
		return nil, wrapf("NewFileProvider", ErrInvalidKeyMaterial)
	}
	if repo == nil {
		return nil, ErrNilRepository
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, wrapf("NewFileProvider", err)
	}

	fp := &FileProvider{root: root, repo: repo}
	copy(fp.masterKey[:], masterKey)
	return fp, nil
}

// DeriveMasterKey stretches an arbitrary-length passphrase into
// exactly keyMaterialBytes bytes using SHA-256, mirroring
// packages/encryption's deriveKey helper. This is a deterministic
// KDF-lite: adequate for turning an operator-supplied, out-of-band
// passphrase into a fixed-size master key for FileProvider, not a
// substitute for a real salted KDF in a higher-security deployment.
func DeriveMasterKey(passphrase string) []byte {
	sum := sha256.Sum256([]byte(passphrase))
	return sum[:]
}

// tenantDir returns (and ensures exists) the per-tenant subdirectory
// for tenantID.
func (p *FileProvider) tenantDir(tenantID string) (string, error) {
	dir := filepath.Join(p.root, filepath.FromSlash(tenantID))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// keyPath returns the on-disk path for a given tenant/key pair.
func (p *FileProvider) keyPath(tenantID, keyID string) (string, error) {
	dir, err := p.tenantDir(tenantID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, keyID+".key"), nil
}

// wrap encrypts material under the master key using AES-256-GCM with
// a fresh random nonce, returning nonce||ciphertext.
func (p *FileProvider) wrap(material []byte) ([]byte, error) {
	block, err := aes.NewCipher(p.masterKey[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, material, nil), nil
}

// unwrap reverses wrap.
func (p *FileProvider) unwrap(blob []byte) ([]byte, error) {
	block, err := aes.NewCipher(p.masterKey[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(blob) < gcm.NonceSize() {
		return nil, ErrInvalidKeyMaterial
	}
	nonce, ciphertext := blob[:gcm.NonceSize()], blob[gcm.NonceSize():]
	material, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidKeyMaterial, err)
	}
	return material, nil
}

// fileRecord is the on-disk JSON envelope for one wrapped key file.
type fileRecord struct {
	WrappedMaterial []byte `json:"wrapped_material"`
}

// writeKeyFile persists material (wrapped) to the file at path.
func (p *FileProvider) writeKeyFile(path string, material []byte) error {
	wrapped, err := p.wrap(material)
	if err != nil {
		return err
	}
	data, err := json.Marshal(fileRecord{WrappedMaterial: wrapped})
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// readKeyFile loads and unwraps the key material stored at path.
func (p *FileProvider) readKeyFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is built internally from tenant/key IDs, not user-controlled traversal input
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrOfflineStoreNotFound
		}
		return nil, err
	}
	var rec fileRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidKeyMaterial, err)
	}
	return p.unwrap(rec.WrappedMaterial)
}

// CurrentKey implements Provider.
func (p *FileProvider) CurrentKey(ctx context.Context, tenantID string) (KeyMaterial, error) {
	tid, err := parseTenantID(tenantID)
	if err != nil {
		return KeyMaterial{}, err
	}
	meta, err := p.repo.GetActive(ctx, tid)
	if err != nil {
		return KeyMaterial{}, wrapf("FileProvider.CurrentKey", err)
	}
	path, err := p.keyPath(tenantID, meta.ID)
	if err != nil {
		return KeyMaterial{}, wrapf("FileProvider.CurrentKey", err)
	}
	material, err := p.readKeyFile(path)
	if err != nil {
		return KeyMaterial{}, wrapf("FileProvider.CurrentKey", err)
	}
	return KeyMaterial{Metadata: *meta, Material: material}, nil
}

// Key implements Provider.
func (p *FileProvider) Key(ctx context.Context, tenantID, keyID string) (KeyMaterial, error) {
	tid, err := parseTenantID(tenantID)
	if err != nil {
		return KeyMaterial{}, err
	}
	meta, err := p.repo.Get(ctx, tid, keyID)
	if err != nil {
		return KeyMaterial{}, wrapf("FileProvider.Key", err)
	}
	path, err := p.keyPath(tenantID, keyID)
	if err != nil {
		return KeyMaterial{}, wrapf("FileProvider.Key", err)
	}
	material, err := p.readKeyFile(path)
	if err != nil {
		return KeyMaterial{}, wrapf("FileProvider.Key", err)
	}
	return KeyMaterial{Metadata: *meta, Material: material}, nil
}

// Rotate implements Provider. It generates fresh random 32-byte key
// material, writes it to a new file, records new KeyMetadata as
// Active, and demotes the tenant's prior Active version (if any) to
// Retired — never deleting its file, so old ciphertext stays
// decryptable, matching packages/encryption.Rotator's contract.
func (p *FileProvider) Rotate(ctx context.Context, tenantID string) (KeyMetadata, error) {
	tid, err := parseTenantID(tenantID)
	if err != nil {
		return KeyMetadata{}, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	prevVersion, err := p.repo.MaxVersion(ctx, tid)
	if err != nil {
		return KeyMetadata{}, wrapf("FileProvider.Rotate", err)
	}
	newVersion := prevVersion + 1

	material := make([]byte, keyMaterialBytes)
	if _, err := rand.Read(material); err != nil {
		return KeyMetadata{}, wrapf("FileProvider.Rotate", err)
	}

	newID := fmt.Sprintf("%s-v%d", tenantID, newVersion)
	path, err := p.keyPath(tenantID, newID)
	if err != nil {
		return KeyMetadata{}, wrapf("FileProvider.Rotate", err)
	}
	if err := p.writeKeyFile(path, material); err != nil {
		return KeyMetadata{}, wrapf("FileProvider.Rotate", err)
	}

	// Demote the prior Active version before promoting the new one, so
	// a crash between the two calls never leaves two Active keys (the
	// database's partial unique index would reject that anyway, but
	// this ordering keeps a crash window from ever reaching that
	// state).
	if prev, err := p.repo.GetActive(ctx, tid); err == nil {
		if err := p.repo.UpdateState(ctx, tid, prev.ID, KeyStateRetired); err != nil {
			return KeyMetadata{}, wrapf("FileProvider.Rotate", err)
		}
	} else if !errors.Is(err, ErrNoActiveKey) {
		return KeyMetadata{}, wrapf("FileProvider.Rotate", err)
	}

	newMeta := &KeyMetadata{
		ID:        newID,
		TenantID:  tid,
		Version:   newVersion,
		State:     KeyStateActive,
		CreatedAt: time.Now().UTC(),
	}
	if err := p.repo.Create(ctx, tid, newMeta); err != nil {
		return KeyMetadata{}, wrapf("FileProvider.Rotate", err)
	}

	return *newMeta, nil
}

var _ Provider = (*FileProvider)(nil)
