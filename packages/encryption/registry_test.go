package encryption_test

import (
	"context"
	"testing"

	"github.com/YASSERRMD/verdex/packages/encryption"
)

type partyRecord struct {
	Name       string
	NationalID string `encrypted:"true"`
	Notes      []byte `encrypted:"true"`
}

type caseRecord struct {
	CaseID string
	Party  partyRecord
}

func TestScanForPlaintext_FlagsUnencryptedTaggedFields(t *testing.T) {
	rec := caseRecord{
		CaseID: "case-1",
		Party: partyRecord{
			Name:       "Jane Doe", // not tagged, never flagged
			NationalID: "784-1985-1234567-1",
			Notes:      []byte("plaintext notes"),
		},
	}

	findings, err := encryption.ScanForPlaintext(&rec)
	if err != nil {
		t.Fatalf("ScanForPlaintext() error = %v, want nil", err)
	}
	if len(findings) != 2 {
		t.Fatalf("ScanForPlaintext() found %d findings, want 2 (NationalID, Notes); got %+v", len(findings), findings)
	}

	paths := map[string]bool{}
	for _, f := range findings {
		paths[f.FieldPath] = true
	}
	if !paths["Party.NationalID"] {
		t.Error("expected a finding for Party.NationalID")
	}
	if !paths["Party.Notes"] {
		t.Error("expected a finding for Party.Notes")
	}
}

func TestScanForPlaintext_NoFindingsAfterEncryption(t *testing.T) {
	ctx := context.Background()
	ks := testKeySource(t)

	nationalIDCipher, err := encryption.Encrypt(ctx, ks, []byte("784-1985-1234567-1"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v, want nil", err)
	}
	notesCipher, err := encryption.Encrypt(ctx, ks, []byte("case notes"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v, want nil", err)
	}

	rec := caseRecord{
		CaseID: "case-1",
		Party: partyRecord{
			Name:       "Jane Doe",
			NationalID: string(nationalIDCipher),
			Notes:      notesCipher,
		},
	}

	findings, err := encryption.ScanForPlaintext(&rec)
	if err != nil {
		t.Fatalf("ScanForPlaintext() error = %v, want nil", err)
	}
	if len(findings) != 0 {
		t.Fatalf("ScanForPlaintext() found %d findings after encryption, want 0; got %+v", len(findings), findings)
	}
}

func TestScanForPlaintext_IgnoresEmptyTaggedFields(t *testing.T) {
	rec := caseRecord{CaseID: "case-1"}

	findings, err := encryption.ScanForPlaintext(&rec)
	if err != nil {
		t.Fatalf("ScanForPlaintext() error = %v, want nil", err)
	}
	if len(findings) != 0 {
		t.Fatalf("ScanForPlaintext() found %d findings for empty tagged fields, want 0", len(findings))
	}
}

func TestScanForPlaintext_RejectsNonStruct(t *testing.T) {
	if _, err := encryption.ScanForPlaintext(42); err == nil {
		t.Fatal("ScanForPlaintext(42) error = nil, want error for a non-struct value")
	}
}

func TestScanForPlaintext_NilPointerIsNoOp(t *testing.T) {
	var rec *caseRecord
	findings, err := encryption.ScanForPlaintext(rec)
	if err != nil {
		t.Fatalf("ScanForPlaintext(nil *caseRecord) error = %v, want nil", err)
	}
	if findings != nil {
		t.Fatalf("ScanForPlaintext(nil *caseRecord) findings = %+v, want nil", findings)
	}
}
