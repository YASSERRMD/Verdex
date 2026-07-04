module github.com/YASSERRMD/verdex/packages/threatmodel

go 1.25.0

replace (
	github.com/YASSERRMD/verdex/packages/auditlog => ../auditlog
	github.com/YASSERRMD/verdex/packages/caselifecycle => ../caselifecycle
	github.com/YASSERRMD/verdex/packages/encryption => ../encryption
	github.com/YASSERRMD/verdex/packages/guardrail => ../guardrail
	github.com/YASSERRMD/verdex/packages/identity => ../identity
	github.com/YASSERRMD/verdex/packages/irac => ../irac
	github.com/YASSERRMD/verdex/packages/observability => ../observability
	github.com/YASSERRMD/verdex/packages/signoff => ../signoff
)

require github.com/google/uuid v1.6.0
