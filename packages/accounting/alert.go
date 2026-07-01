package accounting

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

// AlertType constants for AlertEvent.AlertType.
const (
	AlertTypeBudgetWarning  = "budget_warning"
	AlertTypeBudgetExceeded = "budget_exceeded"
)

// AlertEvent carries information about a budget threshold crossing.
type AlertEvent struct {
	// TenantID identifies the affected tenant.
	TenantID uuid.UUID `json:"tenant_id"`
	// AlertType is one of AlertTypeBudgetWarning or AlertTypeBudgetExceeded.
	AlertType string `json:"alert_type"`
	// CurrentUsage is the total tokens consumed in the current period.
	CurrentUsage int `json:"current_usage"`
	// Limit is the configured token limit that was approached or breached.
	Limit int `json:"limit"`
	// CurrentCostUSD is the accumulated cost in the current period.
	CurrentCostUSD float64 `json:"current_cost_usd"`
	// LimitUSD is the configured cost limit that was approached or breached.
	LimitUSD float64 `json:"limit_usd"`
	// CreatedAt is the UTC time at which the alert was generated.
	CreatedAt time.Time `json:"created_at"`
}

// AlertSink receives alert events for delivery to an external system (e.g. an
// email service, a Slack webhook, or a metrics platform).
type AlertSink interface {
	// Send delivers an AlertEvent.  Implementations should be fast and
	// non-blocking; heavy I/O should be offloaded to a goroutine.
	Send(ctx context.Context, event AlertEvent) error
}

// LoggingAlertSink is an AlertSink that writes events to the standard logger.
// It is suitable for development and debugging.
type LoggingAlertSink struct {
	Logger *log.Logger
}

// Send implements AlertSink by writing the event to the configured logger.
func (s *LoggingAlertSink) Send(_ context.Context, event AlertEvent) error {
	logger := s.Logger
	if logger == nil {
		logger = log.Default()
	}
	logger.Printf(
		"[accounting] alert type=%s tenant=%s current_tokens=%d limit=%d current_cost=%.6f limit_cost=%.6f at=%s",
		event.AlertType, event.TenantID, event.CurrentUsage, event.Limit,
		event.CurrentCostUSD, event.LimitUSD, event.CreatedAt.UTC().Format(time.RFC3339),
	)
	return nil
}

// NoOpAlertSink is an AlertSink that silently discards every event.
// It is used when alert delivery is not required.
type NoOpAlertSink struct{}

// Send implements AlertSink by doing nothing.
func (NoOpAlertSink) Send(_ context.Context, _ AlertEvent) error { return nil }

// MultiAlertSink fans out to multiple AlertSink implementations.  The first
// error encountered is returned but all sinks are still attempted.
type MultiAlertSink struct {
	Sinks []AlertSink
}

// Send implements AlertSink by calling Send on each child sink.
func (m *MultiAlertSink) Send(ctx context.Context, event AlertEvent) error {
	var first error
	for _, s := range m.Sinks {
		if err := s.Send(ctx, event); err != nil && first == nil {
			first = fmt.Errorf("accounting: alert sink error: %w", err)
		}
	}
	return first
}
