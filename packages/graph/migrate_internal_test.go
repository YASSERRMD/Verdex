package graph

import "testing"

// TestCoreMigrations_AreIdempotentCypher verifies every built-in
// migration is expressed as idempotent Cypher (IF NOT EXISTS), the
// property Migrator.Apply's doc comment relies on for "applying the
// same Migrator twice is safe".
func TestCoreMigrations_AreIdempotentCypher(t *testing.T) {
	t.Parallel()

	migrations := coreMigrations()
	if len(migrations) == 0 {
		t.Fatal("expected at least one core migration")
	}

	for _, m := range migrations {
		if m.Name == "" {
			t.Errorf("migration has empty Name: %+v", m)
		}
		if !containsIfNotExists(m.Cypher) {
			t.Errorf("migration %q is not idempotent (missing IF NOT EXISTS): %q", m.Name, m.Cypher)
		}
	}
}

func containsIfNotExists(cypher string) bool {
	for i := 0; i+len("IF NOT EXISTS") <= len(cypher); i++ {
		if cypher[i:i+len("IF NOT EXISTS")] == "IF NOT EXISTS" {
			return true
		}
	}
	return false
}

func TestCoreMigrations_IncludesIndexMigrations(t *testing.T) {
	t.Parallel()

	core := coreMigrations()
	idx := indexMigrations()
	if len(idx) == 0 {
		t.Fatal("expected at least one index migration")
	}
	if len(core) < len(idx) {
		t.Fatalf("expected coreMigrations to include indexMigrations, got %d core vs %d index", len(core), len(idx))
	}
}
