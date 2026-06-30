package config

import "fmt"

// Loader assembles a Config from layered sources. Layers are applied in
// increasing order of precedence:
//
//  1. Default() — built-in, hardcoded defaults.
//  2. YAML file — optional, set via WithFile.
//  3. Profile overlay — optional, selected via WithProfile or the
//     VERDEX_PROFILE environment variable; see profile.go.
//  4. Environment variables — always considered; VERDEX_-prefixed vars
//     win over anything set by an earlier layer.
//
// Each layer mutates only the fields it explicitly sets, so a value
// established by an earlier layer survives untouched if a later layer
// doesn't mention it. After all layers are merged, any field holding a
// secret reference (env:// or vault://) is resolved to its plaintext
// value; see secrets.go.
type Loader struct {
	filePath       string
	profile        string
	profileDir     string
	secretResolver SecretResolver
}

// Option configures a Loader.
type Option func(*Loader)

// WithFile sets the path to an optional base YAML config file. If
// path is empty (the default), the YAML layer is skipped.
func WithFile(path string) Option {
	return func(l *Loader) {
		l.filePath = path
	}
}

// WithProfile explicitly selects a named profile (e.g. "sandbox",
// "production", "airgapped"), taking precedence over whatever
// VERDEX_PROFILE is set to. See profile.go for profile resolution and
// layering rules.
func WithProfile(name string) Option {
	return func(l *Loader) {
		l.profile = name
	}
}

// WithProfileDir overrides the directory profile overlay files are
// read from. It defaults to a "profiles" directory next to the base
// config file (or "./profiles" if no base file is configured); see
// profile.go.
func WithProfileDir(dir string) Option {
	return func(l *Loader) {
		l.profileDir = dir
	}
}

// WithSecretResolver overrides the SecretResolver used to resolve
// secret references after merge. The default is NewDefaultResolver().
func WithSecretResolver(r SecretResolver) Option {
	return func(l *Loader) {
		l.secretResolver = r
	}
}

// NewLoader builds a Loader configured with opts.
func NewLoader(opts ...Option) *Loader {
	l := &Loader{
		secretResolver: NewDefaultResolver(),
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// Load runs all configured layers in precedence order, resolves secret
// references, validates the result, and returns the final merged
// Config. The returned error wraps the originating failure (file
// read/parse, profile load, env parse, secret resolution, or
// validation).
func (l *Loader) Load() (Config, error) {
	cfg := Default()

	if err := loadYAMLFile(&cfg, l.filePath); err != nil {
		return Config{}, err
	}

	if err := applyProfile(&cfg, l); err != nil {
		return Config{}, err
	}

	if err := loadEnv(&cfg); err != nil {
		return Config{}, err
	}

	if err := resolveSecrets(&cfg, l.secretResolver); err != nil {
		return Config{}, err
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}

	return cfg, nil
}
