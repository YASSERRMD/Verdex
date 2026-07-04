package cicdgate

import "testing"

// sampleCommits is a fixed set of commit subjects modeled directly on
// real subject lines from this repository's own git history (see e.g.
// `git log --oneline -20` at the time this package was written:
// "Merge pull request #85 ...", "Fix notifications tenant-isolation
// test ...", "Fix test fixture missing audit permission ..."),
// exercising every releaseNoteCategories bucket plus the
// otherChangesHeading fallback and the blank-subject skip.
var sampleCommits = []CommitInfo{
	{ShortHash: "561c956", Subject: "Scaffold packages/cicdgate module"},
	{ShortHash: "1ebf9ef", Subject: "Add branch-naming and PR commit-count policy validators"},
	{ShortHash: "e06a791", Subject: "Add ReleaseArtifact and BuildAttestation data model"},
	{ShortHash: "6709e95", Subject: "Fix notifications tenant-isolation test to expect ErrForbidden, not success"},
	{ShortHash: "da5d08a", Subject: "Fix test fixture missing audit permission in packages/annotations"},
	{ShortHash: "d7b8457", Subject: "Merge pull request #85 from YASSERRMD/fix-notifications-access-check"},
	{ShortHash: "abc1234", Subject: "Remove unused seed data from packages/category fixtures"},
	{ShortHash: "def5678", Subject: "Rename ControlEvidence.Reference to EvidenceRef for clarity"},
	{ShortHash: "0591839", Subject: "Document CI/CD pipeline gates and rollback triggers"},
	{ShortHash: "aaa9999", Subject: "Test pipeline gate validators against golden fixtures"},
	{ShortHash: "bbb8888", Subject: "Deploy the new dashboard to the staging environment"},
	{ShortHash: "", Subject: "   "},
}

const wantReleaseNotes = `## Release Notes

### Added

- Scaffold packages/cicdgate module (561c956)
- Add branch-naming and PR commit-count policy validators (1ebf9ef)
- Add ReleaseArtifact and BuildAttestation data model (e06a791)

### Fixed

- Fix notifications tenant-isolation test to expect ErrForbidden, not success (6709e95)
- Fix test fixture missing audit permission in packages/annotations (da5d08a)

### Removed

- Remove unused seed data from packages/category fixtures (abc1234)

### Changed

- Rename ControlEvidence.Reference to EvidenceRef for clarity (def5678)

### Documented

- Document CI/CD pipeline gates and rollback triggers (0591839)

### Tests

- Test pipeline gate validators against golden fixtures (aaa9999)

### Other Changes

- Merge pull request #85 from YASSERRMD/fix-notifications-access-check (d7b8457)
- Deploy the new dashboard to the staging environment (bbb8888)
`

func TestGenerateReleaseNotes_GoldenOutput(t *testing.T) {
	got := GenerateReleaseNotes(sampleCommits)
	if got != wantReleaseNotes {
		t.Errorf("GenerateReleaseNotes() mismatch.\n--- got ---\n%s\n--- want ---\n%s", got, wantReleaseNotes)
	}
}

func TestGenerateReleaseNotes_Empty(t *testing.T) {
	got := GenerateReleaseNotes(nil)
	want := "## Release Notes\n"
	if got != want {
		t.Errorf("GenerateReleaseNotes(nil) = %q, want %q", got, want)
	}
}

func TestGenerateReleaseNotes_OnlyBlankSubjects(t *testing.T) {
	got := GenerateReleaseNotes([]CommitInfo{{ShortHash: "abc", Subject: ""}, {ShortHash: "def", Subject: "   "}})
	want := "## Release Notes\n"
	if got != want {
		t.Errorf("GenerateReleaseNotes() = %q, want %q", got, want)
	}
}

func TestGenerateReleaseNotes_MissingShortHashOmitsParens(t *testing.T) {
	got := GenerateReleaseNotes([]CommitInfo{{Subject: "Add a commit with no recorded hash"}})
	want := "## Release Notes\n\n### Added\n\n- Add a commit with no recorded hash\n"
	if got != want {
		t.Errorf("GenerateReleaseNotes() = %q, want %q", got, want)
	}
}

func TestGenerateReleaseNotes_VerbMatchingIsCaseInsensitive(t *testing.T) {
	got := GenerateReleaseNotes([]CommitInfo{{ShortHash: "abc", Subject: "ADD screaming case subject"}})
	want := "## Release Notes\n\n### Added\n\n- ADD screaming case subject (abc)\n"
	if got != want {
		t.Errorf("GenerateReleaseNotes() = %q, want %q", got, want)
	}
}

func TestGenerateReleaseNotes_SectionOrderIsStableRegardlessOfInputOrder(t *testing.T) {
	// Deliberately supply commits in reverse category order; the
	// rendered notes must still follow releaseNoteCategories' declared
	// order, not input order.
	commits := []CommitInfo{
		{ShortHash: "1", Subject: "Test something"},
		{ShortHash: "2", Subject: "Document something"},
		{ShortHash: "3", Subject: "Fix something"},
		{ShortHash: "4", Subject: "Add something"},
	}
	got := GenerateReleaseNotes(commits)
	want := "## Release Notes\n\n" +
		"### Added\n\n- Add something (4)\n\n" +
		"### Fixed\n\n- Fix something (3)\n\n" +
		"### Documented\n\n- Document something (2)\n\n" +
		"### Tests\n\n- Test something (1)\n"
	if got != want {
		t.Errorf("GenerateReleaseNotes() mismatch.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
