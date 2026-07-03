package encryption

import (
	"fmt"
	"net/url"
	"strings"
)

// AtRestConfig declares a deployment's encryption-at-rest posture for
// its backing data store. Real disk/volume encryption (e.g. an
// encrypted EBS volume, a Postgres instance with storage-level
// encryption enabled, an encrypted managed-database offering) is
// configured entirely at the infrastructure layer -- this package has
// no way to reach past the wire and turn it on. What AssertEncryptedAtRest
// provides is a startup-time declaration check: an operator must
// explicitly set EncryptedAtRest true (normally sourced from a
// deployment-specific config profile via packages/config), or startup
// fails loudly instead of silently assuming the infra layer did the
// right thing.
type AtRestConfig struct {
	// EncryptedAtRest must be explicitly set true by the deployment to
	// declare that its backing store (database, object storage, etc.)
	// has encryption-at-rest enabled at the infrastructure layer.
	EncryptedAtRest bool

	// RequireTLSInTransit additionally requires that the connection
	// DSN itself requests an encrypted transport (e.g. Postgres
	// "sslmode=require" or stronger). This is a belt-and-suspenders
	// check layered on top of RequireTLS: RequireTLS builds the
	// *tls.Config a connection uses, while this checks that the DSN
	// driving that connection actually opted into TLS at all.
	RequireTLSInTransit bool
}

// AssertEncryptedAtRest fails closed: it returns ErrNotEncryptedAtRest
// unless cfg explicitly declares EncryptedAtRest true. Callers
// (typically service startup, alongside packages/config's Loader) are
// expected to call this once per process start against their loaded
// AtRestConfig, so a deployment that forgot to configure
// encryption-at-rest for its database fails to start rather than
// running silently unencrypted.
//
// If cfg.RequireTLSInTransit is also set, dsn is additionally checked
// for an explicit "sslmode" (or generic "ssl"/"tls") parameter
// requesting encryption; a DSN with no such parameter, or one
// requesting sslmode=disable, fails the assertion.
func AssertEncryptedAtRest(cfg AtRestConfig, dsn string) error {
	if !cfg.EncryptedAtRest {
		return fmt.Errorf("%w: set encryption.at_rest=true once the backing store's disk/volume encryption is confirmed", ErrNotEncryptedAtRest)
	}
	if cfg.RequireTLSInTransit {
		if err := assertDSNRequestsTLS(dsn); err != nil {
			return err
		}
	}
	return nil
}

// assertDSNRequestsTLS checks that dsn (a Postgres-style connection
// string or URL) requests an encrypted connection via sslmode (or the
// generic ssl/tls parameter some drivers use). Accepted sslmode values
// are anything other than "disable" and "allow" (both of which permit
// a silent downgrade to plaintext).
func assertDSNRequestsTLS(dsn string) error {
	if dsn == "" {
		return fmt.Errorf("encryption: dsn is required to verify in-transit encryption")
	}

	params := parseDSNParams(dsn)

	mode := firstNonEmpty(params["sslmode"], params["ssl"], params["tls"])
	if mode == "" {
		return fmt.Errorf("encryption: dsn does not specify sslmode (or ssl/tls); refusing to assume an encrypted connection")
	}

	switch strings.ToLower(mode) {
	case "disable", "allow", "false", "off":
		return fmt.Errorf("encryption: dsn requests sslmode=%s, which permits an unencrypted connection", mode)
	default:
		return nil
	}
}

// parseDSNParams extracts query-style key/value parameters from a
// Postgres DSN, which may be either a "key=value key=value ..."
// keyword/value string or a "postgres://...?key=value&..." URL. It
// returns an empty map (not an error) if dsn matches neither shape
// cleanly, since callers only use the result to look up a couple of
// optional parameters.
func parseDSNParams(dsn string) map[string]string {
	if u, err := url.Parse(dsn); err == nil && u.Scheme != "" && u.RawQuery != "" {
		out := make(map[string]string, len(u.Query()))
		for k, v := range u.Query() {
			if len(v) > 0 {
				out[strings.ToLower(k)] = v[0]
			}
		}
		return out
	}

	out := make(map[string]string)
	for _, field := range strings.Fields(dsn) {
		kv := strings.SplitN(field, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(kv[0]))
		val := strings.Trim(strings.TrimSpace(kv[1]), `'"`)
		out[key] = val
	}
	return out
}

// firstNonEmpty returns the first non-empty string in vals, or "".
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
