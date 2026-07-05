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

// dependabotBranchPattern matches the branch names Dependabot itself
// generates (e.g. dependabot/npm_and_yarn/tailwindcss-4.3.2,
// dependabot/github_actions/actions/checkout-7) when it opens an
// automated dependency-update pull request. This repository does not
// control these names -- Dependabot assigns them -- so, unlike
// phaseBranchPattern and fixBranchPattern, this is not a style this
// repository is asking contributors to follow; it is a fixed
// third-party format this policy must recognize as legitimate rather
// than reject.
var dependabotBranchPattern = regexp.MustCompile(`^dependabot/`)

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
//
// Dependabot-authored branches (dependabotBranchPattern) are also
// accepted: this repository has no say in how Dependabot names its own
// branches, so rejecting them would permanently block every automated
// dependency-update PR rather than flag a real naming violation.
func ValidateBranchName(name string) error {
	if phaseBranchPattern.MatchString(name) {
		return nil
	}
	if fixBranchPattern.MatchString(name) {
		return nil
	}
	if dependabotBranchPattern.MatchString(name) {
		return nil
	}
	return wrapf("ValidateBranchName", fmt.Errorf("%w: %q does not match phase-NNN-slug, fix-slug, or dependabot/*", ErrInvalidBranchName, name))
}

// ValidatePRCommitCount reports whether count meets MinimumCommitCount
// for branchName, returning an error wrapping ErrInsufficientCommits
// (testable via errors.Is) when it does not.
//
// This is the runnable form of CONTRIBUTING.md's "Minimum 10 atomic
// commits per phase" and .github/pull_request_template.md's "At least
// 10 atomic commits" checklist item -- wired into CI via the
// branch-policy job so an under-sized pull request fails a check
// instead of relying on a reviewer counting commits by hand.
//
// CONTRIBUTING.md frames this minimum as "per phase", and the
// repository's own history bears that out: every fix-slug pull
// request merged before this check existed (e.g.
// fix-notifications-access-check, fix-annotations-audit-permission,
// fix-keymanagement-tenant-isolation) shipped with a single commit.
// fix-slug branches are this repository's documented convention for
// small, non-phase corrective work (see fixBranchPattern and
// CONTRIBUTING.md's "Branching" section), so they are exempt from the
// phase-sized minimum rather than being held to a per-phase quota for
// work that was never phase-sized to begin with. Dependabot branches
// are exempt for the same reason: Dependabot opens one commit per
// dependency bump, never a phase-sized batch, so holding it to
// MinimumCommitCount would mean no automated dependency PR could ever
// merge. A branchName that matches neither phaseBranchPattern,
// fixBranchPattern, nor dependabotBranchPattern (already rejected by
// ValidateBranchName) is treated as non-exempt, so this function never
// silently waives the check for a malformed name.
func ValidatePRCommitCount(branchName string, count int) error {
	if fixBranchPattern.MatchString(branchName) {
		return nil
	}
	if dependabotBranchPattern.MatchString(branchName) {
		return nil
	}
	if count < MinimumCommitCount {
		return wrapf("ValidatePRCommitCount", fmt.Errorf("%w: got %d, want >= %d", ErrInsufficientCommits, count, MinimumCommitCount))
	}
	return nil
}
