package threatmodel_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/YASSERRMD/verdex/packages/threatmodel"
)

// repoRoot resolves this test's expectation of where the real
// repository root (containing go.work) lives, relative to this
// package's directory (packages/threatmodel), so GenerateSBOM can be
// exercised against the actual workspace rather than a synthetic
// fixture -- the whole point of this test is asserting it lists real
// modules from this real repository.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.work")); err != nil {
		t.Skipf("repository root go.work not found at %s (skipping SBOM test outside the full monorepo checkout): %v", root, err)
	}
	return root
}

func TestGenerateSBOM_ListsRealModules(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)

	sbom, err := threatmodel.GenerateSBOM(root)
	if err != nil {
		t.Fatalf("GenerateSBOM() error = %v", err)
	}

	if sbom.SchemaVersion == "" {
		t.Error("GenerateSBOM().SchemaVersion is blank, want a non-empty schema version")
	}
	if sbom.GeneratedAt.IsZero() {
		t.Error("GenerateSBOM().GeneratedAt is zero, want a real timestamp")
	}
	if len(sbom.Components) == 0 {
		t.Fatal("GenerateSBOM().Components is empty, want real modules")
	}

	var foundGoogleUUID, foundOwnModule bool
	for _, c := range sbom.Components {
		if c.Name == "github.com/google/uuid" {
			foundGoogleUUID = true
			if len(c.Versions) == 0 {
				t.Error("github.com/google/uuid component has no Versions recorded")
			}
			if c.Type != threatmodel.SBOMComponentLibrary {
				t.Errorf("github.com/google/uuid component Type = %v, want SBOMComponentLibrary", c.Type)
			}
		}
		if c.Name == "github.com/YASSERRMD/verdex/packages/threatmodel" {
			foundOwnModule = true
			if c.Type != threatmodel.SBOMComponentApplication {
				t.Errorf("this package's own component Type = %v, want SBOMComponentApplication", c.Type)
			}
		}
	}
	if !foundGoogleUUID {
		t.Error("GenerateSBOM() did not list github.com/google/uuid, a real dependency of this very package")
	}
	if !foundOwnModule {
		t.Error("GenerateSBOM() did not list this package's own first-party module")
	}
}

func TestGenerateSBOM_NoIndirectDependenciesListed(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)

	sbom, err := threatmodel.GenerateSBOM(root)
	if err != nil {
		t.Fatalf("GenerateSBOM() error = %v", err)
	}

	// github.com/munnerz/goautoneg is a real transitive dependency
	// (pulled in via prometheus/client_golang) that appears as
	// `// indirect` across many packages' go.mod files in this
	// repository and is never a direct require anywhere -- confirming
	// it does not appear in the SBOM at all proves the indirect-skip
	// logic actually works against real go.mod content, not just a
	// synthetic fixture.
	for _, c := range sbom.Components {
		if c.Name == "github.com/munnerz/goautoneg" {
			t.Errorf("GenerateSBOM() listed indirect-only dependency %q, want only direct requires", c.Name)
		}
	}
}

func TestGenerateSBOM_DeterministicOrdering(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)

	first, err := threatmodel.GenerateSBOM(root)
	if err != nil {
		t.Fatalf("GenerateSBOM() error = %v", err)
	}
	second, err := threatmodel.GenerateSBOM(root)
	if err != nil {
		t.Fatalf("GenerateSBOM() error = %v", err)
	}

	if len(first.Components) != len(second.Components) {
		t.Fatalf("GenerateSBOM() component count not stable across calls: %d vs %d", len(first.Components), len(second.Components))
	}
	for i := range first.Components {
		if first.Components[i].Name != second.Components[i].Name {
			t.Errorf("GenerateSBOM() ordering not deterministic at index %d: %q vs %q", i, first.Components[i].Name, second.Components[i].Name)
		}
	}
}

func TestGenerateSBOM_MissingGoWork(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	_, err := threatmodel.GenerateSBOM(tmp)
	if err == nil {
		t.Fatal("GenerateSBOM() error = nil, want an error for a directory with no go.work")
	}
}

func TestSummarizeSBOM(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)

	sbom, err := threatmodel.GenerateSBOM(root)
	if err != nil {
		t.Fatalf("GenerateSBOM() error = %v", err)
	}
	summary := threatmodel.SummarizeSBOM(sbom)
	if summary == "" {
		t.Error("SummarizeSBOM() returned an empty string")
	}
}

func TestWriteSBOMSnapshot(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	dest := filepath.Join(t.TempDir(), "sbom.json")

	if err := threatmodel.WriteSBOMSnapshot(root, dest); err != nil {
		t.Fatalf("WriteSBOMSnapshot() error = %v", err)
	}

	data, err := os.ReadFile(dest) //nolint:gosec // dest is a t.TempDir() path this test itself constructed.
	if err != nil {
		t.Fatalf("os.ReadFile(%s): %v", dest, err)
	}

	var sbom threatmodel.SBOM
	if err := json.Unmarshal(data, &sbom); err != nil {
		t.Fatalf("json.Unmarshal snapshot: %v", err)
	}
	if len(sbom.Components) == 0 {
		t.Error("WriteSBOMSnapshot() wrote a snapshot with no components")
	}
}
