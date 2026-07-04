// Command checklinks is the CI entry point for packages/docsite's
// internal-link checker. It calls docsite.CheckLinks against a
// repository root and exits non-zero (failing the invoking CI job) if
// any broken link is found.
//
// Run from the repository root as:
//
//	go run ./packages/docsite/cmd/checklinks .
//
// .github/workflows/ci.yml's docs-link-check job invokes this against
// the checked-out repository root.
package main

import (
	"fmt"
	"os"

	"github.com/YASSERRMD/verdex/packages/docsite"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: checklinks <repository-root>")
		os.Exit(2)
	}

	root := os.Args[1]

	report, err := docsite.CheckLinks(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "checklinks: %v\n", err)
		os.Exit(2)
	}

	fmt.Printf("checklinks: scanned %d markdown file(s), checked %d internal link(s)\n",
		len(report.FilesScanned), report.LinksChecked)

	if report.OK() {
		fmt.Println("checklinks: no broken links found")
		return
	}

	fmt.Fprintf(os.Stderr, "checklinks: found %d broken link(s):\n", len(report.Broken))
	for _, broken := range report.Broken {
		fmt.Fprintf(os.Stderr, "  %s:%d: [%s](%s) -- does not resolve (expected %s)\n",
			broken.SourceFile, broken.Line, broken.LinkText, broken.Target, broken.ResolvedPath)
	}
	os.Exit(1)
}
