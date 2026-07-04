package docsite

// BrokenLink records one markdown relative link that did not resolve
// to a real file on disk.
type BrokenLink struct {
	// SourceFile is the path (relative to the repository root passed
	// to CheckLinks) of the markdown file containing the broken link.
	SourceFile string

	// Line is the 1-indexed line number within SourceFile the link
	// appears on.
	Line int

	// LinkText is the markdown link's display text -- the part between
	// "[" and "]".
	LinkText string

	// Target is the raw link target as written in the markdown source
	// -- the part between "(" and ")", including any "#fragment"
	// suffix.
	Target string

	// ResolvedPath is the absolute filesystem path CheckLinks attempted
	// to resolve Target to, before confirming it does not exist.
	ResolvedPath string
}

// Report is the result of a CheckLinks run: every markdown file
// scanned, every relative link extracted from them, and every one of
// those links that failed to resolve.
type Report struct {
	// FilesScanned lists every markdown file CheckLinks examined,
	// relative to the scanned root.
	FilesScanned []string

	// LinksChecked is the total count of relative (internal) links
	// extracted across every file in FilesScanned. External links
	// (http:// and https://) and mailto: links are intentionally not
	// counted here -- see CheckLinks' doc comment.
	LinksChecked int

	// Broken lists every link that failed to resolve, in the order
	// encountered (file order, then line order within a file).
	Broken []BrokenLink
}

// OK reports whether this Report found zero broken links. A Report
// with FilesScanned empty is still OK -- CheckLinks itself returns an
// error if it found no markdown files at all under root, distinguishing
// "found nothing to check" (an error, likely a misconfigured root) from
// "checked N files, found zero broken links" (a clean OK Report).
func (r Report) OK() bool {
	return len(r.Broken) == 0
}
