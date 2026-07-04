package iac

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// repoInfraDir resolves the infra/<tier> directory relative to this
// package's own location (packages/iac/../../infra), so these tests
// run correctly regardless of the caller's working directory (`go
// test ./...` from the module root, or from packages/iac itself both
// resolve the same way `go test` always sets the working directory to
// the package under test).
func repoInfraDir(tier string) string {
	return filepath.Join("..", "..", "infra", tier)
}

// TestValidateManifest_RealCommittedManifests proves ValidateManifest
// passes every manifest this phase actually committed under infra/ --
// the positive case a validator must get right before its negative
// case (below) means anything.
func TestValidateManifest_RealCommittedManifests(t *testing.T) {
	cases := []struct {
		tier  Tier
		files []string
	}{
		{TierCloud, []string{"docker-compose.yml", "configmap.yaml", "deployment.yaml", "service.yaml"}},
		{TierOnPrem, []string{"docker-compose.yml", "configmap.yaml", "deployment.yaml", "service.yaml", "postgres-pvc.yaml"}},
		{TierAirgapped, []string{"docker-compose.yml", "configmap.yaml", "deployment.yaml", "service.yaml", "profile-composition.yaml"}},
	}

	for _, tc := range cases {
		for _, file := range tc.files {
			path := filepath.Join(repoInfraDir(string(tc.tier)), file)
			t.Run(string(tc.tier)+"/"+file, func(t *testing.T) {
				if _, err := os.Stat(path); err != nil {
					t.Fatalf("fixture missing, adjust this test if infra/ layout changed: %v", err)
				}
				report, err := ValidateManifest(tc.tier, path)
				if err != nil {
					t.Fatalf("ValidateManifest(%s, %s) returned error: %v", tc.tier, path, err)
				}
				if !report.Passed() {
					t.Errorf("ValidateManifest(%s, %s) did not pass; failures: %+v", tc.tier, path, report.Failures())
				}
			})
		}
	}
}

// TestValidateManifest_CatchesDeliberatelyBrokenFixtures is the
// negative case the brief calls for explicitly: a validator that
// cannot fail is worthless (mirroring Phase 086's security-testing
// harness principle). Each fixture below deliberately violates one
// rule ValidateManifest checks, and every one must come back with
// Passed() == false.
func TestValidateManifest_CatchesDeliberatelyBrokenFixtures(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name    string
		tier    Tier
		content string
	}{
		{
			name: "airgapped compose image uses public tag not digest",
			tier: TierAirgapped,
			content: `
services:
  gateway:
    image: postgres:16
    environment:
      VERDEX_DATABASE_DSN: "env://VERDEX_DATABASE_DSN"
`,
		},
		{
			name: "compose secret is a literal value not env://",
			tier: TierCloud,
			content: `
services:
  gateway:
    image: verdex/gateway@sha256:1111111111111111111111111111111111111111111111111111111111111111
    environment:
      VERDEX_DATABASE_PASSWORD: "hunter2"
`,
		},
		{
			name: "k8s deployment container missing securityContext hardening",
			tier: TierCloud,
			content: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: broken
spec:
  template:
    spec:
      containers:
        - name: gateway
          image: verdex/gateway@sha256:1111111111111111111111111111111111111111111111111111111111111111
`,
		},
		{
			name: "k8s deployment allows privilege escalation",
			tier: TierCloud,
			content: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: broken
spec:
  template:
    spec:
      containers:
        - name: gateway
          image: verdex/gateway@sha256:1111111111111111111111111111111111111111111111111111111111111111
          securityContext:
            allowPrivilegeEscalation: true
            readOnlyRootFilesystem: true
            capabilities:
              drop: ["ALL"]
`,
		},
		{
			name: "airgapped k8s deployment uses a floating tag and default pull policy",
			tier: TierAirgapped,
			content: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: broken
spec:
  template:
    spec:
      containers:
        - name: gateway
          image: verdex/gateway:latest
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities:
              drop: ["ALL"]
`,
		},
		{
			name: "cloud configmap missing dataresidency region",
			tier: TierCloud,
			content: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: broken
data:
  VERDEX_PROFILE: "cloud"
`,
		},
		{
			name: "onprem configmap wrongly declares a cloud region",
			tier: TierOnPrem,
			content: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: broken
data:
  VERDEX_PROFILE: "onprem"
  VERDEX_DATARESIDENCY_REGION: "eu-west-1"
`,
		},
		{
			name: "configmap profile does not match tier",
			tier: TierOnPrem,
			content: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: broken
data:
  VERDEX_PROFILE: "cloud"
`,
		},
		{
			name: "k8s service with no ports",
			tier: TierCloud,
			content: `
apiVersion: v1
kind: Service
metadata:
  name: broken
spec:
  selector:
    app: gateway
`,
		},
		{
			name: "cloud tier declares a PersistentVolumeClaim it should not",
			tier: TierCloud,
			content: `
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: broken
spec:
  storageClassName: local-path
`,
		},
		{
			name: "onprem PVC uses a cloud storage class",
			tier: TierOnPrem,
			content: `
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: broken
spec:
  storageClassName: cloud-elastic-ssd
`,
		},
		{
			name: "airgap profile-composition missing required fields",
			tier: TierAirgapped,
			content: `
tier: airgapped
offline_registry_host: verdex-registry.local
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, sanitizeFilename(tc.name)+".yaml")
			if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
				t.Fatalf("failed to write fixture: %v", err)
			}

			report, err := ValidateManifest(tc.tier, path)
			if err != nil {
				t.Fatalf("ValidateManifest returned unexpected error (want a failed report, not a Go error): %v", err)
			}
			if report.Passed() {
				t.Errorf("expected broken fixture %q to fail validation, but it passed", tc.name)
			}
			if len(report.Failures()) == 0 {
				t.Errorf("expected at least one recorded failure for %q", tc.name)
			}
		})
	}
}

func sanitizeFilename(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch r {
		case ' ', '/', ':':
			out = append(out, '_')
		default:
			out = append(out, r)
		}
	}
	return string(out)
}

func TestValidateManifest_ArgumentErrors(t *testing.T) {
	if _, err := ValidateManifest(TierCloud, ""); !errors.Is(err, ErrEmptyManifestPath) {
		t.Errorf("empty path: got %v, want ErrEmptyManifestPath", err)
	}
	if _, err := ValidateManifest(Tier("bogus"), "somefile.yaml"); !errors.Is(err, ErrInvalidTier) {
		t.Errorf("invalid tier: got %v, want ErrInvalidTier", err)
	}
	if _, err := ValidateManifest(TierCloud, "/nonexistent/path/does/not/exist.yaml"); !errors.Is(err, ErrManifestNotFound) {
		t.Errorf("missing file: got %v, want ErrManifestNotFound", err)
	}
}

func TestValidateManifest_NotYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "not-yaml.yaml")
	// A tab character at the top level of a YAML document is a
	// classic parse error (YAML disallows tabs for indentation).
	if err := os.WriteFile(path, []byte("key:\n\tvalue: [unterminated"), 0o600); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	if _, err := ValidateManifest(TierCloud, path); !errors.Is(err, ErrManifestNotYAML) {
		t.Errorf("got %v, want ErrManifestNotYAML", err)
	}
}

func TestValidateManifest_EmptyDocument(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.yaml")
	if err := os.WriteFile(path, []byte("---\n"), 0o600); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	report, err := ValidateManifest(TierCloud, path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Passed() {
		t.Error("expected an empty document to fail validation")
	}
}

func TestValidationReport_PassedAndFailures_NilAndEmpty(t *testing.T) {
	var nilReport *ValidationReport
	if nilReport.Passed() {
		t.Error("nil report should not report Passed")
	}
	if nilReport.Failures() != nil {
		t.Error("nil report should have nil Failures")
	}

	empty := &ValidationReport{}
	if empty.Passed() {
		t.Error("report with no checks should not report Passed")
	}
}
