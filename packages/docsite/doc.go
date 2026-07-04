// Package docsite is Phase 098: the internal-link checker for this
// repository's documentation tree -- the automated "review and link
// docs" mechanism, rather than a manual claim that every link in
// docs/ and every packages/*/doc/*.md file actually resolves.
//
// # What is new here
//
//   - CheckLinks (linkcheck.go): walks root's docs/ tree (recursively)
//     and every packages/*/doc/*.md file (each package's doc/
//     directory, non-recursively -- no package nests a subdirectory
//     under its own doc/ as of this phase), extracts every markdown
//     inline link ("[text](target)") from every file found, and
//     verifies each internal (non-http(s), non-mailto) link target
//     resolves to a real file or directory on disk, relative to the
//     linking file's own directory. A target's trailing "#fragment"
//     is stripped before resolution -- this checker verifies the
//     linked file/directory exists, not that a specific heading
//     anchor exists within it.
//   - Report / BrokenLink (types.go): CheckLinks' result -- every file
//     scanned, a total count of internal links checked, and every
//     link that failed to resolve, each recording its source file,
//     line number, link text, raw target, and the absolute path
//     CheckLinks attempted to resolve it to.
//   - cmd/checklinks (cmd/checklinks/main.go): the CI entry point --
//     run against a repository root, it calls CheckLinks, prints
//     every BrokenLink found, and exits non-zero if the Report is not
//     OK, so a broken link fails the build rather than requiring a
//     human to notice it.
//
// # What this package deliberately does not do
//
//   - It is not a CommonMark parser. It matches the inline-link form
//     ("[text](target)") that every link in this repository's
//     documentation uses as of this phase; it does not attempt
//     reference-style links ("[text][ref]") or bare autolinks
//     ("<https://...>").
//   - It does not verify that a "#fragment" heading anchor actually
//     exists within the resolved target file -- only that the file or
//     directory itself exists. Verifying an exact heading-anchor slug
//     match is a reasonable future extension, not something this
//     phase claims to do.
//   - It does not check external links (http://, https://, mailto:)
//     for reachability. This checker only ever asserts about paths
//     inside this repository.
//
// See doc/docsite.md for the full write-up, including how this
// package is wired into .github/workflows/ci.yml.
package docsite
