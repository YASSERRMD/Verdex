module github.com/YASSERRMD/verdex/packages/notifications

go 1.25.0

replace (
	github.com/YASSERRMD/verdex/packages/annotations => ../annotations
	github.com/YASSERRMD/verdex/packages/caselifecycle => ../caselifecycle
	github.com/YASSERRMD/verdex/packages/config => ../config
	github.com/YASSERRMD/verdex/packages/identity => ../identity
	github.com/YASSERRMD/verdex/packages/observability => ../observability
	github.com/YASSERRMD/verdex/packages/persistence => ../persistence
	github.com/YASSERRMD/verdex/packages/reasoningeval => ../reasoningeval
	github.com/YASSERRMD/verdex/packages/signoff => ../signoff
	github.com/YASSERRMD/verdex/packages/tenancy => ../tenancy
)

require (
	github.com/YASSERRMD/verdex/packages/annotations v0.0.0
	github.com/YASSERRMD/verdex/packages/identity v0.0.0
	github.com/YASSERRMD/verdex/packages/persistence v0.0.0
	github.com/YASSERRMD/verdex/packages/reasoningeval v0.0.0
	github.com/YASSERRMD/verdex/packages/signoff v0.0.0
	github.com/YASSERRMD/verdex/packages/tenancy v0.0.0
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.10.0
	github.com/testcontainers/testcontainers-go v0.43.0
	github.com/testcontainers/testcontainers-go/modules/postgres v0.43.0
)
