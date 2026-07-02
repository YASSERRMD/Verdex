package statute

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/jurisdiction"
)

func TestStatuteIngestionService_Ingest_FullPipeline(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	svc := &StatuteIngestionService{
		Loader:    NewDefaultLoader(),
		Embedding: &fakeEmbeddingService{},
		Store:     store,
	}

	nodes, err := svc.Ingest(context.Background(), IngestRequest{
		Source:           strings.NewReader(syntheticTextCorpus),
		JurisdictionCode: "AE",
		LegalFamily:      jurisdiction.LegalFamilyCivilLaw,
		CategoryCode:     "civil",
	})
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}
	if len(nodes) == 0 {
		t.Fatal("Ingest() returned no rule nodes")
	}

	for _, n := range nodes {
		if n.JurisdictionCode != "AE" {
			t.Errorf("node %q JurisdictionCode = %q, want AE", n.ID, n.JurisdictionCode)
		}
		if n.LegalFamily != string(jurisdiction.LegalFamilyCivilLaw) {
			t.Errorf("node %q LegalFamily = %q, want %q", n.ID, n.LegalFamily, jurisdiction.LegalFamilyCivilLaw)
		}
		// Confirm round-trip persistence.
		got, err := store.GetNode(context.Background(), n.ID)
		if err != nil {
			t.Errorf("GetNode(%q) error = %v", n.ID, err)
		}
		if got.Text != n.Text {
			t.Errorf("GetNode(%q).Text = %q, want %q", n.ID, got.Text, n.Text)
		}
	}
}

func TestStatuteIngestionService_IngestDetailed_CrossReferencesAndAmendments(t *testing.T) {
	svc := &StatuteIngestionService{
		Loader:    NewDefaultLoader(),
		Embedding: &fakeEmbeddingService{},
		Store:     graph.NewInMemoryGraphStore(),
	}

	result, err := svc.IngestDetailed(context.Background(), IngestRequest{
		Source:           strings.NewReader(syntheticTextCorpus),
		JurisdictionCode: "AE",
		CategoryCode:     "civil",
	})
	if err != nil {
		t.Fatalf("IngestDetailed() error = %v", err)
	}

	if len(result.CrossReferences) == 0 {
		t.Fatal("expected at least one detected cross-reference in the synthetic corpus")
	}
	// The synthetic corpus references "Section 12(a)" from Act 12's
	// Section 3, which does not exist within Act 12 (only sections 1-3
	// exist) -- expect it to remain unresolved.
	if len(result.UnresolvedXRefs) == 0 {
		t.Error("expected at least one unresolved cross-reference")
	}

	if len(result.PersistedRuleIDs) != len(result.Rules) {
		t.Errorf("len(PersistedRuleIDs) = %d, want %d", len(result.PersistedRuleIDs), len(result.Rules))
	}

	for _, r := range result.Rules {
		if len(r.Embeddings) == 0 {
			t.Errorf("rule %q has no embeddings", r.Node.ID)
		}
		if r.Amendment.RuleID != r.Node.ID {
			t.Errorf("rule %q Amendment.RuleID = %q, want %q", r.Node.ID, r.Amendment.RuleID, r.Node.ID)
		}
	}
}

func TestStatuteIngestionService_Ingest_Errors(t *testing.T) {
	svc := NewStatuteIngestionService()

	if _, err := svc.Ingest(context.Background(), IngestRequest{Source: nil, JurisdictionCode: "AE"}); err == nil {
		t.Error("Ingest() with nil Source, error = nil, want error")
	}
	if _, err := svc.Ingest(context.Background(), IngestRequest{Source: strings.NewReader(syntheticTextCorpus), JurisdictionCode: ""}); err == nil {
		t.Error("Ingest() with empty JurisdictionCode, error = nil, want error")
	}
	if _, err := svc.Ingest(context.Background(), IngestRequest{Source: strings.NewReader(""), JurisdictionCode: "AE"}); err == nil {
		t.Error("Ingest() with empty corpus, error = nil, want error")
	} else if !errors.Is(err, ErrMalformedCorpus) {
		t.Errorf("errors.Is(err, ErrMalformedCorpus) = false, err = %v", err)
	}
}

func TestStatuteIngestionService_Ingest_NoEmbeddingServiceStillPersists(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	svc := &StatuteIngestionService{
		Loader: NewDefaultLoader(),
		Store:  store,
		// Embedding left nil deliberately.
	}

	nodes, err := svc.Ingest(context.Background(), IngestRequest{
		Source:           strings.NewReader(syntheticTextCorpus),
		JurisdictionCode: "PK",
	})
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}
	if len(nodes) == 0 {
		t.Fatal("Ingest() returned no nodes")
	}
	for _, n := range nodes {
		if _, err := store.GetNode(context.Background(), n.ID); err != nil {
			t.Errorf("GetNode(%q) error = %v, want persisted even without embedding", n.ID, err)
		}
	}
}

func TestNewStatuteIngestionService_Defaults(t *testing.T) {
	svc := NewStatuteIngestionService()
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
