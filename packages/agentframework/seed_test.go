package agentframework_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/agentframework"
)

func TestSeed_ZeroValue_DisabledByDefault(t *testing.T) {
	var s agentframework.Seed
	if s.Enabled {
		t.Fatal("zero-value Seed.Enabled = true, want false")
	}
}

func TestNewSeed_SetsValueAndEnabled(t *testing.T) {
	s := agentframework.NewSeed(7)
	if !s.Enabled {
		t.Fatal("NewSeed().Enabled = false, want true")
	}
	if s.Value != 7 {
		t.Fatalf("NewSeed().Value = %d, want 7", s.Value)
	}
}

func TestDeterministicOnly_EnabledWithoutValue(t *testing.T) {
	s := agentframework.DeterministicOnly()
	if !s.Enabled {
		t.Fatal("DeterministicOnly().Enabled = false, want true")
	}
}
