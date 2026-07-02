module github.com/YASSERRMD/verdex/packages/reasoningprofile

go 1.25.0

replace (
	github.com/YASSERRMD/verdex/packages/evidenceweighing => ../evidenceweighing
	github.com/YASSERRMD/verdex/packages/jurisdiction => ../jurisdiction
	github.com/YASSERRMD/verdex/packages/lawapplication => ../lawapplication
)

require (
	github.com/YASSERRMD/verdex/packages/evidenceweighing v0.0.0-00010101000000-000000000000
	github.com/YASSERRMD/verdex/packages/jurisdiction v0.0.0
	github.com/YASSERRMD/verdex/packages/lawapplication v0.0.0-00010101000000-000000000000
)

require github.com/google/uuid v1.6.0 // indirect
