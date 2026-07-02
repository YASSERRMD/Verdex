module github.com/YASSERRMD/verdex/packages/graph

go 1.25.0

require github.com/YASSERRMD/verdex/packages/irac v0.0.0

require github.com/neo4j/neo4j-go-driver/v5 v5.28.4

replace (
	github.com/YASSERRMD/verdex/packages/irac => ../irac
	github.com/YASSERRMD/verdex/packages/persistence => ../persistence
)
