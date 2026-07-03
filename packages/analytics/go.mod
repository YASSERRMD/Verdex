module github.com/YASSERRMD/verdex/packages/analytics

go 1.25.0

replace (
	github.com/YASSERRMD/verdex/packages/accounting => ../accounting
	github.com/YASSERRMD/verdex/packages/adaptiveretrieval => ../adaptiveretrieval
	github.com/YASSERRMD/verdex/packages/agentframework => ../agentframework
	github.com/YASSERRMD/verdex/packages/caselifecycle => ../caselifecycle
	github.com/YASSERRMD/verdex/packages/citation => ../citation
	github.com/YASSERRMD/verdex/packages/config => ../config
	github.com/YASSERRMD/verdex/packages/embedding => ../embedding
	github.com/YASSERRMD/verdex/packages/evidenceweighing => ../evidenceweighing
	github.com/YASSERRMD/verdex/packages/firstpartyagent => ../firstpartyagent
	github.com/YASSERRMD/verdex/packages/gateway => ../gateway
	github.com/YASSERRMD/verdex/packages/graph => ../graph
	github.com/YASSERRMD/verdex/packages/grounding => ../grounding
	github.com/YASSERRMD/verdex/packages/guardrail => ../guardrail
	github.com/YASSERRMD/verdex/packages/hybridretrieval => ../hybridretrieval
	github.com/YASSERRMD/verdex/packages/identity => ../identity
	github.com/YASSERRMD/verdex/packages/irac => ../irac
	github.com/YASSERRMD/verdex/packages/issueagent => ../issueagent
	github.com/YASSERRMD/verdex/packages/jurisdiction => ../jurisdiction
	github.com/YASSERRMD/verdex/packages/knowledgeapi => ../knowledgeapi
	github.com/YASSERRMD/verdex/packages/knowledgeisolation => ../knowledgeisolation
	github.com/YASSERRMD/verdex/packages/lawapplication => ../lawapplication
	github.com/YASSERRMD/verdex/packages/observability => ../observability
	github.com/YASSERRMD/verdex/packages/persistence => ../persistence
	github.com/YASSERRMD/verdex/packages/prompts => ../prompts
	github.com/YASSERRMD/verdex/packages/provider => ../provider
	github.com/YASSERRMD/verdex/packages/reasoningeval => ../reasoningeval
	github.com/YASSERRMD/verdex/packages/reasoningprofile => ../reasoningprofile
	github.com/YASSERRMD/verdex/packages/router => ../router
	github.com/YASSERRMD/verdex/packages/secondpartyagent => ../secondpartyagent
	github.com/YASSERRMD/verdex/packages/synthesisagent => ../synthesisagent
	github.com/YASSERRMD/verdex/packages/tenancy => ../tenancy
	github.com/YASSERRMD/verdex/packages/traversal => ../traversal
	github.com/YASSERRMD/verdex/packages/treeassembly => ../treeassembly
	github.com/YASSERRMD/verdex/packages/treeindex => ../treeindex
	github.com/YASSERRMD/verdex/packages/treevalidation => ../treevalidation
	github.com/YASSERRMD/verdex/packages/vectorindex => ../vectorindex
)

require (
	github.com/YASSERRMD/verdex/packages/accounting v0.0.0
	github.com/YASSERRMD/verdex/packages/caselifecycle v0.0.0
	github.com/YASSERRMD/verdex/packages/identity v0.0.0
	github.com/YASSERRMD/verdex/packages/reasoningeval v0.0.0
	github.com/google/uuid v1.6.0
)

require (
	github.com/YASSERRMD/verdex/packages/agentframework v0.0.0-00010101000000-000000000000 // indirect
	github.com/YASSERRMD/verdex/packages/citation v0.0.0-00010101000000-000000000000 // indirect
	github.com/YASSERRMD/verdex/packages/config v0.0.0 // indirect
	github.com/YASSERRMD/verdex/packages/embedding v0.0.0-00010101000000-000000000000 // indirect
	github.com/YASSERRMD/verdex/packages/evidenceweighing v0.0.0-00010101000000-000000000000 // indirect
	github.com/YASSERRMD/verdex/packages/firstpartyagent v0.0.0-00010101000000-000000000000 // indirect
	github.com/YASSERRMD/verdex/packages/gateway v0.0.0-00010101000000-000000000000 // indirect
	github.com/YASSERRMD/verdex/packages/graph v0.0.0-20260702094313-4d74dfa6d3e6 // indirect
	github.com/YASSERRMD/verdex/packages/grounding v0.0.0-00010101000000-000000000000 // indirect
	github.com/YASSERRMD/verdex/packages/guardrail v0.0.0-00010101000000-000000000000 // indirect
	github.com/YASSERRMD/verdex/packages/hybridretrieval v0.0.0-00010101000000-000000000000 // indirect
	github.com/YASSERRMD/verdex/packages/irac v0.0.0 // indirect
	github.com/YASSERRMD/verdex/packages/issueagent v0.0.0-00010101000000-000000000000 // indirect
	github.com/YASSERRMD/verdex/packages/knowledgeapi v0.0.0-00010101000000-000000000000 // indirect
	github.com/YASSERRMD/verdex/packages/knowledgeisolation v0.0.0-00010101000000-000000000000 // indirect
	github.com/YASSERRMD/verdex/packages/lawapplication v0.0.0-00010101000000-000000000000 // indirect
	github.com/YASSERRMD/verdex/packages/observability v0.0.0 // indirect
	github.com/YASSERRMD/verdex/packages/persistence v0.0.0 // indirect
	github.com/YASSERRMD/verdex/packages/prompts v0.0.0 // indirect
	github.com/YASSERRMD/verdex/packages/provider v0.0.0 // indirect
	github.com/YASSERRMD/verdex/packages/router v0.0.0-00010101000000-000000000000 // indirect
	github.com/YASSERRMD/verdex/packages/secondpartyagent v0.0.0-00010101000000-000000000000 // indirect
	github.com/YASSERRMD/verdex/packages/synthesisagent v0.0.0-00010101000000-000000000000 // indirect
	github.com/YASSERRMD/verdex/packages/tenancy v0.0.0 // indirect
	github.com/YASSERRMD/verdex/packages/traversal v0.0.0-00010101000000-000000000000 // indirect
	github.com/YASSERRMD/verdex/packages/treeassembly v0.0.0-00010101000000-000000000000 // indirect
	github.com/YASSERRMD/verdex/packages/treeindex v0.0.0-00010101000000-000000000000 // indirect
	github.com/YASSERRMD/verdex/packages/treevalidation v0.0.0-00010101000000-000000000000 // indirect
	github.com/YASSERRMD/verdex/packages/vectorindex v0.0.0-00010101000000-000000000000 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/golang-migrate/migrate/v4 v4.19.1 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.10.0 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/lib/pq v1.10.9 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/neo4j/neo4j-go-driver/v5 v5.28.4 // indirect
	github.com/pgvector/pgvector-go v0.4.0 // indirect
	github.com/prometheus/client_golang v1.23.2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.66.1 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel v1.44.0 // indirect
	go.opentelemetry.io/otel/metric v1.44.0 // indirect
	go.opentelemetry.io/otel/sdk v1.44.0 // indirect
	go.opentelemetry.io/otel/trace v1.44.0 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	google.golang.org/protobuf v1.36.8 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
