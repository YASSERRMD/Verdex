module github.com/YASSERRMD/verdex/packages/treevalidation

go 1.25.0

require (
	github.com/YASSERRMD/verdex/packages/irac v0.0.0-00010101000000-000000000000
	github.com/YASSERRMD/verdex/packages/treeassembly v0.0.0-00010101000000-000000000000
)

replace (
	github.com/YASSERRMD/verdex/packages/irac => ../irac
	github.com/YASSERRMD/verdex/packages/treeassembly => ../treeassembly
)
