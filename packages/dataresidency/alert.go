package dataresidency

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

// ViolationType constants for ViolationEvent.ViolationType.
const (
	// ViolationTransferBlocked fires when CheckTransfer or
	// CheckProviderLocality rejects a cross-region operation.
	ViolationTransferBlocked = "transfer_blocked"

	// ViolationVerifyFailed fires when Verify finds the live
	// configuration does not satisfy the deployment's ResidencyPolicy.
	ViolationVerifyFailed = "verify_failed"
)

// ViolationEvent carries information about a residency policy
// violation, mirroring packages/accounting.AlertEvent's shape (task 8:
// "wire a violation into an AlertSink-shaped hook").
type ViolationEvent struct {
	// DeploymentID identifies the deployment the violation occurred on.
	DeploymentID uuid.UUID `json:"deployment_id"`
	// TenantID identifies the owning tenant, when known.
	TenantID uuid.UUID `json:"tenant_id,omitempty"`
	// ViolationType is one of the Violation* constants above.
	ViolationType string `json:"violation_type"`
	// SourceRegion and DestRegion describe the rejected transfer, when
	// ViolationType is ViolationTransferBlocked. Empty for
	// ViolationVerifyFailed.
	SourceRegion string `json:"source_region,omitempty"`
	DestRegion   string `json:"dest_region,omitempty"`
	// Reason is a short human-readable explanation (typically the
	// guard function's error message).
	Reason string `json:"reason"`
	// CreatedAt is the UTC time the violation was detected.
	CreatedAt time.Time `json:"created_at"`
}

// AlertSink receives ViolationEvents for delivery to an external
// system (e.g. an email service, a Slack webhook, or a metrics
// platform), mirroring packages/accounting.AlertSink and
// packages/signoff's notification idiom exactly so operators already
// familiar with those packages recognize this one immediately.
type AlertSink interface {
	// Send delivers a ViolationEvent. Implementations should be fast
	// and non-blocking; heavy I/O should be offloaded to a goroutine.
	Send(ctx context.Context, event ViolationEvent) error
}

// LoggingAlertSink is an AlertSink that writes events to the standard
// logger -- a real, working implementation (task 8 requires one, not
// just an interface), suitable for development and as the default
// wired-in sink until an operator configures a richer delivery
// mechanism.
type LoggingAlertSink struct {
	Logger *log.Logger
}

// Send implements AlertSink by writing the event to the configured
// logger.
func (s *LoggingAlertSink) Send(_ context.Context, event ViolationEvent) error {
	logger := s.Logger
	if logger == nil {
		logger = log.Default()
	}
	logger.Printf(
		"[dataresidency] VIOLATION type=%s deployment=%s tenant=%s source=%s dest=%s reason=%q at=%s",
		event.ViolationType, event.DeploymentID, event.TenantID,
		event.SourceRegion, event.DestRegion, event.Reason,
		event.CreatedAt.UTC().Format(time.RFC3339),
	)
	return nil
}

// NoOpAlertSink is an AlertSink that silently discards every event.
type NoOpAlertSink struct{}

// Send implements AlertSink by doing nothing.
func (NoOpAlertSink) Send(_ context.Context, _ ViolationEvent) error { return nil }

// MultiAlertSink fans out to multiple AlertSink implementations. The
// first error encountered is returned but all sinks are still
// attempted, mirroring packages/accounting.MultiAlertSink.
type MultiAlertSink struct {
	Sinks []AlertSink
}

// Send implements AlertSink by calling Send on each child sink.
func (m *MultiAlertSink) Send(ctx context.Context, event ViolationEvent) error {
	var first error
	for _, s := range m.Sinks {
		if err := s.Send(ctx, event); err != nil && first == nil {
			first = fmt.Errorf("dataresidency: alert sink error: %w", err)
		}
	}
	return first
}
