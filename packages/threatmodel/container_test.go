package threatmodel_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/YASSERRMD/verdex/packages/threatmodel"
)

func TestDefaultContainerHardeningChecklist_HasRealRules(t *testing.T) {
	t.Parallel()

	checklist := threatmodel.DefaultContainerHardeningChecklist()
	if len(checklist.Rules) == 0 {
		t.Fatal("DefaultContainerHardeningChecklist().Rules is empty, want real rules")
	}
	for _, r := range checklist.Rules {
		if r.Name == "" {
			t.Error("a rule has a blank Name")
		}
		if r.Description == "" {
			t.Errorf("rule %q has a blank Description", r.Name)
		}
	}

	names := checklist.RuleNames()
	wantSubset := []string{"non_root_user", "minimal_base_image", "pinned_base_image", "no_unnecessary_packages", "no_embedded_secrets"}
	for _, want := range wantSubset {
		found := false
		for _, n := range names {
			if n == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("RuleNames() = %v, want to include %q", names, want)
		}
	}
}

func TestContainerHardeningChecklist_Satisfied_NonRootUser(t *testing.T) {
	t.Parallel()

	checklist := threatmodel.ContainerHardeningChecklist{
		Rules: []threatmodel.ContainerHardeningRule{
			{Name: "non_root_user", Description: "must not run as root"},
		},
	}

	t.Run("USER nonroot satisfies the rule", func(t *testing.T) {
		t.Parallel()
		content := "FROM alpine\nUSER nonroot\nENTRYPOINT [\"/app\"]\n"
		passed := checklist.Satisfied(content)
		if len(passed) != 1 {
			t.Errorf("Satisfied() = %v, want the non_root_user rule to pass", passed)
		}
	})

	t.Run("USER root does not satisfy the rule", func(t *testing.T) {
		t.Parallel()
		content := "FROM alpine\nUSER root\nENTRYPOINT [\"/app\"]\n"
		passed := checklist.Satisfied(content)
		if len(passed) != 0 {
			t.Errorf("Satisfied() = %v, want no rules to pass for USER root", passed)
		}
	})

	t.Run("no USER directive at all does not satisfy the rule", func(t *testing.T) {
		t.Parallel()
		content := "FROM alpine\nENTRYPOINT [\"/app\"]\n"
		passed := checklist.Satisfied(content)
		if len(passed) != 0 {
			t.Errorf("Satisfied() = %v, want no rules to pass with no USER directive", passed)
		}
	})
}

func TestContainerHardeningChecklist_Satisfied_PinnedBaseImage(t *testing.T) {
	t.Parallel()

	checklist := threatmodel.ContainerHardeningChecklist{
		Rules: []threatmodel.ContainerHardeningRule{
			{Name: "pinned_base_image", Description: "must pin the base image"},
		},
	}

	t.Run("digest pin satisfies the rule", func(t *testing.T) {
		t.Parallel()
		content := "FROM golang@sha256:abcd1234\n"
		if passed := checklist.Satisfied(content); len(passed) != 1 {
			t.Errorf("Satisfied() = %v, want the rule to pass for a digest-pinned FROM", passed)
		}
	})

	t.Run("floating latest tag fails the rule", func(t *testing.T) {
		t.Parallel()
		content := "FROM golang:latest\n"
		if passed := checklist.Satisfied(content); len(passed) != 0 {
			t.Errorf("Satisfied() = %v, want the rule to fail for FROM golang:latest", passed)
		}
	})

	t.Run("no tag at all fails the rule", func(t *testing.T) {
		t.Parallel()
		content := "FROM golang\n"
		if passed := checklist.Satisfied(content); len(passed) != 0 {
			t.Errorf("Satisfied() = %v, want the rule to fail for an untagged FROM", passed)
		}
	})
}

func TestContainerHardeningChecklist_Satisfied_NoEmbeddedSecrets(t *testing.T) {
	t.Parallel()

	checklist := threatmodel.ContainerHardeningChecklist{
		Rules: []threatmodel.ContainerHardeningRule{
			{Name: "no_embedded_secrets", Description: "must not embed secrets"},
		},
	}

	t.Run("clean dockerfile satisfies the rule", func(t *testing.T) {
		t.Parallel()
		content := "FROM alpine\nCOPY app /app\n"
		if passed := checklist.Satisfied(content); len(passed) != 1 {
			t.Errorf("Satisfied() = %v, want the rule to pass for a clean Dockerfile", passed)
		}
	})

	t.Run("embedded password fails the rule", func(t *testing.T) {
		t.Parallel()
		content := "FROM alpine\nENV DB_PASSWORD=hunter2\n"
		if passed := checklist.Satisfied(content); len(passed) != 0 {
			t.Errorf("Satisfied() = %v, want the rule to fail with an embedded password", passed)
		}
	})
}

func TestContainerHardeningChecklist_Unsatisfied_IsComplementOfSatisfied(t *testing.T) {
	t.Parallel()

	checklist := threatmodel.DefaultContainerHardeningChecklist()
	content := "FROM golang:latest\nUSER root\n"

	satisfied := checklist.Satisfied(content)
	unsatisfied := checklist.Unsatisfied(content)

	if len(satisfied)+len(unsatisfied) != len(checklist.Rules) {
		t.Errorf("len(Satisfied)=%d + len(Unsatisfied)=%d != len(Rules)=%d", len(satisfied), len(unsatisfied), len(checklist.Rules))
	}

	seen := make(map[string]bool)
	for _, r := range satisfied {
		seen[r.Name] = true
	}
	for _, r := range unsatisfied {
		if seen[r.Name] {
			t.Errorf("rule %q appears in both Satisfied and Unsatisfied", r.Name)
		}
	}
}

// TestReferenceHardenedDockerfile_SatisfiesChecklist is an integration
// check against the actual committed doc/Dockerfile.hardened template
// (task 7): it must satisfy every automatically-checkable rule in
// DefaultContainerHardeningChecklist, proving the template is not just
// aspirational prose but a real artifact this package's own heuristics
// confirm.
func TestReferenceHardenedDockerfile_SatisfiesChecklist(t *testing.T) {
	t.Parallel()

	path := filepath.Join("doc", "Dockerfile.hardened")
	data, err := os.ReadFile(path) //nolint:gosec // fixed, repo-relative path to a committed doc template, not user input.
	if err != nil {
		t.Fatalf("os.ReadFile(%s): %v", path, err)
	}
	content := string(data)

	checklist := threatmodel.DefaultContainerHardeningChecklist()
	unsatisfied := checklist.Unsatisfied(content)
	if len(unsatisfied) != 0 {
		names := make([]string, 0, len(unsatisfied))
		for _, r := range unsatisfied {
			names = append(names, r.Name)
		}
		t.Errorf("doc/Dockerfile.hardened does not satisfy: %v", names)
	}
}
