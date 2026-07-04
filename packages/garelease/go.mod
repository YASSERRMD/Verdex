module github.com/YASSERRMD/verdex/packages/garelease

go 1.25.0

replace (
	github.com/YASSERRMD/verdex/packages/auditlog => ../auditlog
	github.com/YASSERRMD/verdex/packages/cicdgate => ../cicdgate
	github.com/YASSERRMD/verdex/packages/compliance => ../compliance
	github.com/YASSERRMD/verdex/packages/config => ../config
	github.com/YASSERRMD/verdex/packages/guardrail => ../guardrail
	github.com/YASSERRMD/verdex/packages/identity => ../identity
	github.com/YASSERRMD/verdex/packages/irac => ../irac
	github.com/YASSERRMD/verdex/packages/observability => ../observability
	github.com/YASSERRMD/verdex/packages/perf => ../perf
	github.com/YASSERRMD/verdex/packages/persistence => ../persistence
	github.com/YASSERRMD/verdex/packages/securitytesting => ../securitytesting
	github.com/YASSERRMD/verdex/packages/tenancy => ../tenancy
	github.com/YASSERRMD/verdex/packages/vulnmanagement => ../vulnmanagement
)

require (
	github.com/YASSERRMD/verdex/packages/auditlog v0.0.0-00010101000000-000000000000
	github.com/YASSERRMD/verdex/packages/cicdgate v0.0.0-00010101000000-000000000000
	github.com/YASSERRMD/verdex/packages/compliance v0.0.0-00010101000000-000000000000
	github.com/YASSERRMD/verdex/packages/guardrail v0.0.0-00010101000000-000000000000
	github.com/YASSERRMD/verdex/packages/identity v0.0.0-00010101000000-000000000000
	github.com/YASSERRMD/verdex/packages/persistence v0.0.0-00010101000000-000000000000
	github.com/YASSERRMD/verdex/packages/securitytesting v0.0.0-00010101000000-000000000000
	github.com/YASSERRMD/verdex/packages/vulnmanagement v0.0.0-00010101000000-000000000000
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.10.0
)
