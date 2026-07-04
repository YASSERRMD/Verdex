package bulkimport_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/bulkimport"
)

func TestComputeDedupKey_NormalizesFormatting(t *testing.T) {
	t.Parallel()

	a := bulkimport.ComputeDedupKey("CASE-100", "Dubai Courts", []string{"Jane Doe", "Acme LLC"})
	b := bulkimport.ComputeDedupKey(" case-100 ", "dubai courts", []string{"acme llc", "jane doe"})

	if a == "" || b == "" {
		t.Fatal("ComputeDedupKey returned empty key for non-empty input")
	}
	if a != b {
		t.Fatalf("keys differ for equivalent records: %q vs %q", a, b)
	}
}

func TestComputeDedupKey_DistinctInputsProduceDistinctKeys(t *testing.T) {
	t.Parallel()

	a := bulkimport.ComputeDedupKey("CASE-100", "dubai-courts", []string{"Jane Doe"})
	b := bulkimport.ComputeDedupKey("CASE-200", "dubai-courts", []string{"Jane Doe"})
	if a == b {
		t.Fatalf("distinct case numbers produced the same key: %q", a)
	}

	c := bulkimport.ComputeDedupKey("CASE-100", "abu-dhabi-courts", []string{"Jane Doe"})
	if a == c {
		t.Fatalf("distinct jurisdictions produced the same key: %q", a)
	}

	d := bulkimport.ComputeDedupKey("CASE-100", "dubai-courts", []string{"John Roe"})
	if a == d {
		t.Fatalf("distinct party names produced the same key: %q", a)
	}
}

func TestComputeDedupKey_AllBlankReturnsEmpty(t *testing.T) {
	t.Parallel()
	key := bulkimport.ComputeDedupKey("", "", nil)
	if key != "" {
		t.Fatalf("ComputeDedupKey with all-blank input = %q, want empty string", key)
	}
}

func TestComputeDedupKey_PartyOrderIndependent(t *testing.T) {
	t.Parallel()
	a := bulkimport.ComputeDedupKey("CASE-1", "j1", []string{"Alice", "Bob", "Carol"})
	b := bulkimport.ComputeDedupKey("CASE-1", "j1", []string{"Carol", "Alice", "Bob"})
	if a != b {
		t.Fatalf("party name ordering changed the key: %q vs %q", a, b)
	}
}
