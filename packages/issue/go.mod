module github.com/YASSERRMD/verdex/packages/issue

go 1.25.0

require (
	github.com/YASSERRMD/verdex/packages/evidence v0.0.0-00010101000000-000000000000
	github.com/YASSERRMD/verdex/packages/irac v0.0.0
	github.com/YASSERRMD/verdex/packages/segmentation v0.0.0
	github.com/YASSERRMD/verdex/packages/timeline v0.0.0-00010101000000-000000000000
)

replace (
	github.com/YASSERRMD/verdex/packages/evidence => ../evidence
	github.com/YASSERRMD/verdex/packages/graph => ../graph
	github.com/YASSERRMD/verdex/packages/irac => ../irac
	github.com/YASSERRMD/verdex/packages/segmentation => ../segmentation
	github.com/YASSERRMD/verdex/packages/timeline => ../timeline
)
