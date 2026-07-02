package citation_test

import (
	"context"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

// testProvenance builds a minimal irac.Provenance, mirroring the
// convention used by packages/hybridretrieval's own test helpers.
func testProvenance() irac.Provenance {
	return irac.Provenance{GeneratedBy: "citation-test", GeneratedAt: time.Now()}
}

// mustCreateNode persists node into store, failing the test on error.
func mustCreateNode(t *testing.T, store graph.GraphStore, node irac.Node) {
	t.Helper()
	if err := store.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("CreateNode(%s): %v", node.ID, err)
	}
}

// ruleNode builds a NodeRule irac.Node fixture with the given id, caseID,
// and text.
func ruleNode(id, caseID, text string) irac.Node {
	return irac.Node{
		ID:         id,
		Type:       irac.NodeRule,
		CaseID:     caseID,
		Text:       text,
		CreatedAt:  time.Now(),
		Confidence: 0.9,
		Provenance: testProvenance(),
	}
}

// factNode builds a NodeFact irac.Node fixture with the given id, caseID,
// and text.
func factNode(id, caseID, text string) irac.Node {
	return irac.Node{
		ID:         id,
		Type:       irac.NodeFact,
		CaseID:     caseID,
		Text:       text,
		CreatedAt:  time.Now(),
		Confidence: 0.8,
		Provenance: testProvenance(),
	}
}
