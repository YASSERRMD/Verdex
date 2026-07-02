module github.com/YASSERRMD/verdex/packages/firstpartyagent

go 1.25.0

replace (
	github.com/YASSERRMD/verdex/packages/adaptiveretrieval => ../adaptiveretrieval
	github.com/YASSERRMD/verdex/packages/agentframework => ../agentframework
	github.com/YASSERRMD/verdex/packages/citation => ../citation
	github.com/YASSERRMD/verdex/packages/config => ../config
	github.com/YASSERRMD/verdex/packages/embedding => ../embedding
	github.com/YASSERRMD/verdex/packages/gateway => ../gateway
	github.com/YASSERRMD/verdex/packages/graph => ../graph
	github.com/YASSERRMD/verdex/packages/hybridretrieval => ../hybridretrieval
	github.com/YASSERRMD/verdex/packages/identity => ../identity
	github.com/YASSERRMD/verdex/packages/irac => ../irac
	github.com/YASSERRMD/verdex/packages/issueagent => ../issueagent
	github.com/YASSERRMD/verdex/packages/knowledgeapi => ../knowledgeapi
	github.com/YASSERRMD/verdex/packages/knowledgeisolation => ../knowledgeisolation
	github.com/YASSERRMD/verdex/packages/observability => ../observability
	github.com/YASSERRMD/verdex/packages/persistence => ../persistence
	github.com/YASSERRMD/verdex/packages/prompts => ../prompts
	github.com/YASSERRMD/verdex/packages/provider => ../provider
	github.com/YASSERRMD/verdex/packages/router => ../router
	github.com/YASSERRMD/verdex/packages/tenancy => ../tenancy
	github.com/YASSERRMD/verdex/packages/traversal => ../traversal
	github.com/YASSERRMD/verdex/packages/treeassembly => ../treeassembly
	github.com/YASSERRMD/verdex/packages/treeindex => ../treeindex
	github.com/YASSERRMD/verdex/packages/treevalidation => ../treevalidation
	github.com/YASSERRMD/verdex/packages/vectorindex => ../vectorindex
)

require (
	github.com/YASSERRMD/verdex/packages/agentframework v0.0.0-00010101000000-000000000000
	github.com/YASSERRMD/verdex/packages/irac v0.0.0
	github.com/YASSERRMD/verdex/packages/issueagent v0.0.0-00010101000000-000000000000
	github.com/YASSERRMD/verdex/packages/knowledgeapi v0.0.0-00010101000000-000000000000
	github.com/YASSERRMD/verdex/packages/prompts v0.0.0
	github.com/YASSERRMD/verdex/packages/provider v0.0.0
	github.com/YASSERRMD/verdex/packages/router v0.0.0-00010101000000-000000000000
)

require (
	github.com/YASSERRMD/verdex/packages/citation v0.0.0-00010101000000-000000000000 // indirect
	github.com/YASSERRMD/verdex/packages/config v0.0.0 // indirect
	github.com/YASSERRMD/verdex/packages/embedding v0.0.0-00010101000000-000000000000 // indirect
	github.com/YASSERRMD/verdex/packages/gateway v0.0.0-00010101000000-000000000000 // indirect
	github.com/YASSERRMD/verdex/packages/graph v0.0.0-00010101000000-000000000000 // indirect
	github.com/YASSERRMD/verdex/packages/hybridretrieval v0.0.0-00010101000000-000000000000 // indirect
	github.com/YASSERRMD/verdex/packages/identity v0.0.0-00010101000000-000000000000 // indirect
	github.com/YASSERRMD/verdex/packages/knowledgeisolation v0.0.0-00010101000000-000000000000 // indirect
	github.com/YASSERRMD/verdex/packages/persistence v0.0.0 // indirect
	github.com/YASSERRMD/verdex/packages/traversal v0.0.0-00010101000000-000000000000 // indirect
	github.com/YASSERRMD/verdex/packages/treeassembly v0.0.0-00010101000000-000000000000 // indirect
	github.com/YASSERRMD/verdex/packages/treeindex v0.0.0-00010101000000-000000000000 // indirect
	github.com/YASSERRMD/verdex/packages/treevalidation v0.0.0-00010101000000-000000000000 // indirect
	github.com/YASSERRMD/verdex/packages/vectorindex v0.0.0-00010101000000-000000000000 // indirect
	github.com/golang-migrate/migrate/v4 v4.19.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.10.0 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/lib/pq v1.10.9 // indirect
	github.com/neo4j/neo4j-go-driver/v5 v5.28.4 // indirect
	github.com/pgvector/pgvector-go v0.4.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
