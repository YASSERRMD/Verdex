package adaptiveretrieval_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/adaptiveretrieval"
	"github.com/YASSERRMD/verdex/packages/irac"
)

func TestNewCache_NegativeCapacity_ReturnsError(t *testing.T) {
	_, err := adaptiveretrieval.NewCache(adaptiveretrieval.CacheOptions{Capacity: -1})
	if err == nil {
		t.Fatal("expected error for negative capacity")
	}
}

func TestNewCache_ZeroCapacity_FallsBackToDefault(t *testing.T) {
	c, err := adaptiveretrieval.NewCache(adaptiveretrieval.CacheOptions{})
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil cache")
	}
}

func TestReindexOnRevision_EmptyCaseID_ReturnsError(t *testing.T) {
	c, _ := adaptiveretrieval.NewCache(adaptiveretrieval.CacheOptions{})
	err := adaptiveretrieval.ReindexOnRevision(c, irac.TreeRevision{})
	if err == nil {
		t.Fatal("expected error for empty case id")
	}
}

func TestReindexOnRevision_NilCache_NoOp(t *testing.T) {
	err := adaptiveretrieval.ReindexOnRevision(nil, irac.TreeRevision{CaseID: "case-1", RevisionNumber: 2})
	if err != nil {
		t.Fatalf("expected nil error for nil cache, got %v", err)
	}
}
