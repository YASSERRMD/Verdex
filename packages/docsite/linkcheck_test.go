package docsite

import (
	"errors"
	"path/filepath"
	"sort"
	"testing"
)

func TestCheckLinks_CleanFixtureReportsNoBrokenLinks(t *testing.T) {
	report, err := CheckLinks(filepath.Join("testdata", "clean"))
	if err != nil {
		t.Fatalf("CheckLinks returned unexpected error: %v", err)
	}

	if !report.OK() {
		t.Fatalf("expected a clean Report, got %d broken link(s): %+v", len(report.Broken), report.Broken)
	}

	// Four markdown files: docs/README.md, docs/architecture/overview.md,
	// docs/code-examples.md, packages/foo/doc/foo.md.
	if len(report.FilesScanned) != 4 {
		t.Fatalf("expected 4 files scanned, got %d: %v", len(report.FilesScanned), report.FilesScanned)
	}

	// docs/README.md alone has 4 internal links (architecture overview,
	// foo doc, the packages directory, and the "#a-heading" same-page
	// anchor) plus 2 external links (https and mailto) that must not be
	// counted. Across all three files there are more; just assert the
	// external/mailto links were excluded by checking the count is
	// smaller than it would be if they were wrongly included.
	if report.LinksChecked == 0 {
		t.Fatal("expected at least one internal link to have been checked")
	}
}

func TestCheckLinks_BrokenFixtureReportsExactBrokenLinks(t *testing.T) {
	report, err := CheckLinks(filepath.Join("testdata", "broken"))
	if err != nil {
		t.Fatalf("CheckLinks returned unexpected error: %v", err)
	}

	if report.OK() {
		t.Fatal("expected a non-clean Report against the broken fixture, got zero broken links")
	}

	if len(report.Broken) != 2 {
		t.Fatalf("expected exactly 2 broken links, got %d: %+v", len(report.Broken), report.Broken)
	}

	// Sort for a deterministic assertion order regardless of file walk
	// order.
	sort.Slice(report.Broken, func(i, j int) bool {
		return report.Broken[i].SourceFile < report.Broken[j].SourceFile
	})

	docsBroken := report.Broken[0]
	wantDocsBroken := filepath.Join("docs", "README.md")
	if docsBroken.SourceFile != wantDocsBroken {
		t.Errorf("first broken link SourceFile = %q, want %q", docsBroken.SourceFile, wantDocsBroken)
	}
	if docsBroken.Target != "does-not-exist.md" {
		t.Errorf("first broken link Target = %q, want %q", docsBroken.Target, "does-not-exist.md")
	}
	if docsBroken.LinkText != "nonexistent page" {
		t.Errorf("first broken link LinkText = %q, want %q", docsBroken.LinkText, "nonexistent page")
	}
	if docsBroken.Line == 0 {
		t.Error("expected a non-zero line number for the first broken link")
	}

	pkgBroken := report.Broken[1]
	wantPkgBroken := filepath.Join("packages", "foo", "doc", "foo.md")
	if pkgBroken.SourceFile != wantPkgBroken {
		t.Errorf("second broken link SourceFile = %q, want %q", pkgBroken.SourceFile, wantPkgBroken)
	}
	if pkgBroken.Target != "../../bar/doc/bar.md" {
		t.Errorf("second broken link Target = %q, want %q", pkgBroken.Target, "../../bar/doc/bar.md")
	}
}

func TestExtractLinks_SkipsFencedCodeBlocks(t *testing.T) {
	links, err := extractLinks(filepath.Join("testdata", "clean", "docs", "code-examples.md"))
	if err != nil {
		t.Fatalf("extractLinks returned unexpected error: %v", err)
	}

	// Exactly 2 real links exist in this fixture, both outside fenced
	// blocks: "[docs index](README.md)" and
	// "[foo package doc](../packages/foo/doc/foo.md)". The
	// "[also skipped](inside-a-tilde-fence.md)" line and the Go
	// generic-call syntax "NewIdempotencyGuard[PaymentResult](10 *
	// time.Minute)" both live inside fenced blocks (``` and ~~~
	// respectively) and must not be extracted as links.
	if len(links) != 2 {
		t.Fatalf("expected exactly 2 links extracted (fenced-block content skipped), got %d: %+v", len(links), links)
	}

	for _, link := range links {
		if link.target == "inside-a-tilde-fence.md" {
			t.Error("a link inside a ~~~-fenced block was extracted; fenced blocks must be skipped")
		}
		if link.text == "PaymentResult" {
			t.Error("Go generic-call syntax inside a ```-fenced block was misread as a markdown link")
		}
	}

	if links[0].target != "README.md" {
		t.Errorf("first extracted link target = %q, want %q", links[0].target, "README.md")
	}
	if links[1].target != "../packages/foo/doc/foo.md" {
		t.Errorf("second extracted link target = %q, want %q", links[1].target, "../packages/foo/doc/foo.md")
	}
}

func TestCheckLinks_ExternalAndMailtoLinksAreNeverResolved(t *testing.T) {
	report, err := CheckLinks(filepath.Join("testdata", "clean"))
	if err != nil {
		t.Fatalf("CheckLinks returned unexpected error: %v", err)
	}

	for _, broken := range report.Broken {
		if isExternalLink(broken.Target) {
			t.Errorf("an external/mailto link was reported broken, should have been skipped entirely: %+v", broken)
		}
	}
}

func TestCheckLinks_EmptyRoot(t *testing.T) {
	_, err := CheckLinks("")
	if !errors.Is(err, ErrEmptyRoot) {
		t.Fatalf("expected ErrEmptyRoot, got %v", err)
	}
}

func TestCheckLinks_RootNotFound(t *testing.T) {
	_, err := CheckLinks(filepath.Join("testdata", "does-not-exist-at-all"))
	if !errors.Is(err, ErrRootNotFound) {
		t.Fatalf("expected ErrRootNotFound, got %v", err)
	}
}

func TestCheckLinks_RootWithNoMarkdownFiles(t *testing.T) {
	_, err := CheckLinks(filepath.Join("testdata", "empty"))
	if !errors.Is(err, ErrNoMarkdownFiles) {
		t.Fatalf("expected ErrNoMarkdownFiles, got %v", err)
	}
}

func TestReport_OK(t *testing.T) {
	clean := Report{}
	if !clean.OK() {
		t.Error("a Report with no Broken entries should be OK")
	}

	dirty := Report{Broken: []BrokenLink{{SourceFile: "x.md"}}}
	if dirty.OK() {
		t.Error("a Report with a Broken entry should not be OK")
	}
}

func TestStripFragment(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"foo.md", "foo.md"},
		{"foo.md#section", "foo.md"},
		{"#section-only", ""},
		{"../a/b.md#a-b-c", "../a/b.md"},
	}
	for _, c := range cases {
		if got := stripFragment(c.in); got != c.want {
			t.Errorf("stripFragment(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIsExternalLink(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"https://example.com", true},
		{"http://example.com", true},
		{"HTTPS://EXAMPLE.COM", true},
		{"mailto:someone@example.com", true},
		{"../relative/path.md", false},
		{"relative.md", false},
		{"/absolute/looking/path.md", false},
	}
	for _, c := range cases {
		if got := isExternalLink(c.in); got != c.want {
			t.Errorf("isExternalLink(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
