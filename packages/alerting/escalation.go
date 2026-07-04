package alerting

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Responder is a single named on-call target an EscalationTier pages:
// a person, a schedule, or a team alias (e.g. "oncall-primary",
// "platform-team", "judge-liaison-schedule"). A free-form label, not
// an identity.User reference -- an on-call schedule commonly resolves
// to different concrete users at different times, and this package
// does not itself own schedule rotation, only escalation-tier
// sequencing.
type Responder struct {
	// Name identifies this responder/schedule/team.
	Name string `json:"name"`

	// RecipientUserIDs optionally names the identity.User(s) this
	// Responder currently resolves to, for callers that want to hand
	// off directly to packages/notifications.Service.Notify without a
	// separate schedule-resolution step. May be empty when a caller's
	// own on-call scheduling system resolves Name to users
	// out-of-band.
	RecipientUserIDs []uuid.UUID `json:"recipient_user_ids,omitempty"`
}

// Validate checks r for structural well-formedness.
func (r Responder) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return wrapf("Responder.Validate", ErrInvalidPolicy)
	}
	return nil
}

// EscalationTier is one ordered rung of an EscalationPolicy: a
// Responder to page, and how long to wait for an acknowledgment
// before Route considers escalating to the next tier.
type EscalationTier struct {
	// Responder is who this tier pages.
	Responder Responder `json:"responder"`

	// DelayBeforeNext is how long an alert must remain open at this
	// tier before Route escalates it to the next tier. Ignored on the
	// final tier (there is nothing further to escalate to).
	DelayBeforeNext time.Duration `json:"delay_before_next"`
}

// Validate checks t for structural well-formedness.
func (t EscalationTier) Validate() error {
	if err := t.Responder.Validate(); err != nil {
		return err
	}
	if t.DelayBeforeNext < 0 {
		return wrapf("EscalationTier.Validate", ErrInvalidPolicy)
	}
	return nil
}

// EscalationPolicy is an ordered on-call routing policy (task 6): a
// named sequence of EscalationTiers an AlertEvent walks through until
// acknowledged, each tier waited on for its own DelayBeforeNext before
// moving to the next.
type EscalationPolicy struct {
	// TenantID scopes this policy to a tenant.
	TenantID uuid.UUID `json:"tenant_id"`

	// Name identifies this policy (e.g. "default", "reasoning-quality",
	// "security-incidents"), unique per tenant.
	Name string `json:"name"`

	// MinSeverity is the lowest Severity this policy applies to --
	// callers typically register multiple named policies (e.g. a fast
	// "critical" chain and a slower "warning" chain) and pick the
	// right one for an AlertEvent by inspecting this field alongside
	// Name.
	MinSeverity Severity `json:"min_severity"`

	// Tiers is the ordered escalation chain. Must contain at least one
	// tier.
	Tiers []EscalationTier `json:"tiers"`

	// CreatedBy is the identity.User who set this policy.
	CreatedBy uuid.UUID `json:"created_by"`

	// CreatedAt and UpdatedAt are bookkeeping timestamps.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks p for structural well-formedness: a non-blank Name,
// a valid MinSeverity, and at least one structurally valid Tier.
func (p *EscalationPolicy) Validate() error {
	if p == nil {
		return ErrInvalidPolicy
	}
	if strings.TrimSpace(p.Name) == "" {
		return wrapf("EscalationPolicy.Validate", ErrInvalidPolicy)
	}
	if !p.MinSeverity.IsValid() {
		return wrapf("EscalationPolicy.Validate", ErrInvalidSeverity)
	}
	if len(p.Tiers) == 0 {
		return wrapf("EscalationPolicy.Validate", ErrInvalidPolicy)
	}
	for _, t := range p.Tiers {
		if err := t.Validate(); err != nil {
			return wrapf("EscalationPolicy.Validate", err)
		}
	}
	return nil
}

// AppliesTo reports whether p is eligible to route alert, based on
// alert.Severity meeting p.MinSeverity.
func (p EscalationPolicy) AppliesTo(alert AlertEvent) bool {
	return alert.Severity.AtLeast(p.MinSeverity)
}

// Route walks policy's Tiers in order and returns the Responder that
// should currently own alert, given openSince (when the alert first
// fired) and now (the time to evaluate escalation against) -- real
// time-based escalation logic: tier 0 owns the alert from openSince
// until tier 0's DelayBeforeNext elapses, at which point ownership
// moves to tier 1, and so on, capping at the final tier once its
// cumulative delay has elapsed (an exhausted policy does not wrap
// back to tier 0 or error -- the last tier is expected to eventually
// escalate further via whatever paging system it names, outside this
// package's scope).
//
// Returns ErrNoTiers if policy has no tiers, and a wrapped
// ErrInvalidPolicy if policy otherwise fails Validate.
func Route(alert AlertEvent, policy EscalationPolicy, now time.Time) (Responder, error) {
	if err := policy.Validate(); err != nil {
		return Responder{}, wrapf("Route", err)
	}
	if len(policy.Tiers) == 0 {
		return Responder{}, wrapf("Route", ErrNoTiers)
	}

	elapsed := now.Sub(alert.CreatedAt)
	if elapsed < 0 {
		elapsed = 0
	}

	var cumulative time.Duration
	for i, tier := range policy.Tiers {
		last := i == len(policy.Tiers)-1
		if last {
			return tier.Responder, nil
		}
		cumulative += tier.DelayBeforeNext
		if elapsed < cumulative {
			return tier.Responder, nil
		}
	}
	// Unreachable: the loop above always returns on the last tier.
	return policy.Tiers[len(policy.Tiers)-1].Responder, nil
}

// CurrentTierIndex returns the 0-based index into policy.Tiers that
// Route would currently select for an alert open since openSince, as
// of now. Exposed separately from Route for callers (e.g. a dashboard)
// that want to display "currently at tier N of M" without re-deriving
// the Responder.
func CurrentTierIndex(openSince time.Time, policy EscalationPolicy, now time.Time) (int, error) {
	if err := policy.Validate(); err != nil {
		return 0, wrapf("CurrentTierIndex", err)
	}

	elapsed := now.Sub(openSince)
	if elapsed < 0 {
		elapsed = 0
	}

	var cumulative time.Duration
	for i, tier := range policy.Tiers {
		if i == len(policy.Tiers)-1 {
			return i, nil
		}
		cumulative += tier.DelayBeforeNext
		if elapsed < cumulative {
			return i, nil
		}
	}
	return len(policy.Tiers) - 1, nil
}

// DefaultEscalationPolicy returns a starter three-tier policy suitable
// as a tenant's default: primary on-call, then team lead after 15
// minutes, then the platform team after another 30 minutes -- a
// concrete, usable starting point rather than an empty policy every
// new tenant must configure from scratch before any alert can route
// anywhere.
func DefaultEscalationPolicy(tenantID uuid.UUID) EscalationPolicy {
	return EscalationPolicy{
		TenantID:    tenantID,
		Name:        "default",
		MinSeverity: SeverityWarning,
		Tiers: []EscalationTier{
			{Responder: Responder{Name: "oncall-primary"}, DelayBeforeNext: 15 * time.Minute},
			{Responder: Responder{Name: "oncall-secondary"}, DelayBeforeNext: 30 * time.Minute},
			{Responder: Responder{Name: "platform-team"}},
		},
	}
}

// NotificationRecipientSink is the hand-off point between this
// package's Route decision and actual delivery to a human, mirroring
// packages/reasoningeval.AlertSink and packages/accounting.AlertSink's
// interface shape exactly: this package defines the interface, a
// downstream notifier (e.g. packages/notifications, via an adapter
// mirroring its own ReasoningEvalAlertSink/AccountingAlertSink)
// implements it. This package does not import packages/notifications
// -- see doc.go's "What is explicitly reused, not duplicated" section
// for why.
type NotificationRecipientSink interface {
	// Deliver hands alert off to responder for acknowledgment/action.
	// Implementations should be fast and non-blocking; heavy I/O
	// should be offloaded to a goroutine.
	Deliver(ctx context.Context, alert AlertEvent, responder Responder) error
}

// NoOpNotificationRecipientSink is a NotificationRecipientSink that
// silently discards every hand-off, useful as a default for tests and
// callers that only want Route's decision without any delivery
// side-effect.
type NoOpNotificationRecipientSink struct{}

// Deliver implements NotificationRecipientSink by doing nothing.
func (NoOpNotificationRecipientSink) Deliver(_ context.Context, _ AlertEvent, _ Responder) error {
	return nil
}

// RouteAndDeliver runs Route and, on success, hands the resolved
// Responder off to sink -- the common two-step callers need: decide
// who owns this alert right now, then tell them. A nil sink defaults
// to NoOpNotificationRecipientSink.
func RouteAndDeliver(ctx context.Context, alert AlertEvent, policy EscalationPolicy, now time.Time, sink NotificationRecipientSink) (Responder, error) {
	responder, err := Route(alert, policy, now)
	if err != nil {
		return Responder{}, err
	}
	if sink == nil {
		sink = NoOpNotificationRecipientSink{}
	}
	if err := sink.Deliver(ctx, alert, responder); err != nil {
		return responder, wrapf("RouteAndDeliver", err)
	}
	return responder, nil
}
