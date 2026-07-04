package scalability

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// PolicyConfig is the per-deployment, YAML-loadable scaling/capacity
// policy: the ScalingPolicy and BackpressureConfig a deployment
// actually runs with. This follows packages/config's (Phase 002)
// config-as-data convention -- a typed struct, a Default() baseline,
// and an optional YAML file overlay -- rather than inventing a
// separate configuration mechanism for this phase. Unlike
// packages/config.Config, PolicyConfig does not add its own
// environment-variable or named-profile layering: those are
// packages/config's general-purpose, cross-cutting concerns, and a
// deployment wanting env/profile overrides for its scaling policy can
// embed PolicyConfig as a field of its own service-specific config
// superset (packages/config.Config's own doc comment describes
// exactly this "embedded in a service-specific superset" pattern) and
// get env/profile layering for free from packages/config's existing
// Loader, rather than this package re-implementing that machinery.
type PolicyConfig struct {
	// Scaling is the autoscaling policy (see policy.go).
	Scaling ScalingPolicy `yaml:"scaling"`

	// Backpressure is the load-shedding configuration (see
	// backpressure.go).
	Backpressure BackpressureConfig `yaml:"backpressure"`
}

// scalingPolicyYAML and backpressureConfigYAML mirror
// ScalingPolicy/BackpressureConfig's fields with yaml struct tags,
// used only for (de)serialization. ScalingPolicy/BackpressureConfig
// themselves stay free of yaml tags so policy.go/backpressure.go
// remain readable independent of this file's serialization concern,
// mirroring how packages/router's CircuitBreaker (a pure Go type) is
// unaware of packages/config's separate YAML-loading layer.
type scalingPolicyYAML struct {
	MinReplicas    int     `yaml:"min_replicas"`
	MaxReplicas    int     `yaml:"max_replicas"`
	TargetMetric   float64 `yaml:"target_metric"`
	UpperTolerance float64 `yaml:"upper_tolerance"`
	LowerTolerance float64 `yaml:"lower_tolerance"`
	ScaleUpStep    int     `yaml:"scale_up_step"`
	ScaleDownStep  int     `yaml:"scale_down_step"`
}

type backpressureConfigYAML struct {
	MaxInFlight int `yaml:"max_in_flight"`
}

type policyConfigYAML struct {
	Scaling      scalingPolicyYAML      `yaml:"scaling"`
	Backpressure backpressureConfigYAML `yaml:"backpressure"`
}

// UnmarshalYAML implements yaml.Unmarshaler, translating the
// yaml-tagged intermediate shape into PolicyConfig's real
// ScalingPolicy/BackpressureConfig fields.
//
// raw is seeded from c's current values (not zero-valued) before
// decoding, so that a YAML document mentioning only some fields
// leaves every unmentioned field at whatever c already held (e.g.
// Default()'s values) rather than zeroing them out -- the same
// "only overwrite fields the document actually sets" behavior
// packages/config.loadYAMLFile documents, applied here because a
// custom Unmarshaler must reproduce that merge behavior explicitly
// (encoding/yaml's default struct decoding already merges onto an
// existing value; a hand-written Unmarshaler does not get that for
// free unless it seeds raw itself).
func (c *PolicyConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	raw := policyConfigYAML{
		Scaling: scalingPolicyYAML{
			MinReplicas:    c.Scaling.MinReplicas,
			MaxReplicas:    c.Scaling.MaxReplicas,
			TargetMetric:   c.Scaling.TargetMetric,
			UpperTolerance: c.Scaling.UpperTolerance,
			LowerTolerance: c.Scaling.LowerTolerance,
			ScaleUpStep:    c.Scaling.ScaleUpStep,
			ScaleDownStep:  c.Scaling.ScaleDownStep,
		},
		Backpressure: backpressureConfigYAML{
			MaxInFlight: c.Backpressure.MaxInFlight,
		},
	}
	if err := unmarshal(&raw); err != nil {
		return err
	}
	c.Scaling = ScalingPolicy{
		MinReplicas:    raw.Scaling.MinReplicas,
		MaxReplicas:    raw.Scaling.MaxReplicas,
		TargetMetric:   raw.Scaling.TargetMetric,
		UpperTolerance: raw.Scaling.UpperTolerance,
		LowerTolerance: raw.Scaling.LowerTolerance,
		ScaleUpStep:    raw.Scaling.ScaleUpStep,
		ScaleDownStep:  raw.Scaling.ScaleDownStep,
	}
	c.Backpressure = BackpressureConfig{
		MaxInFlight: raw.Backpressure.MaxInFlight,
	}
	return nil
}

// MarshalYAML implements yaml.Marshaler, the inverse of
// UnmarshalYAML, so a PolicyConfig built in code (e.g.
// DefaultPolicyConfig) can be written back out as a starter YAML
// file.
func (c PolicyConfig) MarshalYAML() (interface{}, error) {
	return policyConfigYAML{
		Scaling: scalingPolicyYAML{
			MinReplicas:    c.Scaling.MinReplicas,
			MaxReplicas:    c.Scaling.MaxReplicas,
			TargetMetric:   c.Scaling.TargetMetric,
			UpperTolerance: c.Scaling.UpperTolerance,
			LowerTolerance: c.Scaling.LowerTolerance,
			ScaleUpStep:    c.Scaling.ScaleUpStep,
			ScaleDownStep:  c.Scaling.ScaleDownStep,
		},
		Backpressure: backpressureConfigYAML{
			MaxInFlight: c.Backpressure.MaxInFlight,
		},
	}, nil
}

// DefaultPolicyConfig returns a PolicyConfig populated with sane
// defaults suitable for local development: DefaultScalingPolicy plus
// a conservative BackpressureConfig.MaxInFlight, mirroring
// packages/config.Default()'s role as the lowest-precedence layer.
func DefaultPolicyConfig() PolicyConfig {
	return PolicyConfig{
		Scaling:      DefaultScalingPolicy(),
		Backpressure: BackpressureConfig{MaxInFlight: 100},
	}
}

// Validate checks that both embedded policies are structurally
// well-formed.
func (c PolicyConfig) Validate() error {
	if err := c.Scaling.Validate(); err != nil {
		return err
	}
	if err := c.Backpressure.Validate(); err != nil {
		return err
	}
	return nil
}

// LoadPolicyConfig returns DefaultPolicyConfig() overlaid with the
// YAML file at path, following packages/config's loadYAMLFile
// convention exactly: only the fields the file actually mentions are
// overwritten, and an empty path is a no-op that just returns the
// defaults. The result is validated before being returned.
//
// If a deployment's own service-wide packages/config.Config (or a
// superset embedding PolicyConfig) is already YAML-loaded elsewhere,
// that loader's file can embed a top-level "scalability:" section
// with this same "scaling"/"backpressure" shape and the caller can
// unmarshal it into a PolicyConfig directly instead of calling this
// function -- LoadPolicyConfig exists for callers that want this
// policy loaded from its own standalone file.
func LoadPolicyConfig(path string) (PolicyConfig, error) {
	cfg := DefaultPolicyConfig()

	if path != "" {
		data, err := os.ReadFile(path) // #nosec G304 -- path is an operator-supplied config location, not untrusted user input
		if err != nil {
			return PolicyConfig{}, fmt.Errorf("scalability: read policy config %q: %w", path, err)
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return PolicyConfig{}, fmt.Errorf("scalability: parse policy config %q: %w", path, err)
		}
	}

	if err := cfg.Validate(); err != nil {
		return PolicyConfig{}, fmt.Errorf("scalability: %w", err)
	}

	return cfg, nil
}
