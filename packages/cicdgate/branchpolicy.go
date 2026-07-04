package cicdgate

import (
	"fmt"
	"regexp"
)

// MinimumCommitCount is the minimum number of commits a pull request
// must contain before it may merge, per CONTRIBUTING.md's "Minimum 10
// atomic commits per phase" and .github/pull_request_template.md's
// "At least 10 atomic commits" checklist item.
const MinimumCommitCount = 10

// phaseBranchPattern matches this repository's primary branch-naming
// convention, documented in CONTRIBUTING.md's "Branching" section:
// "One branch per phase, named phase-NNN-short-slug (e.g.
// phase-007-jurisdiction-loader)". NNN is at least two digits (this
// repository is already past phase 91, so a fixed 3-digit width would
// reject legitimate historical branches like phase-001); slug is one
// or more lowercase alphanumeric segments joined by hyphens.
var phaseBranchPattern = regexp.MustCompile(`^phase-[0-9]{2,}-[a-z0-9]+(-[a-z0-9]+)*$`)

// fixBranchPattern matches this repository's established (if not
// explicitly written down in CONTRIBUTING.md) convention for small,
// non-phase corrective work, reflected in this repository's own git
// history (branches such as fix-notifications-access-check,
// fix-annotations-audit-permission, fix-keymanagement-tenant-isolation):
// fix-<slug>.
var fixBranchPattern = regexp.MustCompile(`^fix-[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidateBranchName reports whether name conforms to this
// repository's branch-naming policy: either the phase-NNN-slug form
// (CONTRIBUTING.md) or the fix-slug form for small corrective changes.
// It returns an error wrapping ErrInvalidBranchName (testable via
// errors.Is) when neither pattern matches.
//
// This is the runnable form of the policy documented in
// CONTRIBUTING.md and .github/pull_request_template.md's "Branch is
// named phase-NNN-short-slug" checklist item -- wired into CI via the
// branch-policy job in .github/workflows/ci.yml so a malformed branch
// name fails a pull request check instead of relying on a reviewer
// remembering to look.
func ValidateBranchName(name string) error {
	if phaseBranchPattern.MatchString(name) {
		return nil
	}
	if fixBranchPattern.MatchString(name) {
		return nil
	}
	return wrapf("ValidateBranchName", fmt.Errorf("%w: %q does not match phase-NNN-slug or fix-slug", ErrInvalidBranchName, name))
}

// ValidatePRCommitCount reports whether count meets
// MinimumCommitCount. It returns an error wrapping
// ErrInsufficientCommits (testable via errors.Is) when count is below
// the minimum.
//
// This is the runnable form of CONTRIBUTING.md's "Minimum 10 atomic
// commits per phase" and .github/pull_request_template.md's "At least
// 10 atomic commits" checklist item -- wired into CI via the
// branch-policy job so an under-sized pull request fails a check
// instead of relying on a reviewer counting commits by hand.
func ValidatePRCommitCount(count int) error {
	if count < MinimumCommitCount {
		return wrapf("ValidatePRCommitCount", fmt.Errorf("%w: got %d, want >= %d", ErrInsufficientCommits, count, MinimumCommitCount))
	}
	return nil
}
