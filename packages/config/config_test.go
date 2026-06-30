package config

import (
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.Deployment.Name != "verdex" {
		t.Errorf("Deployment.Name = %q, want %q", cfg.Deployment.Name, "verdex")
	}
	if cfg.Deployment.Environment != "development" {
		t.Errorf("Deployment.Environment = %q, want %q", cfg.Deployment.Environment, "development")
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, "0.0.0.0")
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, 8080)
	}
	if cfg.Server.ReadTimeout != 5*time.Second {
		t.Errorf("Server.ReadTimeout = %v, want %v", cfg.Server.ReadTimeout, 5*time.Second)
	}
	if cfg.Database.MaxOpenConns != 10 {
		t.Errorf("Database.MaxOpenConns = %d, want %d", cfg.Database.MaxOpenConns, 10)
	}
	if cfg.Observability.LogLevel != "info" {
		t.Errorf("Observability.LogLevel = %q, want %q", cfg.Observability.LogLevel, "info")
	}
	if cfg.Observability.LogFormat != "json" {
		t.Errorf("Observability.LogFormat = %q, want %q", cfg.Observability.LogFormat, "json")
	}
}

func TestDefaultIsIndependentCopy(t *testing.T) {
	a := Default()
	b := Default()

	a.Deployment.Name = "mutated"

	if b.Deployment.Name == "mutated" {
		t.Fatal("Default() returned a shared value; mutating one copy affected another")
	}
}
