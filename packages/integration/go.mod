module github.com/YASSERRMD/verdex/packages/integration

go 1.25.0

replace (
	github.com/YASSERRMD/verdex/packages/auditlog => ../auditlog
	github.com/YASSERRMD/verdex/packages/config => ../config
	github.com/YASSERRMD/verdex/packages/identity => ../identity
	github.com/YASSERRMD/verdex/packages/observability => ../observability
	github.com/YASSERRMD/verdex/packages/persistence => ../persistence
	github.com/YASSERRMD/verdex/packages/tenancy => ../tenancy
)

require (
	github.com/YASSERRMD/verdex/packages/auditlog v0.0.0-00010101000000-000000000000
	github.com/YASSERRMD/verdex/packages/identity v0.0.0-00010101000000-000000000000
	github.com/YASSERRMD/verdex/packages/persistence v0.0.0
	github.com/YASSERRMD/verdex/packages/tenancy v0.0.0
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.10.0
)

require (
	github.com/YASSERRMD/verdex/packages/observability v0.0.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	go.opentelemetry.io/otel v1.44.0 // indirect
	go.opentelemetry.io/otel/trace v1.44.0 // indirect
	golang.org/x/crypto v0.51.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/text v0.37.0 // indirect
)
