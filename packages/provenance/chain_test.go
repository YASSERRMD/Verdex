package provenance_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/provenance"
)

func makeRecord(contentHash string) provenance.ProvenanceRecord {
	tenantID := uuid.New()
	return provenance.ProvenanceRecord{
		ID:            uuid.New(),
		TenantID:      tenantID,
		UploaderID:    uuid.New(),
		Filename:      "test.pdf",
		MIMEType:      "application/pdf",
		SizeBytes:     1024,
		ContentHash:   contentHash,
		HashAlgorithm: "sha256",
		UploadedAt:    time.Now().UTC(),
	}
}

func TestBuildChain_ProducesValidChain(t *testing.T) {
	records := []provenance.ProvenanceRecord{
		makeRecord("hash-a"),
		makeRecord("hash-b"),
		makeRecord("hash-c"),
	}

	built := provenance.BuildChain(records)

	for i, r := range built {
		if r.ChainHash == "" {
			t.Errorf("record[%d] has empty ChainHash", i)
		}
	}

	valid, brokenAt, err := provenance.VerifyChain(built)
	if err != nil {
		t.Fatalf("VerifyChain: %v (brokenAt=%d)", err, brokenAt)
	}
	if !valid {
		t.Fatalf("expected valid chain, broken at %d", brokenAt)
	}
}

func TestVerifyChain_DetectsGap(t *testing.T) {
	records := []provenance.ProvenanceRecord{
		makeRecord("hash-a"),
		makeRecord("hash-b"),
		makeRecord("hash-c"),
	}
	built := provenance.BuildChain(records)

	// Introduce a gap: corrupt the chain hash of record[1].
	built[1].ChainHash = "0000000000000000000000000000000000000000000000000000000000000000"

	valid, brokenAt, err := provenance.VerifyChain(built)
	if valid {
		t.Fatal("expected invalid chain after corruption")
	}
	if brokenAt != 1 {
		t.Fatalf("expected brokenAt=1, got %d", brokenAt)
	}
	if err == nil {
		t.Fatal("expected non-nil error for broken chain")
	}
}

func TestVerifyChain_EmptyReturnsValid(t *testing.T) {
	valid, brokenAt, err := provenance.VerifyChain([]provenance.ProvenanceRecord{})
	if !valid {
		t.Fatal("expected empty chain to be valid")
	}
	if brokenAt != -1 {
		t.Fatalf("expected brokenAt=-1, got %d", brokenAt)
	}
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestComputeChainHash_FirstRecord(t *testing.T) {
	r := makeRecord("hash-x")
	h := provenance.ComputeChainHash(nil, r)
	if h == "" {
		t.Fatal("expected non-empty chain hash for first record")
	}
}

func TestBuildChain_SingleRecord(t *testing.T) {
	records := []provenance.ProvenanceRecord{makeRecord("only")}
	built := provenance.BuildChain(records)
	valid, brokenAt, err := provenance.VerifyChain(built)
	if err != nil {
		t.Fatalf("VerifyChain single: %v", err)
	}
	if !valid {
		t.Fatalf("single-record chain broken at %d", brokenAt)
	}
}
