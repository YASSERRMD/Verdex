package cicdgate

import (
	"fmt"
	"regexp"
	"strings"
)

// CommitInfo is the minimal shape GenerateReleaseNotes needs from a
// commit: its subject line (the first line of the commit message,
// exactly as `git log --format=%s` would report it) and short hash
// (for a reference link in the rendered notes).
type CommitInfo struct {
	// ShortHash is the abbreviated commit SHA (e.g. "561c956").
	ShortHash string

	// Subject is the commit's subject line, expected in this
	// repository's imperative-mood convention: "Add X", "Fix Y",
	// "Remove Z" (CONTRIBUTING.md's "Commits" section).
	Subject string
}

// releaseNoteCategory names one section of a generated changelog, in
// the fixed display order GenerateReleaseNotes renders them in.
type releaseNoteCategory struct {
	heading string
	verbs   []string
}

// releaseNoteCategories maps this repository's actual imperative
// commit-verb vocabulary (surveyed from `git log --format=%s` across
// this repository's history: Add dominates, followed by Document,
// Fix, and a long tail of other verbs -- see doc/cicd.md) into a small
// set of conventional-changelog-style headings. Order here is the
// order sections render in.
var releaseNoteCategories = []releaseNoteCategory{
	{heading: "Added", verbs: []string{"add", "implement", "introduce", "seed", "register", "scaffold"}},
	{heading: "Fixed", verbs: []string{"fix", "correct", "resolve"}},
	{heading: "Removed", verbs: []string{"remove", "delete", "drop"}},
	{heading: "Changed", verbs: []string{"update", "change", "rename", "refactor", "extend", "enable", "wire", "compose", "persist", "expose", "tidy", "renumber"}},
	{heading: "Documented", verbs: []string{"document"}},
	{heading: "Tests", verbs: []string{"test", "tests"}},
}

// otherChangesHeading is the section a commit subject falls into when
// its leading verb does not match any entry in
// releaseNoteCategories -- e.g. "Merge pull request #112 ..." or a
// verb not yet in the vocabulary above. Commits are still included in
// the generated notes (never silently dropped), just grouped
// generically.
const otherChangesHeading = "Other Changes"

// commitSubjectVerb extracts the leading imperative verb from a
// commit subject, e.g. "Add branch policy validators" -> "add". Verbs
// are matched case-insensitively against a single leading word;
// "Merge pull request #112 from ..." yields "merge", which
// deliberately matches no category below (merge commits are noise in
// a changelog, not a change to call out).
var commitSubjectVerb = regexp.MustCompile(`^([A-Za-z]+)\b`)

// verbToHeading builds a reverse lookup: lowercase verb -> section
// heading, from releaseNoteCategories.
func verbToHeading() map[string]string {
	m := make(map[string]string)
	for _, cat := range releaseNoteCategories {
		for _, v := range cat.verbs {
			m[v] = cat.heading
		}
	}
	return m
}

// GenerateReleaseNotes groups commits by their leading imperative verb
// into a markdown changelog section, mirroring this repository's
// actual commit-message convention (CONTRIBUTING.md: "Use the
// imperative mood: Add X, Fix Y, Remove Z") rather than a generic
// Conventional Commits (feat:/fix:) scheme this repository does not
// use (task 8: release notes automation).
//
// Sections render in the fixed order declared by
// releaseNoteCategories (Added, Fixed, Removed, Changed, Documented,
// Tests), followed by "Other Changes" for any commit whose leading
// verb does not match a known category -- most commonly merge
// commits. A section with no matching commits is omitted entirely.
// Within a section, commits render in the order given in commits.
//
// commits with a blank Subject are skipped. GenerateReleaseNotes never
// returns an error: given zero commits, or commits that only contain
// unrecognized verbs, it returns a "## Release Notes" heading with no
// body sections (or only "Other Changes"), never panics or fails.
func GenerateReleaseNotes(commits []CommitInfo) string {
	verbHeading := verbToHeading()

	grouped := make(map[string][]CommitInfo, len(releaseNoteCategories)+1)
	for _, c := range commits {
		subject := strings.TrimSpace(c.Subject)
		if subject == "" {
			continue
		}
		heading := otherChangesHeading
		if m := commitSubjectVerb.FindStringSubmatch(subject); m != nil {
			if h, ok := verbHeading[strings.ToLower(m[1])]; ok {
				heading = h
			}
		}
		grouped[heading] = append(grouped[heading], c)
	}

	var b strings.Builder
	b.WriteString("## Release Notes\n")

	headings := make([]string, 0, len(releaseNoteCategories)+1)
	for _, cat := range releaseNoteCategories {
		headings = append(headings, cat.heading)
	}
	headings = append(headings, otherChangesHeading)

	for _, heading := range headings {
		section := grouped[heading]
		if len(section) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n### %s\n\n", heading)
		for _, c := range section {
			if strings.TrimSpace(c.ShortHash) == "" {
				fmt.Fprintf(&b, "- %s\n", c.Subject)
				continue
			}
			fmt.Fprintf(&b, "- %s (%s)\n", c.Subject, c.ShortHash)
		}
	}

	return b.String()
}
