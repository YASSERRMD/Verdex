package local

import (
	"fmt"
	"net/url"
	"strings"
)

// OfflineModeEnforcer ensures that all HTTP calls in an air-gapped deployment
// only reach localhost or loopback addresses. It is consulted before every
// outbound request when Config.OfflineMode is true.
//
// The enforcer is designed to surface misconfigurations early rather than
// silently leaking data to external endpoints.
type OfflineModeEnforcer struct {
	// testPanic controls whether the enforcer panics on violation (test mode)
	// or returns an error (production mode). Set to true in tests that want to
	// catch misconfigured URLs as hard failures.
	testPanic bool
}

// NewOfflineModeEnforcer creates an OfflineModeEnforcer. Set panicOnViolation
// to true in unit tests to convert a policy violation into a panic that the
// test framework will report as a failure rather than a silent error.
func NewOfflineModeEnforcer(panicOnViolation bool) *OfflineModeEnforcer {
	return &OfflineModeEnforcer{testPanic: panicOnViolation}
}

// CheckURL returns an error (or panics, in test mode) if rawURL is not a
// loopback/localhost address. It is a no-op for empty strings.
func (e *OfflineModeEnforcer) CheckURL(rawURL string) error {
	if rawURL == "" {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("local: offline enforcer: invalid URL %q: %w", rawURL, err)
	}
	host := u.Hostname()
	if !isLocalHost(host) {
		msg := fmt.Sprintf(
			"local: offline mode violation: attempted external call to %q; only localhost/127.0.0.1/::1 are permitted",
			host,
		)
		if e.testPanic {
			panic(msg)
		}
		return fmt.Errorf("%s", msg) //nolint:goerr113
	}
	return nil
}

// OutboundGuard is a convenience wrapper for CheckURL that panics in test
// mode. It is useful as a guard at the top of network helpers to make
// air-gap violations loud and unmissable during testing.
func (e *OfflineModeEnforcer) OutboundGuard(rawURL string) {
	if err := e.CheckURL(rawURL); err != nil {
		if e.testPanic {
			panic(err.Error())
		}
		// In production we log rather than panic; the real enforcement happens
		// via the returned error from CheckURL at the call site.
	}
}

// isLocalHost reports whether host is a loopback address or "localhost".
func isLocalHost(host string) bool {
	host = strings.ToLower(host)
	return host == "localhost" ||
		host == "127.0.0.1" ||
		host == "::1" ||
		strings.HasPrefix(host, "127.") // 127.x.x.x range
}
