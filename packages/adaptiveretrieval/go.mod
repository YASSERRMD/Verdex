module github.com/YASSERRMD/verdex/packages/adaptiveretrieval

go 1.25.0

replace (
	github.com/YASSERRMD/verdex/packages/config => ../config
	github.com/YASSERRMD/verdex/packages/embedding => ../embedding
	github.com/YASSERRMD/verdex/packages/graph => ../graph
	github.com/YASSERRMD/verdex/packages/hybridretrieval => ../hybridretrieval
	github.com/YASSERRMD/verdex/packages/irac => ../irac
	github.com/YASSERRMD/verdex/packages/observability => ../observability
	github.com/YASSERRMD/verdex/packages/persistence => ../persistence
	github.com/YASSERRMD/verdex/packages/provider => ../provider
	github.com/YASSERRMD/verdex/packages/treeindex => ../treeindex
	github.com/YASSERRMD/verdex/packages/traversal => ../traversal
	github.com/YASSERRMD/verdex/packages/vectorindex => ../vectorindex
)

require (
	github.com/YASSERRMD/verdex/packages/graph v0.0.0-00010101000000-000000000000
	github.com/YASSERRMD/verdex/packages/hybridretrieval v0.0.0-00010101000000-000000000000
	github.com/YASSERRMD/verdex/packages/irac v0.0.0
	github.com/YASSERRMD/verdex/packages/traversal v0.0.0-00010101000000-000000000000
	github.com/YASSERRMD/verdex/packages/treeindex v0.0.0-00010101000000-000000000000
)
