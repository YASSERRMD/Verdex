package treeassembly

import (
	"time"

	"github.com/YASSERRMD/verdex/packages/irac"
)

// NextRevision bumps tree's revision to the next one in its case's
// revision sequence, using irac.NextRevision (packages/irac/version.go):
// RevisionNumber incremented by one, ParentRevision set to the current
// RevisionNumber. Called whenever ComposeTree runs again for a case that
// already has an assembled tree (see service.go's
// TreeAssemblyService.Assemble), so each re-assembly produces a new,
// traceable revision rather than mutating the previous one in place.
//
// A nil tree returns the zero irac.TreeRevision, since there is no prior
// revision to bump.
func NextRevision(tree *Tree) irac.TreeRevision {
	if tree == nil {
		return irac.TreeRevision{}
	}
	return irac.NextRevision(tree.Revision, time.Now())
}
