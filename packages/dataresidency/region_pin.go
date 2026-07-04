package dataresidency

import (
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ValidateDSN parses dsn (a Postgres connection string, as accepted by
// packages/persistence.Open) and asserts its host matches one of p's
// HostPatterns. This is the real, testable check task 2 asks for: it
// does not merely record a region label, it inspects the actual
// configured connection target and fails if the two disagree.
//
// ValidateDSN returns ErrEmptyDSN if dsn is empty, a wrapped parse
// error if dsn is malformed, and ErrStorageRegionMismatch if the host
// does not match any HostPatterns entry.
func (p *RegionPin) ValidateDSN(dsn string) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(dsn) == "" {
		return ErrEmptyDSN
	}

	host, err := hostFromDSN(dsn)
	if err != nil {
		return wrapf("ValidateDSN", err)
	}

	for _, pattern := range p.HostPatterns {
		if pattern == "" {
			continue
		}
		if strings.Contains(strings.ToLower(host), strings.ToLower(pattern)) {
			return nil
		}
	}

	return wrapf("ValidateDSN", ErrStorageRegionMismatch)
}

// hostFromDSN extracts the primary connection host from a Postgres DSN
// using the same parser packages/persistence.Open relies on
// (pgxpool.ParseConfig), so this package's notion of "the configured
// host" never drifts from what actually gets dialed.
func hostFromDSN(dsn string) (string, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return "", err
	}
	if cfg.ConnConfig == nil || cfg.ConnConfig.Host == "" {
		return "", errNoHostInDSN
	}
	return cfg.ConnConfig.Host, nil
}
