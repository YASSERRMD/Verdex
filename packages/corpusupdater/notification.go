package corpusupdater

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// AffectedCaseResolver resolves the case IDs that reference a given
// rule/precedent target, letting Engine.ApplyAmendment name "who is
// affected" in a ChangeNotification without this package querying the
// knowledge graph directly (task 5). A nil AffectedCaseResolver yields
// an empty AffectedCaseIDs list on every ChangeNotification -- useful
// for callers that don't yet have a resolver wired up; production
// callers should supply one backed by packages/graph or
// packages/knowledgeapi.
type AffectedCaseResolver func(ctx context.Context, corpus CorpusTarget, targetID string) ([]uuid.UUID, error)

// ChangeNotification is the event Engine.ApplyAmendment fires via
// NotificationSink once an Amendment affecting a rule/precedent goes
// live (its EffectiveDate <= the applying instant).
type ChangeNotification struct {
	// TenantID scopes this notification to a tenant.
	TenantID uuid.UUID

	// JobID is the CorpusUpdateJob the amendment belongs to.
	JobID uuid.UUID

	// AmendmentID is the Amendment that went live.
	AmendmentID uuid.UUID

	// TargetCorpus and TargetID name the rule/precedent that changed.
	TargetCorpus CorpusTarget
	TargetID     string

	// ChangeType names the kind of change applied.
	ChangeType ChangeType

	// Citation is the amending instrument's citation.
	Citation string

	// AffectedCaseIDs lists the cases AffectedCaseResolver named as
	// referencing TargetID. Empty if no resolver was configured or the
	// resolver found no references.
	AffectedCaseIDs []uuid.UUID

	// OccurredAt is when this amendment was applied.
	OccurredAt time.Time
}

// NotificationSink receives a ChangeNotification whenever an applied
// Amendment goes live. Composing conceptually with
// packages/notifications (Phase 072) by shape, not by import --
// exactly the seam packages/signoff.NotificationSink and
// packages/annotations.MentionSink already establish: this package
// fires the event, a notifications-side adapter (mirroring
// notifications.SignoffNotificationSink) is the real sink that lands
// it in a user's inbox.
type NotificationSink interface {
	// NotifyChange delivers n. Implementations should treat delivery
	// failure as non-fatal to the corpus update itself -- Engine logs
	// but does not fail ApplyAmendment solely because notification
	// delivery returned an error, mirroring how packages/notifications
	// itself never lets a stub channel failure roll back the in-app
	// Notification write.
	NotifyChange(ctx context.Context, n ChangeNotification) error
}
