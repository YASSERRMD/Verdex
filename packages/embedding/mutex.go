package embedding

import "sync"

// newMu returns a *sync.Mutex wrapped as the interface expected by
// AccumulatingMetricsSink.  Kept in a separate file to avoid cluttering
// metrics.go.
func newMu() interface{ Lock(); Unlock() } {
	return &sync.Mutex{}
}
