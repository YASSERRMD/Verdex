package notifications

import (
	"time"

	"github.com/google/uuid"
)

// Kind classifies what event a Notification represents, mirroring how
// packages/caseversioning.ArtifactKind and packages/reasoningeval.AlertKind
// use a small closed string enum rather than a free-form category field.
type Kind string

const (
	// KindIngestionComplete fires when an evidence ingestion run
	// (packages/ingestion) finishes processing for a case. This package
	// does not call into packages/ingestion itself — see doc/notifications.md,
	// "Ingestion-complete" — it only defines the Kind and the Notify
	// entrypoint packages/ingestion can eventually call.
	KindIngestionComplete Kind = "ingestion_complete"

	// KindPendingSignoff fires when a case enters (or remains in) the
	// "awaiting sign-off" state, mirroring packages/signoff.PendingSignoffEvent.
	// Delivered via SignoffNotificationSink, which implements
	// packages/signoff.NotificationSink.
	KindPendingSignoff Kind = "pending_signoff"

	// KindMention fires when an annotation's body mentions a user,
	// mirroring packages/annotations.Mention. Delivered via
	// AnnotationsMentionSink, which implements packages/annotations.MentionSink.
	KindMention Kind = "mention"

	// KindQualityAlert fires when packages/reasoningeval detects a
	// reasoning-quality regression, mirroring packages/reasoningeval.Alert.
	// Delivered via ReasoningEvalAlertSink, which implements
	// packages/reasoningeval.AlertSink.
	KindQualityAlert Kind = "quality_alert"

	// KindBudgetAlert fires when packages/accounting detects a budget
	// threshold crossing, mirroring packages/accounting.AlertEvent.
	// Delivered via AccountingAlertSink, which implements
	// packages/accounting.AlertSink.
	KindBudgetAlert Kind = "budget_alert"

	// KindTaskAssignment fires when a user is assigned to review or act
	// on a case (e.g. "you are assigned to review case X"). No new
	// assignment engine is introduced by this package — Notify is simply
	// the entrypoint packages/caselifecycle/packages/signoff-adjacent
	// call sites can invoke once they decide who is assigned.
	KindTaskAssignment Kind = "task_assignment"
)

// allKinds is the exhaustive set of recognized Kind values, used by
// IsValid.
var allKinds = map[Kind]struct{}{
	KindIngestionComplete: {},
	KindPendingSignoff:    {},
	KindMention:           {},
	KindQualityAlert:      {},
	KindBudgetAlert:       {},
	KindTaskAssignment:    {},
}

// IsValid reports whether k is one of the recognized Kind constants.
func (k Kind) IsValid() bool {
	_, ok := allKinds[k]
	return ok
}

// String satisfies fmt.Stringer.
func (k Kind) String() string { return string(k) }

// Channel identifies a delivery channel a Notification can be pushed
// through. In-app (the persisted Notification row itself, always
// delivered) is not a Channel value — it is the baseline every
// Notification gets. Channel values name the *additional* channels
// preferences can opt a Kind into.
type Channel string

const (
	// ChannelEmail is a no-op/logged stub — see EmailChannel — until a
	// real email transport is wired in.
	ChannelEmail Channel = "email"

	// ChannelPush is a no-op/logged stub — see PushChannel — until a
	// real push-notification transport is wired in.
	ChannelPush Channel = "push"
)

// allChannels is the exhaustive set of recognized Channel values, used
// by IsValid.
var allChannels = map[Channel]struct{}{
	ChannelEmail: {},
	ChannelPush:  {},
}

// IsValid reports whether c is one of the recognized Channel constants.
func (c Channel) IsValid() bool {
	_, ok := allChannels[c]
	return ok
}

// String satisfies fmt.Stringer.
func (c Channel) String() string { return string(c) }

// Notification is a single, persisted, user-facing notice: the
// central entity this package adds. Every upstream event-hook package
// (packages/signoff, packages/annotations, packages/reasoningeval,
// packages/accounting) already emits a strongly-typed event of its
// own; an adapter in this package (see adapters.go) translates that
// event into a Notification and calls Notify, which is the single
// write path — see doc.go.
type Notification struct {
	// ID uniquely identifies this notification.
	ID uuid.UUID `json:"id"`

	// TenantID is the tenant this notification belongs to. Every
	// Repository method is scoped to a tenantID and refuses
	// cross-tenant access, mirroring packages/caseversioning exactly.
	TenantID uuid.UUID `json:"tenant_id"`

	// RecipientID is the identity.User this notification is addressed
	// to. Required.
	RecipientID uuid.UUID `json:"recipient_id"`

	// Kind classifies this notification. Required, one of the Kind
	// constants.
	Kind Kind `json:"kind"`

	// Title is a short, human-readable summary line (e.g. "Case #123
	// awaiting your sign-off").
	Title string `json:"title"`

	// Body is a longer free-form description, may be empty.
	Body string `json:"body,omitempty"`

	// CaseID identifies the case this notification relates to, when
	// applicable. Nil for notifications not tied to a single case.
	CaseID *uuid.UUID `json:"case_id,omitempty"`

	// RelatedEntityID identifies the upstream entity that triggered
	// this notification (e.g. an annotations.Annotation ID, a
	// signoff decision ID, a reasoningeval regression run ID), when
	// one is derivable. Optional.
	RelatedEntityID *uuid.UUID `json:"related_entity_id,omitempty"`

	// CreatedAt is when this notification was recorded.
	CreatedAt time.Time `json:"created_at"`

	// ReadAt is set the first time MarkRead/MarkAllRead marks this
	// notification read; nil while unread.
	ReadAt *time.Time `json:"read_at,omitempty"`
}

// Validate checks that n has every field required to be persisted: a
// non-nil TenantID and RecipientID, and a valid Kind.
func (n *Notification) Validate() error {
	if n == nil {
		return ErrNilNotification
	}
	if n.TenantID == uuid.Nil {
		return ErrEmptyTenantID
	}
	if n.RecipientID == uuid.Nil {
		return ErrEmptyRecipientID
	}
	if !n.Kind.IsValid() {
		return ErrInvalidKind
	}
	return nil
}

// IsRead reports whether n has been marked read.
func (n *Notification) IsRead() bool {
	return n != nil && n.ReadAt != nil
}

// Preference is a per-user, per-Kind delivery setting: whether the
// user wants to receive notifications of that Kind at all (Enabled),
// and which additional Channel values (beyond the always-on in-app
// delivery) to push them through.
type Preference struct {
	// TenantID is the tenant this preference belongs to.
	TenantID uuid.UUID `json:"tenant_id"`

	// UserID identifies the user this preference belongs to.
	UserID uuid.UUID `json:"user_id"`

	// Kind is the notification Kind this preference governs.
	Kind Kind `json:"kind"`

	// Enabled controls whether Notify persists/delivers notifications
	// of this Kind for this user at all. Defaults to true (opt-out,
	// not opt-in) when no explicit Preference row exists — see
	// Service.isEnabled.
	Enabled bool `json:"enabled"`

	// Channels lists the additional delivery channels (beyond in-app)
	// this Kind should be pushed through for this user.
	Channels []Channel `json:"channels,omitempty"`
}

// Validate checks that p has every field required to be persisted.
func (p *Preference) Validate() error {
	if p == nil {
		return ErrNilPreference
	}
	if p.TenantID == uuid.Nil {
		return ErrEmptyTenantID
	}
	if p.UserID == uuid.Nil {
		return ErrEmptyRecipientID
	}
	if !p.Kind.IsValid() {
		return ErrInvalidKind
	}
	for _, c := range p.Channels {
		if !c.IsValid() {
			return ErrInvalidChannel
		}
	}
	return nil
}
