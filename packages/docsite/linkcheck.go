package docsite

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// mdLinkPattern matches a markdown inline link: "[link text](target)".
// It deliberately does not attempt to match reference-style links
// ("[text][ref]") or bare autolinks ("<https://...>") -- every link in
// this repository's documentation (as of this phase) uses the inline
// form, and this pattern is intentionally simple and auditable rather
// than a full CommonMark parser.
var mdLinkPattern = regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\)`)

// docLocations names the two known documentation locations CheckLinks
// scans, relative to root: the root-level docs/ tree (walked
// recursively) and every package's own doc/ directory (walked
// non-recursively -- no package's doc/ directory nests
// subdirectories as of this phase).
const (
	docsDirName    = "docs"
	packagesDir    = "packages"
	packageDocName = "doc"
)

// CheckLinks walks root's docs/ tree and every packages/*/doc/*.md
// file, extracts every markdown inline link from each, and verifies
// that every relative (internal) link target resolves to a real file
// or directory on disk relative to the linking file's own directory.
//
// External links (http://, https://) and mailto: links are skipped
// entirely -- this checker only ever asserts about paths inside this
// repository, never network reachability. A target's "#fragment"
// suffix (if any) is stripped before resolution; this checker
// verifies the linked file/directory exists, not that a specific
// heading anchor exists within it.
//
// CheckLinks returns ErrEmptyRoot for a blank root, ErrRootNotFound if
// root does not exist or is not a directory, and ErrNoMarkdownFiles if
// walking root's docs/ and packages/*/doc/ locations finds zero
// markdown files -- each of these is almost always a misconfigured
// root rather than a legitimately empty documentation tree, so they
// are hard errors rather than a silently clean Report. Once at least
// one markdown file is found, CheckLinks always returns a non-nil
// Report and a nil error -- broken links are reported via
// Report.Broken/Report.OK, never via the returned error, so a caller
// can distinguish "the checker itself failed to run" from "the
// checker ran and found problems."
func CheckLinks(root string) (Report, error) {
	if root == "" {
		return Report{}, wrapf("CheckLinks", ErrEmptyRoot)
	}

	rootInfo, err := os.Stat(root)
	if err != nil || !rootInfo.IsDir() {
		return Report{}, wrapf("CheckLinks", ErrRootNotFound)
	}

	files, err := discoverMarkdownFiles(root)
	if err != nil {
		return Report{}, wrapf("CheckLinks", err)
	}
	if len(files) == 0 {
		return Report{}, wrapf("CheckLinks", ErrNoMarkdownFiles)
	}

	report := Report{FilesScanned: files}

	for _, relFile := range files {
		absFile := filepath.Join(root, relFile)
		links, err := extractLinks(absFile)
		if err != nil {
			return Report{}, wrapf("CheckLinks", err)
		}

		fileDir := filepath.Dir(absFile)
		for _, link := range links {
			if isExternalLink(link.target) {
				continue
			}
			report.LinksChecked++

			targetPath := stripFragment(link.target)
			if targetPath == "" {
				// A pure "#fragment" same-page anchor link: nothing
				// to resolve on disk.
				continue
			}

			resolved := filepath.Join(fileDir, filepath.FromSlash(targetPath))
			if _, statErr := os.Stat(resolved); statErr != nil {
				report.Broken = append(report.Broken, BrokenLink{
					SourceFile:   relFile,
					Line:         link.line,
					LinkText:     link.text,
					Target:       link.target,
					ResolvedPath: resolved,
				})
			}
		}
	}

	return report, nil
}

// discoverMarkdownFiles returns every *.md file under root's docs/
// tree (recursive) and every root/packages/*/doc/*.md file
// (non-recursive per package), as paths relative to root, sorted for
// deterministic output.
func discoverMarkdownFiles(root string) ([]string, error) {
	var found []string

	docsRoot := filepath.Join(root, docsDirName)
	if info, err := os.Stat(docsRoot); err == nil && info.IsDir() {
		err := filepath.WalkDir(docsRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if strings.EqualFold(filepath.Ext(path), ".md") {
				rel, relErr := filepath.Rel(root, path)
				if relErr != nil {
					return relErr
				}
				found = append(found, rel)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	packagesRoot := filepath.Join(root, packagesDir)
	packageEntries, err := os.ReadDir(packagesRoot)
	if err == nil {
		for _, pkgEntry := range packageEntries {
			if !pkgEntry.IsDir() {
				continue
			}
			docDir := filepath.Join(packagesRoot, pkgEntry.Name(), packageDocName)
			docEntries, err := os.ReadDir(docDir)
			if err != nil {
				continue // this package has no doc/ directory
			}
			for _, docEntry := range docEntries {
				if docEntry.IsDir() {
					continue
				}
				if strings.EqualFold(filepath.Ext(docEntry.Name()), ".md") {
					rel, relErr := filepath.Rel(root, filepath.Join(docDir, docEntry.Name()))
					if relErr != nil {
						return nil, relErr
					}
					found = append(found, rel)
				}
			}
		}
	}

	sort.Strings(found)
	return found, nil
}

// extractedLink is one markdown inline link found within a file, plus
// its 1-indexed line number.
type extractedLink struct {
	line   int
	text   string
	target string
}

// extractLinks scans absPath line by line for markdown inline links.
// Scanning per line (rather than the whole file at once) means a
// reported Line number is always exact, and deliberately does not
// support a link target split across multiple lines -- no link in
// this repository's documentation does that as of this phase.
func extractLinks(absPath string) ([]extractedLink, error) {
	f, err := os.Open(absPath) //nolint:gosec // absPath is built from a repository-relative walk, not untrusted user input.
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // read-only scan; nothing to react to on close failure.

	var links []extractedLink
	scanner := bufio.NewScanner(f)
	// Documentation lines can be long (e.g. dense tables); grow the
	// scanner's buffer well past bufio.Scanner's 64KiB default line
	// limit rather than silently truncating or erroring on a long line.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		for _, match := range mdLinkPattern.FindAllStringSubmatch(line, -1) {
			links = append(links, extractedLink{
				line:   lineNum,
				text:   match[1],
				target: match[2],
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return links, nil
}

// isExternalLink reports whether target is a link this checker never
// resolves against the local filesystem: a network URL or a mailto:
// link.
func isExternalLink(target string) bool {
	lower := strings.ToLower(target)
	return strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "mailto:")
}

// stripFragment removes a trailing "#fragment" from target, if
// present, and returns the remaining path portion (which may be empty
// for a pure same-page anchor link like "#section").
func stripFragment(target string) string {
	if idx := strings.Index(target, "#"); idx >= 0 {
		return target[:idx]
	}
	return target
}
