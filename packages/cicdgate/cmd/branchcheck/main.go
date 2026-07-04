// Command branchcheck is the CI entry point for task 3's branch and
// commit-count policy gate. It calls cicdgate.ValidateBranchName and
// cicdgate.ValidatePRCommitCount and exits non-zero (failing the
// invoking CI job) when either check fails.
//
// Run from the repository root as:
//
//	go run ./packages/cicdgate/cmd/branchcheck <branch-name> <commit-count>
//
// .github/workflows/ci.yml's branch-policy job invokes this with
// ${{ github.event.pull_request.head.ref }} and
// ${{ github.event.pull_request.commits }}, both of which are present
// directly on the pull_request event payload with no extra API call
// needed.
package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/YASSERRMD/verdex/packages/cicdgate"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: branchcheck <branch-name> <commit-count>")
		os.Exit(2)
	}

	branchName := os.Args[1]
	commitCount, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "branchcheck: commit-count %q is not an integer: %v\n", os.Args[2], err)
		os.Exit(2)
	}

	failed := false

	if err := cicdgate.ValidateBranchName(branchName); err != nil {
		fmt.Fprintln(os.Stderr, "branchcheck:", err)
		failed = true
	} else {
		fmt.Printf("branchcheck: branch name %q OK\n", branchName)
	}

	if err := cicdgate.ValidatePRCommitCount(branchName, commitCount); err != nil {
		fmt.Fprintln(os.Stderr, "branchcheck:", err)
		failed = true
	} else {
		fmt.Printf("branchcheck: commit count %d OK\n", commitCount)
	}

	if failed {
		os.Exit(1)
	}
}
