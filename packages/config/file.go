package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// loadYAMLFile reads the YAML file at path and unmarshals it onto cfg,
// overwriting only the fields present in the file. Fields omitted from
// the YAML document are left untouched, so a Config previously
// populated with defaults retains those defaults for any key the file
// does not mention.
//
// If path is empty, loadYAMLFile is a no-op and returns nil: a YAML
// layer is optional.
func loadYAMLFile(cfg *Config, path string) error {
	if path == "" {
		return nil
	}

	data, err := os.ReadFile(path) // #nosec G304 -- path is an operator-supplied config location, not untrusted user input
	if err != nil {
		return fmt.Errorf("config: read YAML file %q: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("config: parse YAML file %q: %w", path, err)
	}

	return nil
}
