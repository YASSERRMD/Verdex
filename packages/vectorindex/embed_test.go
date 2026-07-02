package vectorindex_test

import (
	"context"
	"testing"

	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/vectorindex"
)

func TestEmbedLeaves_NilService(t *testing.T) {
	_, err := vectorindex.EmbedLeaves(context.Background(), nil, []vectorindex.IndexableLeaf{{ID: "a"}})
	if err != vectorindex.ErrNilEmbeddingService {
		t.Fatalf("expected ErrNilEmbeddingService, got %v", err)
	}
}

func TestEmbedLeaves_EmptyInput(t *testing.T) {
	svc := newFakeEmbeddingService("contract")
	records, err := vectorindex.EmbedLeaves(context.Background(), svc, nil)
	if err != nil {
		t.Fatalf("EmbedLeaves: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected 0 records, got %d", len(records))
	}
}

func TestEmbedLeaves_ProducesRecordsInOrder(t *testing.T) {
	svc := newFakeEmbeddingService("contract", "tort")
	leaves := []vectorindex.IndexableLeaf{
		{ID: "fact-1", NodeType: irac.NodeFact, CaseID: "case-1", Text: "a contract fact", CategoryCode: "contract"},
		{ID: "rule-1", NodeType: irac.NodeRule, CaseID: "case-1", Text: "a tort rule", JurisdictionCode: "us-ny"},
	}

	records, err := vectorindex.EmbedLeaves(context.Background(), svc, leaves)
	if err != nil {
		t.Fatalf("EmbedLeaves: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	if records[0].ID != "fact-1" || records[1].ID != "rule-1" {
		t.Fatalf("expected records in the same order as input leaves, got %q then %q", records[0].ID, records[1].ID)
	}
	if records[0].CategoryCode != "contract" {
		t.Errorf("record 0 CategoryCode = %q, want %q", records[0].CategoryCode, "contract")
	}
	if records[1].JurisdictionCode != "us-ny" {
		t.Errorf("record 1 JurisdictionCode = %q, want %q", records[1].JurisdictionCode, "us-ny")
	}
	for _, r := range records {
		if len(r.Vector) == 0 {
			t.Errorf("record %q: expected a non-empty vector", r.ID)
		}
		if r.ModelID != "fake-model" || r.ProviderID != "fake-provider" {
			t.Errorf("record %q: expected fake model/provider stamped, got %q/%q", r.ID, r.ModelID, r.ProviderID)
		}
	}
}
