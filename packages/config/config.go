// Package config provides typed, layered configuration for Verdex services.
//
// Configuration is assembled from multiple layers, applied in increasing
// order of precedence: built-in defaults, an optional YAML file, an
// optional named profile overlay, and environment variables. See the
// package README for the full precedence and usage documentation.
package config

import "time"

// Config is the root configuration schema shared by all Verdex services.
// Every Verdex service loads a Config (directly or embedded in a
// service-specific superset) via the Loader in this package.
type Config struct {
	// Deployment describes the identity of this running instance.
	Deployment DeploymentConfig `yaml:"deployment"`

	// Server holds HTTP server settings.
	Server ServerConfig `yaml:"server"`

	// Database holds database connectivity settings.
	Database DatabaseConfig `yaml:"database"`

	// Observability holds logging/metrics/tracing settings.
	Observability ObservabilityConfig `yaml:"observability"`

	// Provider is a placeholder for future model/LLM provider
	// configuration. It intentionally carries no model-specific fields
	// in this phase; Phase 11 is responsible for populating it. No
	// Verdex code may hardcode a model provider (see CONTRIBUTING.md).
	Provider ProviderConfig `yaml:"provider"`
}

// DeploymentConfig identifies the running deployment.
type DeploymentConfig struct {
	// Name is a human-readable identifier for this deployment, e.g.
	// "verdex-api" or "verdex-worker".
	Name string `yaml:"name"`

	// Environment is the deployment environment, e.g. "development",
	// "sandbox", "production", or "airgapped".
	Environment string `yaml:"environment"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	// Host is the address the HTTP server binds to.
	Host string `yaml:"host"`

	// Port is the TCP port the HTTP server listens on.
	Port int `yaml:"port"`

	// ReadTimeout bounds the time allowed to read an entire request,
	// including the body.
	ReadTimeout time.Duration `yaml:"read_timeout"`

	// WriteTimeout bounds the time allowed to write a response.
	WriteTimeout time.Duration `yaml:"write_timeout"`

	// IdleTimeout bounds how long to keep idle keep-alive connections
	// open.
	IdleTimeout time.Duration `yaml:"idle_timeout"`

	// ShutdownTimeout bounds how long graceful shutdown waits for
	// in-flight requests to finish before forcing close.
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

// DatabaseConfig holds database connectivity settings.
type DatabaseConfig struct {
	// DSN is the database connection string. It may be a literal value
	// or a secret reference (see secrets.go) such as
	// "env://VERDEX_DATABASE_DSN". It is redacted whenever the config
	// is logged or printed.
	DSN string `yaml:"dsn" redact:"true"`

	// MaxOpenConns is the maximum number of open connections to the
	// database.
	MaxOpenConns int `yaml:"max_open_conns"`

	// MaxIdleConns is the maximum number of idle connections retained
	// in the pool.
	MaxIdleConns int `yaml:"max_idle_conns"`

	// ConnMaxLifetime bounds how long a connection may be reused.
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`

	// RequireTLS, when true, requires DSN to request an encrypted
	// connection (e.g. Postgres sslmode=require or stronger) before
	// persistence.Open will use it. It defaults to false so that
	// local-development and test DSNs (which commonly use
	// sslmode=disable against a loopback database) keep working
	// without every caller needing to opt out; production and any
	// deployment profile handling real data is expected to set this
	// true. See packages/encryption's AssertEncryptedAtRest, which
	// persistence.Open delegates this check to.
	RequireTLS bool `yaml:"require_tls"`
}

// ObservabilityConfig holds logging, metrics, and tracing settings.
type ObservabilityConfig struct {
	// LogLevel is the minimum severity logged, e.g. "debug", "info",
	// "warn", or "error".
	LogLevel string `yaml:"log_level"`

	// LogFormat is the structured log encoding, e.g. "json" or
	// "console".
	LogFormat string `yaml:"log_format"`
}

// ProviderConfig is an intentionally empty placeholder for future
// model/LLM provider configuration. Do not add provider- or
// model-specific fields here in this phase; that work belongs to the
// phase that introduces packages/provider's LLMProvider interface.
type ProviderConfig struct{}

// Default returns a Config populated with sane, non-secret default
// values suitable for local development. Default is the first and
// lowest-precedence layer applied by Loader.
func Default() Config {
	return Config{
		Deployment: DeploymentConfig{
			Name:        "verdex",
			Environment: "development",
		},
		Server: ServerConfig{
			Host:            "0.0.0.0",
			Port:            8080,
			ReadTimeout:     5 * time.Second,
			WriteTimeout:    10 * time.Second,
			IdleTimeout:     120 * time.Second,
			ShutdownTimeout: 15 * time.Second,
		},
		Database: DatabaseConfig{
			DSN:             "",
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: 30 * time.Minute,
			RequireTLS:      false,
		},
		Observability: ObservabilityConfig{
			LogLevel:  "info",
			LogFormat: "json",
		},
		Provider: ProviderConfig{},
	}
}
