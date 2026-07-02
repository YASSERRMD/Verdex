module github.com/YASSERRMD/verdex/packages/treeindex

go 1.25.0

replace (
	github.com/YASSERRMD/verdex/packages/graph => ../graph
	github.com/YASSERRMD/verdex/packages/irac => ../irac
)

require (
	github.com/YASSERRMD/verdex/packages/graph v0.0.0-00010101000000-000000000000
	github.com/YASSERRMD/verdex/packages/irac v0.0.0
)
