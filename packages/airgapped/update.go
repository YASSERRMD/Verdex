package airgapped

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// updateManifestFileName is the fixed file name an update bundle
// directory must contain: a JSON manifest listing every other file in
// the bundle, its checksum, and a signature over the manifest itself.
const updateManifestFileName = "manifest.json"

// UpdateManifest is the on-disk, signed manifest of an offline update
// bundle (task 5). Files maps a bundle-relative file path to its
// SHA-256 checksum (hex-encoded); Signature is a provenance.Signer
// signature over the canonical JSON encoding of Files (see
// canonicalManifestPayload), so a tampered Files entry (or a tampered
// bundle file whose checksum no longer matches) is detected without
// any network call.
type UpdateManifest struct {
	// Version is a human-readable bundle version/identifier (e.g.
	// "2026-07-corpus-update"), informational only.
	Version string `json:"version"`

	// Files maps each bundle-relative file path to its expected
	// SHA-256 checksum, hex-encoded.
	Files map[string]string `json:"files"`

	// Signature is the hex-encoded provenance.Signer signature over
	// the canonical encoding of Version+Files.
	Signature string `json:"signature"`
}

// canonicalManifestPayload returns the deterministic byte encoding of
// m's signed fields (Version and Files, sorted by key), excluding
// Signature itself, mirroring packages/provenance's
// canonicalPayload/SignRecord convention of excluding the signature
// field from its own signed payload.
func canonicalManifestPayload(m UpdateManifest) []byte {
	keys := make([]string, 0, len(m.Files))
	for k := range m.Files {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	type entry struct {
		Path     string `json:"path"`
		Checksum string `json:"checksum"`
	}
	payload := struct {
		Version string  `json:"version"`
		Files   []entry `json:"files"`
	}{Version: m.Version}
	for _, k := range keys {
		payload.Files = append(payload.Files, entry{Path: k, Checksum: m.Files[k]})
	}
	// Marshal errors are impossible for this fixed, all-string/slice
	// shape.
	data, _ := json.Marshal(payload)
	return data
}

// SignManifest computes and sets m.Signature using signer over m's
// canonical payload. Used to produce an update bundle offline before
// it is distributed to an air-gapped deployment; ApplyUpdateBundle
// performs the corresponding verification.
func SignManifest(ctx context.Context, signer Signer, m *UpdateManifest) error {
	if signer == nil {
		return ErrNilSigner
	}
	sig, err := signer.Sign(ctx, canonicalManifestPayload(*m))
	if err != nil {
		return wrapf("SignManifest", err)
	}
	m.Signature = sig
	return nil
}

// Signer is the signature-verification dependency this package needs
// from packages/provenance, expressed as a local interface (matching
// provenance.Signer's method set exactly) so this package depends only
// on the shape it uses rather than importing packages/provenance's
// full surface into every signature. Any *provenance.HMACSigner
// satisfies this interface directly.
type Signer interface {
	Sign(ctx context.Context, data []byte) (signature string, err error)
	Verify(ctx context.Context, data []byte, signature string) (bool, error)
}

// UpdateReport is the result of ApplyUpdateBundle: which files were
// verified and applied, and the manifest version applied.
type UpdateReport struct {
	Version      string    `json:"version"`
	AppliedFiles []string  `json:"applied_files"`
	AppliedAt    time.Time `json:"applied_at"`
}

// ApplyUpdateBundle validates and applies a signed local update bundle
// (task 5) with no network call: bundlePath must be a directory
// containing manifest.json (an UpdateManifest) plus every file it
// references. The manifest's signature is verified via signer
// (typically a *provenance.HMACSigner keyed with an operator-controlled
// key distributed out of band), then every referenced file's SHA-256
// checksum is recomputed and compared against the manifest -- a
// tampered file or a tampered manifest both fail closed.
//
// ApplyUpdateBundle itself does not know how to "apply" a corpus
// update or a config change; task 5 asks for validation +
// application, and this function's application step is intentionally
// generic: on success it returns the verified file list so the caller
// (which knows whether the bundle contains corpus files for
// ProvisionCorpus, config overlays, etc.) can act on each verified
// file, e.g. by passing bundlePath itself to ProvisionCorpus once
// ApplyUpdateBundle confirms integrity.
func ApplyUpdateBundle(ctx context.Context, signer Signer, bundlePath string) (UpdateReport, error) {
	if bundlePath == "" {
		return UpdateReport{}, ErrEmptyBundlePath
	}
	if signer == nil {
		return UpdateReport{}, ErrNilSigner
	}
	if err := ctx.Err(); err != nil {
		return UpdateReport{}, err
	}

	manifestPath := filepath.Join(bundlePath, updateManifestFileName)
	data, err := os.ReadFile(manifestPath) //nolint:gosec // bundlePath is operator-supplied local path
	if err != nil {
		if os.IsNotExist(err) {
			return UpdateReport{}, wrapf("ApplyUpdateBundle", ErrInvalidManifest)
		}
		return UpdateReport{}, wrapf("ApplyUpdateBundle", err)
	}

	var manifest UpdateManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return UpdateReport{}, wrapf("ApplyUpdateBundle", ErrInvalidManifest)
	}
	if manifest.Signature == "" || len(manifest.Files) == 0 {
		return UpdateReport{}, wrapf("ApplyUpdateBundle", ErrInvalidManifest)
	}

	ok, err := signer.Verify(ctx, canonicalManifestPayload(manifest), manifest.Signature)
	if err != nil {
		return UpdateReport{}, wrapf("ApplyUpdateBundle", err)
	}
	if !ok {
		return UpdateReport{}, wrapf("ApplyUpdateBundle", ErrSignatureInvalid)
	}

	paths := make([]string, 0, len(manifest.Files))
	for p := range manifest.Files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, rel := range paths {
		expected := manifest.Files[rel]
		full := filepath.Join(bundlePath, rel)
		contents, err := os.ReadFile(full) //nolint:gosec // rel is drawn from a signature-verified manifest, not user input
		if err != nil {
			return UpdateReport{}, wrapf("ApplyUpdateBundle", err)
		}
		sum := sha256.Sum256(contents)
		actual := hex.EncodeToString(sum[:])
		if actual != expected {
			return UpdateReport{}, wrapf("ApplyUpdateBundle", ErrChecksumMismatch)
		}
	}

	return UpdateReport{
		Version:      manifest.Version,
		AppliedFiles: paths,
		AppliedAt:    time.Now().UTC(),
	}, nil
}

// ChecksumFile computes the hex-encoded SHA-256 checksum of the file at
// path, for building an UpdateManifest.Files entry.
func ChecksumFile(path string) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is operator-supplied when building a bundle offline
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
