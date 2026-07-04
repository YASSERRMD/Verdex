package garelease

import (
	"github.com/YASSERRMD/verdex/packages/cicdgate"
)

// BuildReleaseNotes is task 10's "release notes automation" -- real
// composition with packages/cicdgate.GenerateReleaseNotes (Phase 095),
// not a reimplementation. commits is the full commit list spanning this
// release (e.g. every commit reachable from the release's CommitSHA
// back to the previous release's CommitSHA), in cicdgate.CommitInfo's
// exact shape (ShortHash, Subject).
//
// This function is a one-line pass-through by design: cicdgate.GenerateReleaseNotes
// already groups commits by this repository's real imperative-verb
// convention (Add/Fix/Remove/Document/etc, surveyed from actual git
// history) into the exact markdown section structure this platform's
// release notes use. garelease's only job for task 10 is to call it
// with this release's commit list -- see doc/ga-release.md for the
// worked example this package's own release used, and CHANGELOG.md for
// the human-authored, high-level (8-part-plan) summary this function's
// output complements rather than replaces.
func BuildReleaseNotes(commits []cicdgate.CommitInfo) string {
	return cicdgate.GenerateReleaseNotes(commits)
}
