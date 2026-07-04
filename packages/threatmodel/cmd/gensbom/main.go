// Command gensbom regenerates the committed SBOM snapshot at
// packages/threatmodel/doc/sbom.json by calling
// threatmodel.WriteSBOMSnapshot against the repository root. Run it
// from the repository root as:
//
//	go run ./packages/threatmodel/cmd/gensbom
//
// or with explicit paths:
//
//	go run ./packages/threatmodel/cmd/gensbom <workspace-root> <dest-path>
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/YASSERRMD/verdex/packages/threatmodel"
)

func main() {
	workspaceRoot := "."
	destPath := filepath.Join("packages", "threatmodel", "doc", "sbom.json")

	switch len(os.Args) {
	case 1:
		// Use the defaults above.
	case 3:
		workspaceRoot = os.Args[1]
		destPath = os.Args[2]
	default:
		fmt.Fprintln(os.Stderr, "usage: gensbom [workspace-root dest-path]")
		os.Exit(2)
	}

	if err := threatmodel.WriteSBOMSnapshot(workspaceRoot, destPath); err != nil {
		fmt.Fprintln(os.Stderr, "gensbom:", err)
		os.Exit(1)
	}
	fmt.Println("wrote", destPath)
}
