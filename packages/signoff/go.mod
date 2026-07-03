module github.com/YASSERRMD/verdex/packages/signoff

go 1.25.0

replace (
	github.com/YASSERRMD/verdex/packages/caselifecycle => ../caselifecycle
	github.com/YASSERRMD/verdex/packages/config => ../config
	github.com/YASSERRMD/verdex/packages/guardrail => ../guardrail
	github.com/YASSERRMD/verdex/packages/identity => ../identity
	github.com/YASSERRMD/verdex/packages/irac => ../irac
	github.com/YASSERRMD/verdex/packages/observability => ../observability
	github.com/YASSERRMD/verdex/packages/persistence => ../persistence
	github.com/YASSERRMD/verdex/packages/tenancy => ../tenancy
)

require (
	github.com/YASSERRMD/verdex/packages/caselifecycle v0.0.0
	github.com/YASSERRMD/verdex/packages/config v0.0.0
	github.com/YASSERRMD/verdex/packages/guardrail v0.0.0
	github.com/YASSERRMD/verdex/packages/identity v0.0.0-00010101000000-000000000000
	github.com/YASSERRMD/verdex/packages/observability v0.0.0
	github.com/YASSERRMD/verdex/packages/persistence v0.0.0
	github.com/YASSERRMD/verdex/packages/tenancy v0.0.0
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.10.0
	github.com/testcontainers/testcontainers-go v0.43.0
	github.com/testcontainers/testcontainers-go/modules/postgres v0.43.0
)
