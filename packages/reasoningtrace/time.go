package reasoningtrace

import "time"

// nowFunc is overridden in tests for deterministic timestamps, mirroring
// packages/reasoningorchestration's own nowFunc convention.
var nowFunc = time.Now
