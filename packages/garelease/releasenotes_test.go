package garelease_test

import (
	"strings"
	"testing"

	"github.com/YASSERRMD/verdex/packages/cicdgate"
	"github.com/YASSERRMD/verdex/packages/garelease"
)

func TestBuildReleaseNotes_GroupsByImperativeVerb(t *testing.T) {
	commits := []cicdgate.CommitInfo{
		{ShortHash: "aaa1111", Subject: "Add release-readiness engine"},
		{ShortHash: "bbb2222", Subject: "Fix guardrail fixture wording"},
		{ShortHash: "ccc3333", Subject: "Document the eight-part plan"},
		{ShortHash: "ddd4444", Subject: "Merge pull request #123 from feature-branch"},
	}

	notes := garelease.BuildReleaseNotes(commits)

	if !strings.Contains(notes, "## Release Notes") {
		t.Fatalf("BuildReleaseNotes output missing '## Release Notes' heading:\n%s", notes)
	}
	if !strings.Contains(notes, "### Added") || !strings.Contains(notes, "Add release-readiness engine") {
		t.Errorf("BuildReleaseNotes output missing Added section entry:\n%s", notes)
	}
	if !strings.Contains(notes, "### Fixed") || !strings.Contains(notes, "Fix guardrail fixture wording") {
		t.Errorf("BuildReleaseNotes output missing Fixed section entry:\n%s", notes)
	}
	if !strings.Contains(notes, "### Documented") {
		t.Errorf("BuildReleaseNotes output missing Documented section:\n%s", notes)
	}
	if !strings.Contains(notes, "Merge pull request #123") {
		t.Errorf("BuildReleaseNotes output dropped a commit with an unrecognized leading verb:\n%s", notes)
	}
}

func TestBuildReleaseNotes_EmptyCommitsNeverErrors(t *testing.T) {
	notes := garelease.BuildReleaseNotes(nil)
	if !strings.Contains(notes, "## Release Notes") {
		t.Fatalf("BuildReleaseNotes(nil) = %q, want at least the heading", notes)
	}
}
