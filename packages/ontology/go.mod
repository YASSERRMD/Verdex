module github.com/YASSERRMD/verdex/packages/ontology

go 1.25.0

require (
	github.com/YASSERRMD/verdex/packages/category v0.0.0
	github.com/YASSERRMD/verdex/packages/irac v0.0.0
)

replace (
	github.com/YASSERRMD/verdex/packages/category => ../category
	github.com/YASSERRMD/verdex/packages/irac => ../irac
)
