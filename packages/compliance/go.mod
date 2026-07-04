module github.com/YASSERRMD/verdex/packages/compliance

go 1.25.0

replace (
	github.com/YASSERRMD/verdex/packages/auditlog => ../auditlog
	github.com/YASSERRMD/verdex/packages/caselifecycle => ../caselifecycle
	github.com/YASSERRMD/verdex/packages/config => ../config
	github.com/YASSERRMD/verdex/packages/encryption => ../encryption
	github.com/YASSERRMD/verdex/packages/guardrail => ../guardrail
	github.com/YASSERRMD/verdex/packages/identity => ../identity
	github.com/YASSERRMD/verdex/packages/irac => ../irac
	github.com/YASSERRMD/verdex/packages/observability => ../observability
	github.com/YASSERRMD/verdex/packages/persistence => ../persistence
	github.com/YASSERRMD/verdex/packages/signoff => ../signoff
	github.com/YASSERRMD/verdex/packages/tenancy => ../tenancy
)

require github.com/google/uuid v1.6.0
