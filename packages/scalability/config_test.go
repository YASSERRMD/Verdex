package scalability

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefaultPolicyConfigIsValid(t *testing.T) {
	cfg := DefaultPolicyConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected DefaultPolicyConfig to be valid, got %v", err)
	}
	if cfg.Scaling != DefaultScalingPolicy() {
		t.Fatalf("expected Scaling=DefaultScalingPolicy(), got %+v", cfg.Scaling)
	}
	if cfg.Backpressure.MaxInFlight != 100 {
		t.Fatalf("expected Backpressure.MaxInFlight=100, got %d", cfg.Backpressure.MaxInFlight)
	}
}

func TestPolicyConfigValidatePropagatesScalingError(t *testing.T) {
	cfg := DefaultPolicyConfig()
	cfg.Scaling.MinReplicas = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error from invalid Scaling policy")
	}
}

func TestPolicyConfigValidatePropagatesBackpressureError(t *testing.T) {
	cfg := DefaultPolicyConfig()
	cfg.Backpressure.MaxInFlight = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error from invalid Backpressure config")
	}
}

func TestLoadPolicyConfigEmptyPathReturnsDefaults(t *testing.T) {
	cfg, err := LoadPolicyConfig("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != DefaultPolicyConfig() {
		t.Fatalf("expected empty path to yield DefaultPolicyConfig(), got %+v", cfg)
	}
}

func TestLoadPolicyConfigMissingFile(t *testing.T) {
	_, err := LoadPolicyConfig(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadPolicyConfigOverlaysOnlyMentionedFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	// Only mention max_replicas and max_in_flight; every other field
	// should retain its Default() value, mirroring
	// packages/config.loadYAMLFile's "only overwrite mentioned
	// fields" contract.
	yamlContent := `
scaling:
  max_replicas: 50
backpressure:
  max_in_flight: 500
`
	if err := os.WriteFile(path, []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write test fixture: %v", err)
	}

	cfg, err := LoadPolicyConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Scaling.MaxReplicas != 50 {
		t.Fatalf("expected MaxReplicas=50 from file, got %d", cfg.Scaling.MaxReplicas)
	}
	if cfg.Backpressure.MaxInFlight != 500 {
		t.Fatalf("expected MaxInFlight=500 from file, got %d", cfg.Backpressure.MaxInFlight)
	}

	def := DefaultScalingPolicy()
	if cfg.Scaling.MinReplicas != def.MinReplicas {
		t.Fatalf("expected MinReplicas to retain default %d, got %d", def.MinReplicas, cfg.Scaling.MinReplicas)
	}
	if cfg.Scaling.TargetMetric != def.TargetMetric {
		t.Fatalf("expected TargetMetric to retain default %v, got %v", def.TargetMetric, cfg.Scaling.TargetMetric)
	}
}

func TestLoadPolicyConfigRejectsInvalidResult(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.yaml")
	// max_replicas below min_replicas (default MinReplicas=2) fails
	// ScalingPolicy.Validate.
	yamlContent := `
scaling:
  min_replicas: 10
  max_replicas: 1
`
	if err := os.WriteFile(path, []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write test fixture: %v", err)
	}

	_, err := LoadPolicyConfig(path)
	if err == nil {
		t.Fatal("expected validation error for max_replicas < min_replicas")
	}
}

func TestLoadPolicyConfigMalformedYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "malformed.yaml")
	if err := os.WriteFile(path, []byte("scaling: [this is not a mapping"), 0o600); err != nil {
		t.Fatalf("failed to write test fixture: %v", err)
	}

	_, err := LoadPolicyConfig(path)
	if err == nil {
		t.Fatal("expected parse error for malformed YAML")
	}
}

// TestPolicyConfigYAMLRoundTrip confirms Marshal then Unmarshal
// reproduces the original PolicyConfig exactly, so a
// DefaultPolicyConfig() can be written out as a real starter file
// (see doc/scalability.md) and read back unchanged.
func TestPolicyConfigYAMLRoundTrip(t *testing.T) {
	original := DefaultPolicyConfig()

	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}

	var roundTripped PolicyConfig
	if err := yaml.Unmarshal(data, &roundTripped); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}

	if roundTripped != original {
		t.Fatalf("round-trip mismatch: original=%+v roundTripped=%+v", original, roundTripped)
	}
}

// TestLoadPolicyConfigExampleFile loads the real, committed
// doc/policy.example.yaml fixture end to end, confirming it is
// actually valid and up to date with PolicyConfig's current shape
// rather than a stale, untested example a reader might copy-paste and
// have silently fail.
func TestLoadPolicyConfigExampleFile(t *testing.T) {
	cfg, err := LoadPolicyConfig("doc/policy.example.yaml")
	if err != nil {
		t.Fatalf("unexpected error loading example file: %v", err)
	}
	if cfg != DefaultPolicyConfig() {
		t.Fatalf("expected example file to match DefaultPolicyConfig() (it documents the same baseline), got %+v", cfg)
	}
}
