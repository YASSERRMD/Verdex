package signoff

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

// PendingSignoffEvent carries information about a case entering (or
// remaining in) the "awaiting sign-off" state — either because it was
// never reviewed, or because ReReviewOnCaseUpdate just reverted a
// prior approval back to Pending.
type PendingSignoffEvent struct {
	// TenantID identifies the affected tenant.
	TenantID uuid.UUID `json:"tenant_id"`
	// CaseID identifies the case now awaiting sign-off.
	CaseID uuid.UUID `json:"case_id"`
	// Reason is a short human-readable explanation, e.g. "case never
	// reviewed" or "case content changed after approval".
	Reason string `json:"reason"`
	// CaseVersion is the case's MetadataVersion at the time the event
	// fired.
	CaseVersion int `json:"case_version"`
	// CreatedAt is the UTC time the event was generated.
	CreatedAt time.Time `json:"created_at"`
}

// NotificationSink receives PendingSignoffEvent notifications for
// delivery to an external system (e.g. an email service, a Slack
// webhook, or a task queue), mirroring
// packages/accounting.AlertSink's idiom exactly.
type NotificationSink interface {
	// Notify delivers a PendingSignoffEvent. Implementations should be
	// fast and non-blocking; heavy I/O should be offloaded to a
	// goroutine.
	Notify(ctx context.Context, event PendingSignoffEvent) error
}

// LoggingNotificationSink is a NotificationSink that writes events to
// the standard logger. It is suitable for development, debugging, and
// as a real (not mocked) default in production until a richer channel
// (email, in-app, etc.) is wired in.
type LoggingNotificationSink struct {
	Logger *log.Logger
}

// Notify implements NotificationSink by writing the event to the
// configured logger.
func (s *LoggingNotificationSink) Notify(_ context.Context, event PendingSignoffEvent) error {
	logger := s.Logger
	if logger == nil {
		logger = log.Default()
	}
	logger.Printf(
		"[signoff] pending sign-off tenant=%s case=%s version=%d reason=%q at=%s",
		event.TenantID, event.CaseID, event.CaseVersion, event.Reason,
		event.CreatedAt.UTC().Format(time.RFC3339),
	)
	return nil
}

// NoOpNotificationSink is a NotificationSink that silently discards
// every event. It is used when notification delivery is not required
// (e.g. in tests that do not assert on notifications).
type NoOpNotificationSink struct{}

// Notify implements NotificationSink by doing nothing.
func (NoOpNotificationSink) Notify(_ context.Context, _ PendingSignoffEvent) error { return nil }

// MultiNotificationSink fans out to multiple NotificationSink
// implementations. The first error encountered is returned but all
// sinks are still attempted, mirroring
// packages/accounting.MultiAlertSink.
type MultiNotificationSink struct {
	Sinks []NotificationSink
}

// Notify implements NotificationSink by calling Notify on each child
// sink.
func (m *MultiNotificationSink) Notify(ctx context.Context, event PendingSignoffEvent) error {
	var first error
	for _, s := range m.Sinks {
		if err := s.Notify(ctx, event); err != nil && first == nil {
			first = fmt.Errorf("signoff: notification sink error: %w", err)
		}
	}
	return first
}
