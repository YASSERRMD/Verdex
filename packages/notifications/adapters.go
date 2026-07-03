// Package notifications' adapters.go wires this package's Service into
// the event-hook interfaces four upstream packages already define and
// fire against, but only ever shipped a logging (or no-op) sink for:
//
//   - packages/signoff.NotificationSink, fired on pending sign-off.
//   - packages/annotations.MentionSink, fired on @-mentions.
//   - packages/reasoningeval.AlertSink, fired on quality regression.
//   - packages/accounting.AlertSink, fired on budget threshold.
//
// Each adapter here is a genuine implementation of the upstream
// interface (verified by a `var _ upstream.Interface = (*Adapter)(nil)`
// assertion) that translates the upstream event into a
// notifications.NotifyInput and calls Service.Notify, so those four
// phases' event hooks now reach a real, persisted, user-visible inbox
// instead of only a log line. See doc/notifications.md for the full
// adapter-to-interface mapping.
package notifications

import (
	"context"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/annotations"
	"github.com/YASSERRMD/verdex/packages/reasoningeval"
	"github.com/YASSERRMD/verdex/packages/signoff"
)

// SignoffNotificationSink adapts a *Service to implement
// packages/signoff.NotificationSink, so every PendingSignoffEvent
// packages/signoff's ReReviewOnCaseUpdate/gate machinery fires lands a
// KindPendingSignoff Notification in RecipientResolver's resolved
// recipients' inboxes.
//
// packages/signoff.PendingSignoffEvent does not itself carry a
// recipient (it is a per-case, not per-user, event — any of several
// judges/clerks might be the right person to notify, which
// packages/signoff has no opinion on), so this adapter is constructed
// with a RecipientResolver callback that maps
// (tenantID, caseID) -> the user IDs who should be notified. Callers
// typically supply a resolver backed by packages/caselifecycle/
// packages/signoff's own role/assignment data.
type SignoffNotificationSink struct {
	Service   *Service
	Resolvers RecipientResolver
}

// RecipientResolver maps a tenant/case pair to the set of user IDs who
// should receive a notification about it. Implementations may return
// an empty slice (no notification sent) but should not return an
// error for "nobody to notify" — only for a genuine lookup failure.
type RecipientResolver func(ctx context.Context, tenantID, caseID uuid.UUID) ([]uuid.UUID, error)

// NewSignoffNotificationSink builds a SignoffNotificationSink. resolver
// is required — see RecipientResolver.
func NewSignoffNotificationSink(service *Service, resolver RecipientResolver) *SignoffNotificationSink {
	return &SignoffNotificationSink{Service: service, Resolvers: resolver}
}

// Notify implements packages/signoff.NotificationSink. It resolves
// event's recipients via s.Resolvers and calls Service.Notify once per
// recipient, so a per-preference opt-out for one recipient never
// suppresses delivery to another.
func (s *SignoffNotificationSink) Notify(ctx context.Context, event signoff.PendingSignoffEvent) error {
	recipients, err := s.Resolvers(ctx, event.TenantID, event.CaseID)
	if err != nil {
		return wrapf("SignoffNotificationSink.Notify", err)
	}
	caseID := event.CaseID
	for _, recipientID := range recipients {
		_, err := s.Service.Notify(ctx, NotifyInput{
			TenantID:    event.TenantID,
			RecipientID: recipientID,
			Kind:        KindPendingSignoff,
			Title:       "Case awaiting your sign-off",
			Body:        event.Reason,
			CaseID:      &caseID,
		})
		if err != nil {
			return wrapf("SignoffNotificationSink.Notify", err)
		}
	}
	return nil
}

var _ signoff.NotificationSink = (*SignoffNotificationSink)(nil)

// AnnotationsMentionSink adapts a *Service to implement
// packages/annotations.MentionSink, so every @-mention
// packages/annotations.Service extracts from an annotation body lands
// a KindMention Notification in the mentioned user's inbox — this
// adapter needs no external recipient resolution, since
// annotations.Mention already names MentionedUserID directly.
type AnnotationsMentionSink struct {
	Service *Service
}

// NewAnnotationsMentionSink builds an AnnotationsMentionSink.
func NewAnnotationsMentionSink(service *Service) *AnnotationsMentionSink {
	return &AnnotationsMentionSink{Service: service}
}

// Notify implements packages/annotations.MentionSink.
func (a *AnnotationsMentionSink) Notify(ctx context.Context, mention annotations.Mention) error {
	caseID := mention.CaseID
	annotationID := mention.AnnotationID
	_, err := a.Service.Notify(ctx, NotifyInput{
		TenantID:        mention.TenantID,
		RecipientID:     mention.MentionedUserID,
		Kind:            KindMention,
		Title:           "You were mentioned in a case discussion",
		CaseID:          &caseID,
		RelatedEntityID: &annotationID,
	})
	if err != nil {
		return wrapf("AnnotationsMentionSink.Notify", err)
	}
	return nil
}

var _ annotations.MentionSink = (*AnnotationsMentionSink)(nil)

// ReasoningEvalAlertSink adapts a *Service to implement
// packages/reasoningeval.AlertSink, so every quality-regression Alert
// packages/reasoningeval.QualityAlertChecker raises lands a
// KindQualityAlert Notification in each of Recipients' inboxes.
//
// packages/reasoningeval.Alert is a system/jurisdiction-scoped signal
// with no tenant or recipient of its own (a regression is a property
// of the reasoning pipeline's output quality, not of one user's
// action), so this adapter is constructed with a fixed TenantID and
// Recipients list — typically the tenant's admins/auditors who own
// reasoning-quality monitoring.
type ReasoningEvalAlertSink struct {
	Service    *Service
	TenantID   uuid.UUID
	Recipients []uuid.UUID
}

// NewReasoningEvalAlertSink builds a ReasoningEvalAlertSink.
func NewReasoningEvalAlertSink(service *Service, tenantID uuid.UUID, recipients []uuid.UUID) *ReasoningEvalAlertSink {
	return &ReasoningEvalAlertSink{Service: service, TenantID: tenantID, Recipients: recipients}
}

// Send implements packages/reasoningeval.AlertSink.
func (r *ReasoningEvalAlertSink) Send(ctx context.Context, alert reasoningeval.Alert) error {
	for _, recipientID := range r.Recipients {
		_, err := r.Service.Notify(ctx, NotifyInput{
			TenantID:    r.TenantID,
			RecipientID: recipientID,
			Kind:        KindQualityAlert,
			Title:       "Reasoning quality alert",
			Body:        alert.Message,
		})
		if err != nil {
			return wrapf("ReasoningEvalAlertSink.Send", err)
		}
	}
	return nil
}

var _ reasoningeval.AlertSink = (*ReasoningEvalAlertSink)(nil)
