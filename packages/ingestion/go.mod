module github.com/YASSERRMD/verdex/packages/ingestion

go 1.25.0

require (
	github.com/YASSERRMD/verdex/packages/evidence v0.0.0
	github.com/YASSERRMD/verdex/packages/ocr v0.0.0
	github.com/YASSERRMD/verdex/packages/segmentation v0.0.0
	github.com/YASSERRMD/verdex/packages/stt v0.0.0
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
