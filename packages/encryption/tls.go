package encryption

import (
	"crypto/tls"
	"fmt"
)

// MinSupportedTLSVersion is the lowest TLS protocol version RequireTLS
// ever permits, regardless of caller-requested minimum. TLS 1.0 and
// 1.1 are never acceptable for Verdex service traffic.
const MinSupportedTLSVersion = tls.VersionTLS12

// approvedCipherSuites is the restricted list of modern,
// forward-secret cipher suites RequireTLS configures for any TLS
// connection negotiated at 1.2 (TLS 1.3's cipher suites are fixed by
// the standard library and are not configurable, nor do they need to
// be -- every TLS 1.3 suite is already AEAD and forward-secret). Every
// entry here is an ECDHE (forward secret) + AEAD (AES-GCM or
// ChaCha20-Poly1305) suite; no CBC-mode or RSA-key-exchange suite is
// included.
var approvedCipherSuites = []uint16{
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
	tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
}

// TLSOption customizes the *tls.Config RequireTLS builds.
type TLSOption func(*tls.Config)

// WithMinVersion overrides the minimum TLS version RequireTLS
// configures. Any version below MinSupportedTLSVersion (TLS 1.2) is
// rejected -- RequireTLS clamps up to MinSupportedTLSVersion rather
// than honoring a weaker request, since silently downgrading transport
// security below the floor this package enforces would defeat the
// point of calling RequireTLS at all.
func WithMinVersion(version uint16) TLSOption {
	return func(cfg *tls.Config) {
		if version < MinSupportedTLSVersion {
			version = MinSupportedTLSVersion
		}
		cfg.MinVersion = version
	}
}

// WithServerName sets the SNI/verification server name for a client
// config built by RequireTLS.
func WithServerName(name string) TLSOption {
	return func(cfg *tls.Config) {
		cfg.ServerName = name
	}
}

// RequireTLS builds a *tls.Config enforcing Verdex's minimum
// transport-security floor: TLS 1.2 minimum (TLS 1.3 preferred, and
// negotiated automatically whenever both peers support it), a
// restricted forward-secret/AEAD cipher suite list for TLS 1.2
// connections, and certificate verification always enabled
// (InsecureSkipVerify is never set true by this constructor -- there
// is no option to disable it, by design).
//
// Every Verdex HTTP server or client, and packages/persistence's
// Postgres connection, should construct its *tls.Config through
// RequireTLS rather than assembling one by hand, so a single place
// defines the enforced floor and a future tightening (e.g. dropping
// TLS 1.2 entirely) only needs to change here.
func RequireTLS(opts ...TLSOption) *tls.Config {
	cfg := &tls.Config{
		MinVersion:   MinSupportedTLSVersion,
		CipherSuites: approvedCipherSuites,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

// ValidateTLSConfig checks that cfg meets Verdex's minimum transport
// floor: MinVersion at or above MinSupportedTLSVersion, and (for
// configs that pin to TLS 1.2 specifically) a CipherSuites list drawn
// only from approvedCipherSuites. A cfg with MinVersion at or above
// TLS 1.3 is accepted regardless of CipherSuites, since TLS 1.3's
// suite list is fixed by the standard library and not attacker- or
// operator-weakenable.
func ValidateTLSConfig(cfg *tls.Config) error {
	if cfg == nil {
		return fmt.Errorf("encryption: tls config is required")
	}
	if cfg.InsecureSkipVerify {
		return fmt.Errorf("encryption: tls config must not set InsecureSkipVerify")
	}
	if cfg.MinVersion < MinSupportedTLSVersion {
		return fmt.Errorf("encryption: tls MinVersion %#x is below required minimum %#x (TLS 1.2)", cfg.MinVersion, MinSupportedTLSVersion)
	}
	if cfg.MinVersion >= tls.VersionTLS13 {
		return nil
	}
	if len(cfg.CipherSuites) == 0 {
		return fmt.Errorf("encryption: tls config targeting TLS 1.2 must set an explicit cipher suite list")
	}
	approved := make(map[uint16]bool, len(approvedCipherSuites))
	for _, suite := range approvedCipherSuites {
		approved[suite] = true
	}
	for _, suite := range cfg.CipherSuites {
		if !approved[suite] {
			return fmt.Errorf("encryption: tls cipher suite %#x is not in the approved forward-secret/AEAD list", suite)
		}
	}
	return nil
}
