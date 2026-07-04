module github.com/YASSERRMD/verdex/packages/alerting

go 1.25.0

replace (
	github.com/YASSERRMD/verdex/packages/accounting => ../accounting
	github.com/YASSERRMD/verdex/packages/config => ../config
	github.com/YASSERRMD/verdex/packages/identity => ../identity
	github.com/YASSERRMD/verdex/packages/observability => ../observability
	github.com/YASSERRMD/verdex/packages/persistence => ../persistence
	github.com/YASSERRMD/verdex/packages/provider => ../provider
	github.com/YASSERRMD/verdex/packages/reasoningeval => ../reasoningeval
	github.com/YASSERRMD/verdex/packages/reliability => ../reliability
	github.com/YASSERRMD/verdex/packages/tenancy => ../tenancy
)

require (
	github.com/YASSERRMD/verdex/packages/accounting v0.0.0-00010101000000-000000000000
	github.com/YASSERRMD/verdex/packages/identity v0.0.0-00010101000000-000000000000
	github.com/YASSERRMD/verdex/packages/observability v0.0.0-00010101000000-000000000000
	github.com/YASSERRMD/verdex/packages/persistence v0.0.0-00010101000000-000000000000
	github.com/YASSERRMD/verdex/packages/reasoningeval v0.0.0-00010101000000-000000000000
	github.com/YASSERRMD/verdex/packages/reliability v0.0.0-00010101000000-000000000000
	github.com/YASSERRMD/verdex/packages/tenancy v0.0.0-00010101000000-000000000000
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.10.0
)
