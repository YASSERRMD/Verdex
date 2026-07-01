module github.com/YASSERRMD/verdex/packages/router

go 1.25.0

require (
	github.com/YASSERRMD/verdex/packages/provider v0.0.0
	github.com/YASSERRMD/verdex/packages/tenancy v0.0.0
)

replace (
	github.com/YASSERRMD/verdex/packages/provider => ../provider
	github.com/YASSERRMD/verdex/packages/tenancy => ../tenancy
)
