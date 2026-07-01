module github.com/YASSERRMD/verdex/packages/eval

go 1.25.0

replace (
	github.com/YASSERRMD/verdex/packages/prompts => ../prompts
	github.com/YASSERRMD/verdex/packages/provider => ../provider
)

require github.com/YASSERRMD/verdex/packages/provider v0.0.0-00010101000000-000000000000
