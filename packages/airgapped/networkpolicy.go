package airgapped

import (
	"context"
	"net"
	"net/url"
	"strings"
	"time"
)

// NetworkPolicy is a real, testable allow-list guard (task 3) for
// every outbound network target an air-gapped deployment might
// attempt to reach. It is deliberately independent of
// packages/adapters/local's OfflineModeEnforcer (which checks only the
// local adapter's own configured BaseURL): NetworkPolicy is meant to be
// consulted by any caller anywhere in a deployment -- not just the
// local model adapter -- before dialing out, and it also accepts an
// explicit allow-list of additional non-loopback targets (e.g. a local
// model server reachable over a private LAN address rather than
// localhost), which OfflineModeEnforcer does not support.
type NetworkPolicy struct {
	// AllowedTargets lists additional host (or host:port) values that
	// are permitted beyond loopback/localhost. A target matches if its
	// host equals, or its host:port equals, an entry in this list.
	AllowedTargets []string
}

// NewNetworkPolicy builds a NetworkPolicy from a Profile's
// AllowedNetworkTargets.
func NewNetworkPolicy(profile *Profile) (*NetworkPolicy, error) {
	if profile == nil {
		return nil, ErrNilProfile
	}
	return &NetworkPolicy{AllowedTargets: append([]string(nil), profile.AllowedNetworkTargets...)}, nil
}

// CheckAddress returns nil if addr (a "host", "host:port", or full URL
// string) resolves to loopback or an explicitly allow-listed target,
// and ErrDisallowedAddress otherwise. It never performs a DNS lookup or
// dials anything itself -- it is a pure, offline, testable string/IP
// check, matching the rest of this phase's "no network call" posture.
func (p *NetworkPolicy) CheckAddress(addr string) error {
	if addr == "" {
		return ErrEmptyAddress
	}
	host, hostport := normalizeAddress(addr)

	if isLoopbackHost(host) {
		return nil
	}
	for _, allowed := range p.AllowedTargets {
		allowedHost, allowedHostPort := normalizeAddress(allowed)
		if host == allowedHost || hostport == allowedHostPort || hostport == allowedHost || host == allowedHostPort {
			return nil
		}
	}
	return wrapf("CheckAddress", ErrDisallowedAddress)
}

// normalizeAddress extracts a bare host and a host:port form from addr,
// which may be a full URL ("http://127.0.0.1:11434/v1"), a bare
// "host:port" pair, or a bare host/IP.
func normalizeAddress(addr string) (host string, hostport string) {
	if u, err := url.Parse(addr); err == nil && u.Host != "" {
		addr = u.Host
	}
	h, _, err := net.SplitHostPort(addr)
	if err != nil {
		// addr has no port component.
		return strings.ToLower(strings.Trim(addr, "[]")), strings.ToLower(strings.Trim(addr, "[]"))
	}
	return strings.ToLower(strings.Trim(h, "[]")), strings.ToLower(addr)
}

// isLoopbackHost reports whether host is "localhost" or an IP in the
// loopback range (127.0.0.0/8 or ::1).
func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

// DialContextFunc matches the signature of net.Dialer.DialContext, so
// GuardedDialContext can wrap any caller's dialer.
type DialContextFunc func(ctx context.Context, network, address string) (net.Conn, error)

// GuardedDialContext wraps next (typically (*net.Dialer).DialContext)
// so that every dial attempt is checked against p.CheckAddress first
// -- the "real, testable guard function... a dialer wrapper" task 3
// asks for. A disallowed address never reaches next at all.
func GuardedDialContext(p *NetworkPolicy, next DialContextFunc) DialContextFunc {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		if err := p.CheckAddress(address); err != nil {
			return nil, err
		}
		return next(ctx, network, address)
	}
}

// DefaultDialTimeout is the dial timeout used by NewGuardedDialer.
const DefaultDialTimeout = 10 * time.Second

// NewGuardedDialer returns a DialContextFunc backed by a real
// net.Dialer, wrapped with p's allow-list guard, ready to be assigned
// to an http.Transport.DialContext (or any other caller that dials by
// address) so every outbound TCP connection attempt in the process is
// enforced, not just calls that happen to remember to check first.
func NewGuardedDialer(p *NetworkPolicy) DialContextFunc {
	d := &net.Dialer{Timeout: DefaultDialTimeout}
	return GuardedDialContext(p, d.DialContext)
}
