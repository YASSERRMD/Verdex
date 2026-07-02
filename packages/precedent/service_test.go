package precedent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/YASSERRMD/verdex/packages/graph"
)

func TestPrecedentIngestionService_Ingest_FullPipeline(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	svc := &PrecedentIngestionService{
		Loader:    NewDefaultLoader(),
		Embedding: &fakeEmbeddingService{},
		Store:     store,
	}

	rules, err := svc.Ingest(context.Background(), IngestRequest{
		Source:           strings.NewReader(syntheticTextCorpus),
		JurisdictionCode: "UK",
		LegalFamily:      "common_law",
		CategoryCode:     "tort",
	})
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}
	if len(rules) == 0 {
		t.Fatal("Ingest() returned no rules")
	}

	for _, r := range rules {
		if r.JurisdictionCode != "UK" {
			t.Errorf("rule %q JurisdictionCode = %q, want UK", r.ID, r.JurisdictionCode)
		}
		if r.LegalFamily != "common_law" {
			t.Errorf("rule %q LegalFamily = %q, want common_law", r.ID, r.LegalFamily)
		}
		if r.Citation == "" {
			t.Errorf("rule %q Citation is empty", r.ID)
		}
		if r.Holding == "" {
			t.Errorf("rule %q Holding is empty", r.ID)
		}
		// Confirm round-trip persistence.
		got, err := store.GetNode(context.Background(), r.ID)
		if err != nil {
			t.Errorf("GetNode(%q) error = %v", r.ID, err)
		}
		if got.Text != r.Text {
			t.Errorf("GetNode(%q).Text = %q, want %q", r.ID, got.Text, r.Text)
		}
	}
}

func TestPrecedentIngestionService_IngestDetailed_CourtHierarchyAndAuthority(t *testing.T) {
	svc := &PrecedentIngestionService{
		Loader:    NewDefaultLoader(),
		Embedding: &fakeEmbeddingService{},
		Store:     graph.NewInMemoryGraphStore(),
	}

	result, err := svc.IngestDetailed(context.Background(), IngestRequest{
		Source:           strings.NewReader(syntheticTextCorpus),
		JurisdictionCode: "UK",
		CategoryCode:     "tort",
	})
	if err != nil {
		t.Fatalf("IngestDetailed() error = %v", err)
	}

	if len(result.PersistedRuleIDs) != len(result.Rules) {
		t.Errorf("len(PersistedRuleIDs) = %d, want %d", len(result.PersistedRuleIDs), len(result.Rules))
	}

	for _, r := range result.Rules {
		if !r.CourtLevel.IsValid() {
			t.Errorf("rule %q CourtLevel = %q, want a valid CourtLevel", r.ID, r.CourtLevel)
		}
		if len(r.Embeddings) == 0 {
			t.Errorf("rule %q has no embeddings", r.ID)
		}
		if r.Authority <= 0 {
			t.Errorf("rule %q Authority = %v, want > 0", r.ID, r.Authority)
		}
		if len(r.IssueKeywords) == 0 {
			t.Errorf("rule %q has no IssueKeywords", r.ID)
		}
	}
}

func TestPrecedentIngestionService_Ingest_Errors(t *testing.T) {
	svc := NewPrecedentIngestionService()

	if _, err := svc.Ingest(context.Background(), IngestRequest{Source: nil, JurisdictionCode: "UK"}); err == nil {
		t.Error("Ingest() with nil Source, error = nil, want error")
	}
	if _, err := svc.Ingest(context.Background(), IngestRequest{Source: strings.NewReader(syntheticTextCorpus), JurisdictionCode: ""}); err == nil {
		t.Error("Ingest() with empty JurisdictionCode, error = nil, want error")
	}
	if _, err := svc.Ingest(context.Background(), IngestRequest{Source: strings.NewReader(""), JurisdictionCode: "UK"}); err == nil {
		t.Error("Ingest() with empty corpus, error = nil, want error")
	} else if !errors.Is(err, ErrMalformedCorpus) {
		t.Errorf("errors.Is(err, ErrMalformedCorpus) = false, err = %v", err)
	}
}

func TestPrecedentIngestionService_Ingest_NoEmbeddingServiceStillPersists(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	svc := &PrecedentIngestionService{
		Loader: NewDefaultLoader(),
		Store:  store,
		// Embedding left nil deliberately.
	}

	rules, err := svc.Ingest(context.Background(), IngestRequest{
		Source:           strings.NewReader(syntheticTextCorpus),
		JurisdictionCode: "PK",
	})
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}
	if len(rules) == 0 {
		t.Fatal("Ingest() returned no rules")
	}
	for _, r := range rules {
		if _, err := store.GetNode(context.Background(), r.ID); err != nil {
			t.Errorf("GetNode(%q) error = %v, want persisted even without embedding", r.ID, err)
		}
	}
}

func TestNewPrecedentIngestionService_Defaults(t *testing.T) {
	svc := NewPrecedentIngestionService()
	if svc.Loader == nil {
		t.Error("Loader should default to a non-nil DefaultLoader")
	}
	if svc.Store == nil {
		t.Error("Store should default to a non-nil in-memory GraphStore")
	}
	if svc.Embedding != nil {
		t.Error("Embedding should default to nil (no live provider assumed)")
	}
}

func TestPrecedentIngestionService_CustomHoldingExtractor(t *testing.T) {
	svc := &PrecedentIngestionService{
		Loader: NewDefaultLoader(),
		Store:  graph.NewInMemoryGraphStore(),
		HoldingExtractor: func(fullText string) (HoldingExtractionResult, error) {
			return HoldingExtractionResult{Holding: "custom holding", RatioDecidendi: "custom ratio"}, nil
		},
	}

	rules, err := svc.Ingest(context.Background(), IngestRequest{
		Source:           strings.NewReader(syntheticTextCorpus),
		JurisdictionCode: "UK",
	})
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}
	for _, r := range rules {
		if r.Holding != "custom holding" {
			t.Errorf("Holding = %q, want custom holding", r.Holding)
		}
	}
}
