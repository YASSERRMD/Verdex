module github.com/YASSERRMD/verdex/packages/setup

go 1.25.0

require (
	github.com/YASSERRMD/verdex/packages/jurisdiction v0.0.0
	github.com/YASSERRMD/verdex/packages/tenancy v0.0.0
	github.com/google/uuid v1.6.0
)

require github.com/jackc/pgx/v5 v5.10.0 // indirect

replace (
	github.com/YASSERRMD/verdex/packages/jurisdiction => ../jurisdiction
	github.com/YASSERRMD/verdex/packages/tenancy => ../tenancy
)
