package airgapped_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/YASSERRMD/verdex/packages/airgapped"
	"github.com/YASSERRMD/verdex/packages/provenance"
)

func testSigner() *provenance.HMACSigner {
	return provenance.NewHMACSigner([]byte("air-gapped-test-signing-key"))
}

func testSignerWithKey(key string) *provenance.HMACSigner {
	return provenance.NewHMACSigner([]byte(key))
}

// testBundleVersion is the fixed manifest version used by every test
// bundle buildSignedBundle produces.
const testBundleVersion = "2026-07-corpus-update"

// buildSignedBundle writes a bundle directory containing one data file
// and a signed manifest.json referencing it, returning the bundle dir.
func buildSignedBundle(t *testing.T, signer airgapped.Signer) string {
	t.Helper()
	dir := t.TempDir()
	dataPath := filepath.Join(dir, "corpus-update.statute.txt")
	if err := os.WriteFile(dataPath, []byte(testStatuteBody), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	checksum, err := airgapped.ChecksumFile(dataPath)
	if err != nil {
		t.Fatalf("ChecksumFile: %v", err)
	}

	manifest := airgapped.UpdateManifest{
		Version: testBundleVersion,
		Files:   map[string]string{"corpus-update.statute.txt": checksum},
	}
	if err := airgapped.SignManifest(context.Background(), signer, &manifest); err != nil {
		t.Fatalf("SignManifest: %v", err)
	}

	manifestBytes := marshalManifest(t, manifest)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), manifestBytes, 0o600); err != nil {
		t.Fatalf("WriteFile(manifest.json): %v", err)
	}
	return dir
}

func TestApplyUpdateBundle_ValidBundle(t *testing.T) {
	signer := testSigner()
	dir := buildSignedBundle(t, signer)

	report, err := airgapped.ApplyUpdateBundle(context.Background(), signer, dir)
	if err != nil {
		t.Fatalf("ApplyUpdateBundle: %v", err)
	}
	if report.Version != "2026-07-corpus-update" {
		t.Errorf("Version = %q, want %q", report.Version, "2026-07-corpus-update")
	}
	if len(report.AppliedFiles) != 1 {
		t.Fatalf("len(AppliedFiles) = %d, want 1", len(report.AppliedFiles))
	}
}

func TestApplyUpdateBundle_TamperedFileRejected(t *testing.T) {
	signer := testSigner()
	dir := buildSignedBundle(t, signer)

	// Tamper with the data file after the manifest was signed against
	// its original checksum.
	dataPath := filepath.Join(dir, "corpus-update.statute.txt")
	if err := os.WriteFile(dataPath, []byte("ACT 99: Tampered\nmalicious content\n"), 0o600); err != nil {
		t.Fatalf("WriteFile (tamper): %v", err)
	}

	_, err := airgapped.ApplyUpdateBundle(context.Background(), signer, dir)
	if !errors.Is(err, airgapped.ErrChecksumMismatch) {
		t.Fatalf("ApplyUpdateBundle(tampered file) = %v, want ErrChecksumMismatch", err)
	}
}

func TestApplyUpdateBundle_TamperedManifestRejected(t *testing.T) {
	signer := testSigner()
	dir := buildSignedBundle(t, signer)

	// Tamper with the manifest's declared checksum without re-signing.
	manifestPath := filepath.Join(dir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile(manifest): %v", err)
	}
	tampered := tamperManifestChecksum(t, data)
	if err := os.WriteFile(manifestPath, tampered, 0o600); err != nil {
		t.Fatalf("WriteFile(manifest tampered): %v", err)
	}

	_, err = airgapped.ApplyUpdateBundle(context.Background(), signer, dir)
	if !errors.Is(err, airgapped.ErrSignatureInvalid) {
		t.Fatalf("ApplyUpdateBundle(tampered manifest) = %v, want ErrSignatureInvalid", err)
	}
}

func TestApplyUpdateBundle_WrongSignerRejected(t *testing.T) {
	signer := testSigner()
	dir := buildSignedBundle(t, signer)

	wrongSigner := provenance.NewHMACSigner([]byte("a-different-key-entirely"))
	_, err := airgapped.ApplyUpdateBundle(context.Background(), wrongSigner, dir)
	if !errors.Is(err, airgapped.ErrSignatureInvalid) {
		t.Fatalf("ApplyUpdateBundle(wrong signer) = %v, want ErrSignatureInvalid", err)
	}
}

func TestApplyUpdateBundle_MissingManifest(t *testing.T) {
	dir := t.TempDir()
	_, err := airgapped.ApplyUpdateBundle(context.Background(), testSigner(), dir)
	if !errors.Is(err, airgapped.ErrInvalidManifest) {
		t.Fatalf("ApplyUpdateBundle(no manifest) = %v, want ErrInvalidManifest", err)
	}
}

func TestApplyUpdateBundle_EmptyBundlePath(t *testing.T) {
	_, err := airgapped.ApplyUpdateBundle(context.Background(), testSigner(), "")
	if !errors.Is(err, airgapped.ErrEmptyBundlePath) {
		t.Fatalf("ApplyUpdateBundle(\"\") = %v, want ErrEmptyBundlePath", err)
	}
}

func TestApplyUpdateBundle_NilSigner(t *testing.T) {
	_, err := airgapped.ApplyUpdateBundle(context.Background(), nil, "/tmp/whatever")
	if !errors.Is(err, airgapped.ErrNilSigner) {
		t.Fatalf("ApplyUpdateBundle(nil signer) = %v, want ErrNilSigner", err)
	}
}
