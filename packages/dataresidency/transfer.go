package dataresidency

import (
	"context"
	"strings"
)

// CheckTransfer guards any cross-region operation (e.g. routing a
// request to a provider hosted outside the deployment's home region,
// replicating storage, or exporting a report to another jurisdiction).
// It is the composable guard function task 3 asks for: callable before
// any cross-border data movement, independent of any specific caller
// (router, persistence, reportexport, ...).
//
// CheckTransfer returns nil if the transfer is permitted, and a
// non-nil error (wrapping one of ErrEmptyRegion, ErrStrictModeViolation,
// or ErrRegionNotAllowed) otherwise. ctx is accepted for future
// cancellation/tracing use and to match this repository's convention
// of context-first guard functions, even though the current
// implementation is pure and does not block.
func CheckTransfer(_ context.Context, sourceRegion, destRegion string, policy *ResidencyPolicy) error {
	if policy == nil {
		return ErrNilPolicy
	}
	if strings.TrimSpace(sourceRegion) == "" || strings.TrimSpace(destRegion) == "" {
		return wrapf("CheckTransfer", ErrEmptyRegion)
	}

	// StrictMode forbids ANY movement across a region boundary, even
	// between two individually-allowed regions.
	if policy.StrictMode && !strings.EqualFold(sourceRegion, destRegion) {
		return wrapf("CheckTransfer", ErrStrictModeViolation)
	}

	if !policy.AllowsRegion(destRegion) {
		return wrapf("CheckTransfer", ErrRegionNotAllowed)
	}

	return nil
}

// CheckTransferForDataClass is CheckTransfer narrowed to a specific
// data class via ResidencyPolicy.AllowedRegionsFor, so a caller moving
// a specific class of sensitive data (e.g. "pii") is checked against
// that class's tighter rule rather than the deployment-wide default.
func CheckTransferForDataClass(_ context.Context, sourceRegion, destRegion, dataClass string, policy *ResidencyPolicy) error {
	if policy == nil {
		return ErrNilPolicy
	}
	if strings.TrimSpace(sourceRegion) == "" || strings.TrimSpace(destRegion) == "" {
		return wrapf("CheckTransferForDataClass", ErrEmptyRegion)
	}
	if policy.StrictMode && !strings.EqualFold(sourceRegion, destRegion) {
		return wrapf("CheckTransferForDataClass", ErrStrictModeViolation)
	}

	allowed := policy.AllowedRegionsFor(dataClass)
	for _, r := range allowed {
		if strings.EqualFold(r, destRegion) {
			return nil
		}
	}
	return wrapf("CheckTransferForDataClass", ErrRegionNotAllowed)
}
