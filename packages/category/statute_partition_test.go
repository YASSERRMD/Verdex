package category

import "testing"

func TestStatutePartitions_RegisterAndLookup(t *testing.T) {
	sp := NewStatutePartitions()

	ipc := StatutePartitionRef{PartitionID: "IN-IPC", Description: "Indian Penal Code"}
	consumerAct := StatutePartitionRef{PartitionID: "IN-CONSUMER-ACT"}

	sp.Register("IN", CodeCriminal, ipc)
	sp.Register("IN", CodeConsumer, consumerAct)

	tests := []struct {
		name             string
		jurisdictionCode string
		categoryCode     CategoryCode
		wantIDs          []string
	}{
		{"criminal in IN", "IN", CodeCriminal, []string{"IN-IPC"}},
		{"consumer in IN", "IN", CodeConsumer, []string{"IN-CONSUMER-ACT"}},
		{"unregistered category", "IN", CodeLabor, nil},
		{"unregistered jurisdiction", "AE", CodeCriminal, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sp.Lookup(tt.jurisdictionCode, tt.categoryCode)
			if len(got) != len(tt.wantIDs) {
				t.Fatalf("got %d refs, want %d", len(got), len(tt.wantIDs))
			}
			for i, ref := range got {
				if ref.PartitionID != tt.wantIDs[i] {
					t.Errorf("ref[%d].PartitionID = %q, want %q", i, ref.PartitionID, tt.wantIDs[i])
				}
			}
		})
	}
}

func TestStatutePartitions_RegisterAppends(t *testing.T) {
	sp := NewStatutePartitions()
	sp.Register("IN", CodeCriminal, StatutePartitionRef{PartitionID: "IN-IPC"})
	sp.Register("IN", CodeCriminal, StatutePartitionRef{PartitionID: "IN-CrPC"})

	got := sp.Lookup("IN", CodeCriminal)
	if len(got) != 2 {
		t.Fatalf("got %d refs, want 2 (register should append, not replace)", len(got))
	}
}

func TestStatutePartitions_LookupCategory(t *testing.T) {
	sp := NewStatutePartitions()
	sp.Register("IN", CodeCriminal, StatutePartitionRef{PartitionID: "IN-IPC"})

	criminal := Category{Code: CodeCriminal, Name: "Criminal"}
	got := sp.LookupCategory("IN", criminal)
	if len(got) != 1 || got[0].PartitionID != "IN-IPC" {
		t.Errorf("LookupCategory() = %v, want [{IN-IPC ...}]", got)
	}
}

func TestStatutePartitions_ZeroValue(t *testing.T) {
	var sp StatutePartitions
	got := sp.Lookup("IN", CodeCriminal)
	if got != nil {
		t.Errorf("zero-value Lookup() = %v, want nil", got)
	}

	sp.Register("IN", CodeCriminal, StatutePartitionRef{PartitionID: "IN-IPC"})
	got = sp.Lookup("IN", CodeCriminal)
	if len(got) != 1 {
		t.Errorf("got %d refs after Register on zero value, want 1", len(got))
	}
}
