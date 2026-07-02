module github.com/YASSERRMD/verdex/packages/ingestion

go 1.25.0

require (
	github.com/YASSERRMD/verdex/packages/evidence v0.0.0
	github.com/YASSERRMD/verdex/packages/intake v0.0.0-00010101000000-000000000000
	github.com/YASSERRMD/verdex/packages/multilingual v0.0.0-00010101000000-000000000000
	github.com/YASSERRMD/verdex/packages/ocr v0.0.0
	github.com/YASSERRMD/verdex/packages/segmentation v0.0.0
	github.com/YASSERRMD/verdex/packages/stt v0.0.0
	github.com/google/uuid v1.6.0
)

require (
	github.com/YASSERRMD/verdex/packages/config v0.0.0 // indirect
	github.com/YASSERRMD/verdex/packages/persistence v0.0.0 // indirect
	github.com/YASSERRMD/verdex/packages/tenancy v0.0.0 // indirect
	github.com/golang-migrate/migrate/v4 v4.19.1 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.10.0 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/lib/pq v1.10.9 // indirect
	github.com/neo4j/neo4j-go-driver/v5 v5.28.4 // indirect
	github.com/pgvector/pgvector-go v0.4.0 // indirect
	golang.org/x/sync v0.21.0 // indirect
	golang.org/x/text v0.38.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/YASSERRMD/verdex/packages/config => ../config
	github.com/YASSERRMD/verdex/packages/evidence => ../evidence
	github.com/YASSERRMD/verdex/packages/intake => ../intake
	github.com/YASSERRMD/verdex/packages/multilingual => ../multilingual
	github.com/YASSERRMD/verdex/packages/observability => ../observability
	github.com/YASSERRMD/verdex/packages/ocr => ../ocr
	github.com/YASSERRMD/verdex/packages/persistence => ../persistence
	github.com/YASSERRMD/verdex/packages/segmentation => ../segmentation
	github.com/YASSERRMD/verdex/packages/stt => ../stt
	github.com/YASSERRMD/verdex/packages/tenancy => ../tenancy
)
