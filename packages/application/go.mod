module github.com/YASSERRMD/verdex/packages/application

go 1.25.0

require github.com/YASSERRMD/verdex/packages/irac v0.0.0

replace (
	github.com/YASSERRMD/verdex/packages/config => ../config
	github.com/YASSERRMD/verdex/packages/evidence => ../evidence
	github.com/YASSERRMD/verdex/packages/fact => ../fact
	github.com/YASSERRMD/verdex/packages/graph => ../graph
	github.com/YASSERRMD/verdex/packages/irac => ../irac
	github.com/YASSERRMD/verdex/packages/issue => ../issue
	github.com/YASSERRMD/verdex/packages/observability => ../observability
	github.com/YASSERRMD/verdex/packages/persistence => ../persistence
	github.com/YASSERRMD/verdex/packages/segmentation => ../segmentation
	github.com/YASSERRMD/verdex/packages/timeline => ../timeline
)
