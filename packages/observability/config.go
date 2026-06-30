package observability

import (
	"fmt"

	"github.com/YASSERRMD/verdex/packages/config"
)

// NewLoggerFromConfig builds a Logger from cfg.Observability,
// resolving cfg.Observability.LogLevel and cfg.Observability.LogFormat
// via ParseLevel and ParseFormat respectively. Additional opts are
// applied after the config-derived level/format, so a caller can still
// override the output destination (e.g. WithOutput for tests) without
// fighting the config-driven defaults.
//
// cfg is expected to have already passed config.Config.Validate(),
// which constrains LogLevel and LogFormat to the values ParseLevel and
// ParseFormat accept; NewLoggerFromConfig still returns an error
// rather than panicking if an unvalidated Config is passed in.
func NewLoggerFromConfig(cfg *config.Config, opts ...Option) (*Logger, error) {
	if cfg == nil {
		return nil, fmt.Errorf("observability: NewLoggerFromConfig: cfg must not be nil")
	}

	level, err := ParseLevel(cfg.Observability.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("observability: NewLoggerFromConfig: %w", err)
	}

	format, err := ParseFormat(cfg.Observability.LogFormat)
	if err != nil {
		return nil, fmt.Errorf("observability: NewLoggerFromConfig: %w", err)
	}

	allOpts := append([]Option{WithLevel(level), WithFormat(format)}, opts...)
	return New(allOpts...), nil
}
