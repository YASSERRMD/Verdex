package config

// Loader assembles a Config from layered sources. Layers are applied in
// increasing order of precedence:
//
//  1. Default() — built-in, hardcoded defaults.
//  2. YAML file — optional, set via WithFile.
//  3. Environment variables — always considered; VERDEX_-prefixed vars
//     win over anything set by an earlier layer.
//
// Each layer mutates only the fields it explicitly sets, so a value
// established by an earlier layer survives untouched if a later layer
// doesn't mention it.
type Loader struct {
	filePath string
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

// NewLoader builds a Loader configured with opts.
func NewLoader(opts ...Option) *Loader {
	l := &Loader{}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// Load runs all configured layers in precedence order and returns the
// final merged Config. The returned error wraps the originating
// failure (file read/parse or env parse).
func (l *Loader) Load() (Config, error) {
	cfg := Default()

	if err := loadYAMLFile(&cfg, l.filePath); err != nil {
		return Config{}, err
	}

	if err := loadEnv(&cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}
