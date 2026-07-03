package encryption_test

import (
	"crypto/tls"
	"testing"

	"github.com/YASSERRMD/verdex/packages/encryption"
)

func TestRequireTLS_DefaultsMeetFloor(t *testing.T) {
	cfg := encryption.RequireTLS()

	if err := encryption.ValidateTLSConfig(cfg); err != nil {
		t.Fatalf("ValidateTLSConfig(RequireTLS()) error = %v, want nil", err)
	}
	if cfg.MinVersion < encryption.MinSupportedTLSVersion {
		t.Fatalf("RequireTLS().MinVersion = %#x, want >= %#x", cfg.MinVersion, encryption.MinSupportedTLSVersion)
	}
	if cfg.InsecureSkipVerify {
		t.Fatal("RequireTLS() must never set InsecureSkipVerify")
	}
}

func TestRequireTLS_WithMinVersionClampsWeakRequest(t *testing.T) {
	cfg := encryption.RequireTLS(encryption.WithMinVersion(tls.VersionTLS10))

	if cfg.MinVersion < encryption.MinSupportedTLSVersion {
		t.Fatalf("RequireTLS(WithMinVersion(TLS1.0)).MinVersion = %#x, want clamped to >= %#x", cfg.MinVersion, encryption.MinSupportedTLSVersion)
	}
}

func TestRequireTLS_WithMinVersionAllowsStrongerFloor(t *testing.T) {
	cfg := encryption.RequireTLS(encryption.WithMinVersion(tls.VersionTLS13))

	if cfg.MinVersion != tls.VersionTLS13 {
		t.Fatalf("RequireTLS(WithMinVersion(TLS1.3)).MinVersion = %#x, want %#x", cfg.MinVersion, tls.VersionTLS13)
	}
	if err := encryption.ValidateTLSConfig(cfg); err != nil {
		t.Fatalf("ValidateTLSConfig() error = %v, want nil", err)
	}
}

func TestRequireTLS_WithServerName(t *testing.T) {
	cfg := encryption.RequireTLS(encryption.WithServerName("db.internal.verdex.example"))
	if cfg.ServerName != "db.internal.verdex.example" {
		t.Fatalf("RequireTLS(WithServerName(...)).ServerName = %q, want %q", cfg.ServerName, "db.internal.verdex.example")
	}
}

func TestValidateTLSConfig_RejectsWeakVersion(t *testing.T) {
	cfg := &tls.Config{MinVersion: tls.VersionTLS11}
	if err := encryption.ValidateTLSConfig(cfg); err == nil {
		t.Fatal("ValidateTLSConfig() error = nil, want error for TLS 1.1 MinVersion")
	}
}

func TestValidateTLSConfig_RejectsInsecureSkipVerify(t *testing.T) {
	cfg := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		CipherSuites:       []uint16{tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384},
		InsecureSkipVerify: true,
	}
	if err := encryption.ValidateTLSConfig(cfg); err == nil {
		t.Fatal("ValidateTLSConfig() error = nil, want error when InsecureSkipVerify is true")
	}
}

func TestValidateTLSConfig_RejectsWeakCipherSuite(t *testing.T) {
	cfg := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		CipherSuites: []uint16{tls.TLS_RSA_WITH_AES_128_CBC_SHA}, // no forward secrecy, CBC mode
	}
	if err := encryption.ValidateTLSConfig(cfg); err == nil {
		t.Fatal("ValidateTLSConfig() error = nil, want error for a non-forward-secret CBC cipher suite")
	}
}

func TestValidateTLSConfig_RejectsMissingCipherSuitesAtTLS12(t *testing.T) {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if err := encryption.ValidateTLSConfig(cfg); err == nil {
		t.Fatal("ValidateTLSConfig() error = nil, want error when TLS1.2 config has no explicit cipher suite list")
	}
}

func TestValidateTLSConfig_NilConfig(t *testing.T) {
	if err := encryption.ValidateTLSConfig(nil); err == nil {
		t.Fatal("ValidateTLSConfig(nil) error = nil, want error")
	}
}

func TestValidateTLSConfig_TLS13SkipsCipherSuiteCheck(t *testing.T) {
	cfg := &tls.Config{MinVersion: tls.VersionTLS13}
	if err := encryption.ValidateTLSConfig(cfg); err != nil {
		t.Fatalf("ValidateTLSConfig() error = %v, want nil for TLS1.3-only config with no explicit cipher suites", err)
	}
}
