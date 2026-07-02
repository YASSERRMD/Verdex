module github.com/YASSERRMD/verdex/packages/statute

go 1.25.0

require (
	github.com/YASSERRMD/verdex/packages/embedding v0.0.0-00010101000000-000000000000
	github.com/YASSERRMD/verdex/packages/irac v0.0.0
	github.com/YASSERRMD/verdex/packages/jurisdiction v0.0.0-00010101000000-000000000000
)

require (
	github.com/YASSERRMD/verdex/packages/provider v0.0.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
)

replace (
	github.com/YASSERRMD/verdex/packages/embedding => ../embedding
	github.com/YASSERRMD/verdex/packages/graph => ../graph
	github.com/YASSERRMD/verdex/packages/irac => ../irac
	github.com/YASSERRMD/verdex/packages/jurisdiction => ../jurisdiction
	github.com/YASSERRMD/verdex/packages/provider => ../provider
)
