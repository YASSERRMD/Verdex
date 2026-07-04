package bulkimport_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/bulkimport"
)

func TestInMemoryRecordSource_ReadAt(t *testing.T) {
	t.Parallel()
	records := sampleSourceRecords(7)
	source := bulkimport.NewInMemoryRecordSource(records)

	if got := source.Len(); got != 7 {
		t.Fatalf("Len() = %d, want 7", got)
	}

	got, done, err := source.ReadAt(t.Context(), 0, 3)
	if err != nil {
		t.Fatalf("ReadAt(0, 3): %v", err)
	}
	if len(got) != 3 || done {
		t.Fatalf("ReadAt(0, 3) = (%d records, done=%v), want (3, false)", len(got), done)
	}

	got, done, err = source.ReadAt(t.Context(), 3, 3)
	if err != nil {
		t.Fatalf("ReadAt(3, 3): %v", err)
	}
	if len(got) != 3 || done {
		t.Fatalf("ReadAt(3, 3) = (%d records, done=%v), want (3, false)", len(got), done)
	}

	got, done, err = source.ReadAt(t.Context(), 6, 3)
	if err != nil {
		t.Fatalf("ReadAt(6, 3): %v", err)
	}
	if len(got) != 1 || !done {
		t.Fatalf("ReadAt(6, 3) = (%d records, done=%v), want (1, true)", len(got), done)
	}
}

func TestInMemoryRecordSource_ReadAt_PastEnd(t *testing.T) {
	t.Parallel()
	source := bulkimport.NewInMemoryRecordSource(sampleSourceRecords(3))

	got, done, err := source.ReadAt(t.Context(), 10, 5)
	if err != nil {
		t.Fatalf("ReadAt(10, 5): %v", err)
	}
	if len(got) != 0 || !done {
		t.Fatalf("ReadAt(10, 5) = (%d records, done=%v), want (0, true)", len(got), done)
	}
}

func TestInMemoryRecordSource_ReadAt_InvalidArgs(t *testing.T) {
	t.Parallel()
	source := bulkimport.NewInMemoryRecordSource(sampleSourceRecords(3))

	if _, done, err := source.ReadAt(t.Context(), -1, 5); err != nil || !done {
		t.Fatalf("ReadAt(-1, 5) = (done=%v, err=%v), want (true, nil)", done, err)
	}
	if _, done, err := source.ReadAt(t.Context(), 0, 0); err != nil || !done {
		t.Fatalf("ReadAt(0, 0) = (done=%v, err=%v), want (true, nil)", done, err)
	}
}

func TestInMemoryRecordSource_ReadAt_DeterministicAcrossCalls(t *testing.T) {
	t.Parallel()
	records := sampleSourceRecords(5)
	sourceA := bulkimport.NewInMemoryRecordSource(records)
	sourceB := bulkimport.NewInMemoryRecordSource(records)

	gotA, _, err := sourceA.ReadAt(t.Context(), 2, 2)
	if err != nil {
		t.Fatalf("sourceA.ReadAt: %v", err)
	}
	gotB, _, err := sourceB.ReadAt(t.Context(), 2, 2)
	if err != nil {
		t.Fatalf("sourceB.ReadAt: %v", err)
	}
	if len(gotA) != len(gotB) {
		t.Fatalf("ReadAt at the same index returned different lengths: %d vs %d", len(gotA), len(gotB))
	}
	for i := range gotA {
		if gotA[i].CaseNumber != gotB[i].CaseNumber {
			t.Fatalf("ReadAt at the same index returned different records at position %d: %q vs %q", i, gotA[i].CaseNumber, gotB[i].CaseNumber)
		}
	}
}
