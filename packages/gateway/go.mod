module github.com/YASSERRMD/verdex/packages/gateway

go 1.25.0

require github.com/google/uuid v1.6.0

replace (
	github.com/YASSERRMD/verdex/packages/config => ../config
	github.com/YASSERRMD/verdex/packages/identity => ../identity
	github.com/YASSERRMD/verdex/packages/observability => ../observability
	github.com/YASSERRMD/verdex/packages/persistence => ../persistence
	github.com/YASSERRMD/verdex/packages/tenancy => ../tenancy
)
