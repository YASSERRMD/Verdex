package config

import (
	"errors"
	"fmt"
)

// validLogLevels enumerates the log levels accepted by Validate.
var validLogLevels = map[string]bool{
	"debug": true,
	"info":  true,
	"warn":  true,
	"error": true,
}

// validLogFormats enumerates the log formats accepted by Validate.
var validLogFormats = map[string]bool{
	"json":    true,
	"console": true,
}

// Validate checks cfg for missing required fields, out-of-range
// values, and internally inconsistent settings. It returns a single
// error joining every violation found (via errors.Join), or nil if cfg
// is valid. Loader.Load calls Validate as its final step and wraps any
// failure with additional context.
func (c Config) Validate() error {
	var errs []error

	if c.Deployment.Name == "" {
		errs = append(errs, errors.New("deployment.name must not be empty"))
	}
	if c.Deployment.Environment == "" {
		errs = append(errs, errors.New("deployment.environment must not be empty"))
	}

	if c.Server.Host == "" {
		errs = append(errs, errors.New("server.host must not be empty"))
	}
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		errs = append(errs, fmt.Errorf("server.port must be in range [1, 65535], got %d", c.Server.Port))
	}
	if c.Server.ReadTimeout <= 0 {
		errs = append(errs, fmt.Errorf("server.read_timeout must be positive, got %s", c.Server.ReadTimeout))
	}
	if c.Server.WriteTimeout <= 0 {
		errs = append(errs, fmt.Errorf("server.write_timeout must be positive, got %s", c.Server.WriteTimeout))
	}
	if c.Server.IdleTimeout <= 0 {
		errs = append(errs, fmt.Errorf("server.idle_timeout must be positive, got %s", c.Server.IdleTimeout))
	}
	if c.Server.ShutdownTimeout <= 0 {
		errs = append(errs, fmt.Errorf("server.shutdown_timeout must be positive, got %s", c.Server.ShutdownTimeout))
	}

	if c.Database.MaxOpenConns < 1 {
		errs = append(errs, fmt.Errorf("database.max_open_conns must be at least 1, got %d", c.Database.MaxOpenConns))
	}
	if c.Database.MaxIdleConns < 0 {
		errs = append(errs, fmt.Errorf("database.max_idle_conns must not be negative, got %d", c.Database.MaxIdleConns))
	}
	if c.Database.MaxIdleConns > c.Database.MaxOpenConns {
		errs = append(errs, fmt.Errorf("database.max_idle_conns (%d) must not exceed database.max_open_conns (%d)", c.Database.MaxIdleConns, c.Database.MaxOpenConns))
	}
	if c.Database.ConnMaxLifetime < 0 {
		errs = append(errs, fmt.Errorf("database.conn_max_lifetime must not be negative, got %s", c.Database.ConnMaxLifetime))
	}

	if !validLogLevels[c.Observability.LogLevel] {
		errs = append(errs, fmt.Errorf("observability.log_level must be one of debug|info|warn|error, got %q", c.Observability.LogLevel))
	}
	if !validLogFormats[c.Observability.LogFormat] {
		errs = append(errs, fmt.Errorf("observability.log_format must be one of json|console, got %q", c.Observability.LogFormat))
	}

	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("config: validation failed: %w", errors.Join(errs...))
}
