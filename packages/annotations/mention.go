package annotations

import (
	"context"
	"regexp"
	"time"

	"github.com/google/uuid"
)

// mentionPattern matches an "@<uuid>" token in an annotation Body. User
// IDs throughout this repository (see packages/identity.User.ID) are
// uuid.UUID values, so mentions reference users by their canonical
// hyphenated UUID string rather than a display handle — the UI layer
// is responsible for rendering "@<uuid>" as a resolved display name.
var mentionPattern = regexp.MustCompile(`@([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})`)

// ExtractMentions scans body for "@<userID>" tokens and returns the
// distinct set of mentioned user IDs, in first-occurrence order.
// Malformed tokens (not a valid UUID) are ignored rather than causing
// an error, since Body is free-text a user typed.
func ExtractMentions(body string) []uuid.UUID {
	matches := mentionPattern.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[uuid.UUID]struct{}, len(matches))
	out := make([]uuid.UUID, 0, len(matches))
	for _, m := range matches {
		id, err := uuid.Parse(m[1])
		if err != nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// Mention records that an Annotation's Body mentioned a user.
// Repository implementations persist one Mention per "@<userID>"
// token found in a.Body on Create/UpdateBody, queryable via
// Repository.MentionsFor/Service.MentionsFor — a future notification
// pipeline (Phase 072) can read this log without re-parsing every
// annotation's Body. Separately, Service also pushes each Mention
// through MentionSink at write time for real-time delivery; this
// package only emits that event — it does not deliver or batch
// notifications itself.
type Mention struct {
	// AnnotationID identifies the Annotation whose Body contains the
	// mention.
	AnnotationID uuid.UUID `json:"annotation_id"`

	// CaseID identifies the case the annotation belongs to, so a
	// notification consumer can deep-link without a second lookup.
	CaseID uuid.UUID `json:"case_id"`

	// TenantID identifies the tenant the annotation belongs to.
	TenantID uuid.UUID `json:"tenant_id"`

	// AuthorID is the user who wrote the mentioning annotation.
	AuthorID uuid.UUID `json:"author_id"`

	// MentionedUserID is the user referenced by the "@<userID>" token.
	MentionedUserID uuid.UUID `json:"mentioned_user_id"`

	// CreatedAt is when the mentioning annotation was created.
	CreatedAt time.Time `json:"created_at"`
}

// MentionSink receives Mention events for delivery to an external
// system (e.g. an email service, a Slack webhook, or a task queue),
// mirroring packages/signoff.NotificationSink's and
// packages/accounting.AlertSink's idiom exactly. Phase 072 is expected
// to build the full notification pipeline; this package only needs to
// emit the event cleanly.
type MentionSink interface {
	// Notify delivers a Mention. Implementations should be fast and
	// non-blocking; heavy I/O should be offloaded to a goroutine.
	Notify(ctx context.Context, mention Mention) error
}

// NoOpMentionSink is a MentionSink that silently discards every
// mention. It is the default when a Service is constructed without an
// explicit sink.
type NoOpMentionSink struct{}

// Notify implements MentionSink by doing nothing.
func (NoOpMentionSink) Notify(context.Context, Mention) error { return nil }

// MultiMentionSink fans out to multiple MentionSink implementations.
// The first error encountered is returned but all sinks are still
// attempted, mirroring packages/signoff.MultiNotificationSink.
type MultiMentionSink struct {
	Sinks []MentionSink
}

// Notify implements MentionSink by calling Notify on each child sink.
func (m *MultiMentionSink) Notify(ctx context.Context, mention Mention) error {
	var first error
	for _, s := range m.Sinks {
		if err := s.Notify(ctx, mention); err != nil && first == nil {
			first = wrapf("MultiMentionSink.Notify", err)
		}
	}
	return first
}
