package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// ProfileEnvVar is the environment variable used to select a named
// config profile when the Loader is not given an explicit profile via
// WithProfile.
const ProfileEnvVar = "VERDEX_PROFILE"

// defaultProfileDirName is the directory, relative to the base config
// file's directory, that profile overlay files are read from when the
// loader is not given an explicit WithProfileDir.
const defaultProfileDirName = "profiles"

// applyProfile resolves which profile (if any) is selected and, if
// one is, layers its YAML overlay file on top of cfg. Profile
// selection precedence is: explicit Loader.profile (set via
// WithProfile) first, then the VERDEX_PROFILE environment variable. If
// neither is set, applyProfile is a no-op.
//
// The overlay file is expected at <profileDir>/<profile>.yaml, where
// profileDir defaults to "profiles" alongside the base config file (or
// the current directory if no base file was configured). A missing
// overlay file for an explicitly selected profile is an error: a typo
// in a profile name should fail loudly, not silently fall back to
// base config.
func applyProfile(cfg *Config, l *Loader) error {
	profile := l.profile
	if profile == "" {
		profile = os.Getenv(ProfileEnvVar)
	}
	if profile == "" {
		return nil
	}

	dir := l.profileDir
	if dir == "" {
		dir = defaultProfileDir(l.filePath)
	}

	overlayPath := filepath.Join(dir, profile+".yaml")
	if err := loadYAMLFile(cfg, overlayPath); err != nil {
		return fmt.Errorf("config: load profile %q: %w", profile, err)
	}

	return nil
}

// defaultProfileDir derives the default profile overlay directory from
// the base config file path: "<dir-of-basePath>/profiles". If basePath
// is empty, it defaults to "./profiles".
func defaultProfileDir(basePath string) string {
	if basePath == "" {
		return defaultProfileDirName
	}
	return filepath.Join(filepath.Dir(basePath), defaultProfileDirName)
}
