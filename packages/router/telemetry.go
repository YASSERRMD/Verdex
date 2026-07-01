package router

import (
	"fmt"
	"time"
)

// RouterEvent captures the outcome of a single provider attempt made by the
// router.
type RouterEvent struct {
	// TaskType is the kind of task that was attempted.
	TaskType TaskType
	// ProviderID is the provider that was tried.
	ProviderID string
	// Success indicates whether the provider returned a successful response.
	Success bool
	// Latency is the wall-clock time from request dispatch to response (or
	// error) receipt.
	Latency time.Duration
	// Attempt is the 1-based attempt number within the current router call
	// (1 = first provider tried, 2 = first fallback, etc.).
	Attempt int
	// TenantID is the tenant on whose behalf the call was made.
	TenantID string
}

// TelemetrySink receives RouterEvent values emitted by the router.
type TelemetrySink interface {
	// Record is called once per provider attempt.  Implementations must be
	// safe for concurrent use from multiple goroutines.
	Record(event RouterEvent)
}

// NoOpTelemetrySink discards all events.  It is the default sink when none
// is provided.
type NoOpTelemetrySink struct{}

// Record implements TelemetrySink by doing nothing.
func (NoOpTelemetrySink) Record(RouterEvent) {}

// LoggingTelemetrySink writes a human-readable line per event to any
// function that accepts a string (typically log.Println or fmt.Println).
type LoggingTelemetrySink struct {
	// Logf is called with a pre-formatted log line for each event.  If nil,
	// fmt.Println is used.
	Logf func(line string)
}

// Record implements TelemetrySink by formatting and logging the event.
func (s *LoggingTelemetrySink) Record(e RouterEvent) {
	status := "ok"
	if !e.Success {
		status = "fail"
	}
	line := fmt.Sprintf(
		"[router] tenant=%s task=%s attempt=%d provider=%s status=%s latency=%s",
		e.TenantID, e.TaskType, e.Attempt, e.ProviderID, status, e.Latency,
	)
	if s.Logf != nil {
		s.Logf(line)
	} else {
		fmt.Println(line)
	}
}
