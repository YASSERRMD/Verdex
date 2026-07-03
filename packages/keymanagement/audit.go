package keymanagement

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/observability"
)

// AuditAction identifies which key operation an AuditEntry records,
// mirroring how packages/caseversioning.ArtifactKind and
// packages/notifications.Kind use a small closed string enum.
type AuditAction string

const (
	// AuditActionCurrentKey records a CurrentKey call — every
	// encryption operation that resolves "the key to use right now"
	// for a tenant.
	AuditActionCurrentKey AuditAction = "current_key"

	// AuditActionKeyLookup records a Key(ctx, keyID) call resolving a
	// specific (possibly historical) key version.
	AuditActionKeyLookup AuditAction = "key_lookup"

	// AuditActionRotate records a Rotate call.
	AuditActionRotate AuditAction = "rotate"

	// AuditActionRevoke records a Revoke call.
	AuditActionRevoke AuditAction = "revoke"

	// AuditActionViewMetadata records a metadata-only read (ListForTenant/Get)
	// that does not touch key material.
	AuditActionViewMetadata AuditAction = "view_metadata"

	// AuditActionBreakGlassGrant records the creation of a break-glass
	// grant (before it is ever used).
	AuditActionBreakGlassGrant AuditAction = "break_glass_grant"

	// AuditActionBreakGlassUse records a key access performed under an
	// active break-glass grant, outside the normal access-policy path.
	AuditActionBreakGlassUse AuditAction = "break_glass_use"
)

// AuditOutcome records the result of an audited key operation.
type AuditOutcome string

const (
	// AuditOutcomeSuccess means the operation completed as requested.
	AuditOutcomeSuccess AuditOutcome = "success"

	// AuditOutcomeDenied means the operation was rejected by an access
	// policy check (permission, tenant scope, or break-glass
	// expiry/justification).
	AuditOutcomeDenied AuditOutcome = "denied"

	// AuditOutcomeError means the operation failed for a reason other
	// than an access-policy denial (e.g. the backing Provider errored).
	AuditOutcomeError AuditOutcome = "error"
)

// AuditEntry is one immutable, queryable record of a key access,
// satisfying task 7 ("every CurrentKey/Key/Rotate/break-glass call
// recorded, queryable") beyond what
// packages/observability.AuditLogger alone provides (a log sink, not
// a queryable store). AuditRecorder (below) writes both: the
// structured log line via AuditLogger, and this persisted row via
// AuditRepository.
type AuditEntry struct {
	// ID uniquely identifies this audit entry.
	ID uuid.UUID `json:"id"`

	// TenantID is the tenant this entry belongs to.
	TenantID uuid.UUID `json:"tenant_id"`

	// Actor identifies who (or what) performed the action — a user ID
	// (as a string, mirroring observability.AuditEvent.Actor), or
	// "system" for internal callers with no authenticated identity.User
	// on ctx.
	Actor string `json:"actor"`

	// Action classifies what happened. Required, one of the
	// AuditAction constants.
	Action AuditAction `json:"action"`

	// KeyID identifies the key version this action concerned, when
	// applicable (empty for a denied CurrentKey call that never
	// resolved a key, for example).
	KeyID string `json:"key_id,omitempty"`

	// Outcome records the result. Required, one of the AuditOutcome
	// constants.
	Outcome AuditOutcome `json:"outcome"`

	// Justification carries the break-glass justification string for
	// AuditActionBreakGlassGrant/AuditActionBreakGlassUse entries;
	// empty for ordinary operations.
	Justification string `json:"justification,omitempty"`

	// Detail is a short free-form note, e.g. an error message summary
	// or which permission was missing.
	Detail string `json:"detail,omitempty"`

	// OccurredAt is when this action happened.
	OccurredAt time.Time `json:"occurred_at"`
}

// AuditRecorder records a key-access AuditEntry both to a structured
// log sink (via packages/observability.AuditLogger) and to a
// queryable AuditRepository, so key access is never only a log line —
// see doc.go, "Audit".
type AuditRecorder struct {
	logger *observability.AuditLogger
	repo   AuditRepository
}

// NewAuditRecorder builds an AuditRecorder. logger and repo must both
// be non-nil.
func NewAuditRecorder(logger *observability.AuditLogger, repo AuditRepository) (*AuditRecorder, error) {
	if logger == nil {
		return nil, wrapf("NewAuditRecorder", ErrNilProvider)
	}
	if repo == nil {
		return nil, ErrNilRepository
	}
	return &AuditRecorder{logger: logger, repo: repo}, nil
}

// Record writes entry to both the log sink and the repository. A
// zero ID/OccurredAt is filled in. Repository write failures are
// returned to the caller (audit recording is not best-effort for the
// persisted trail, even though the log line is always written first
// so an operator has some signal even if persistence fails).
func (r *AuditRecorder) Record(ctx context.Context, entry AuditEntry) error {
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}
	if entry.OccurredAt.IsZero() {
		entry.OccurredAt = time.Now().UTC()
	}

	r.logger.Log(ctx, observability.AuditEvent{
		Time:    entry.OccurredAt,
		Actor:   entry.Actor,
		Action:  "keymanagement." + string(entry.Action),
		Target:  entry.KeyID,
		Outcome: string(entry.Outcome),
	})

	if err := r.repo.Record(ctx, entry.TenantID, &entry); err != nil {
		return wrapf("AuditRecorder.Record", err)
	}
	return nil
}

// actorLabel returns a stable string identifying ctx's authenticated
// actor for audit purposes, or "system" when none is present —
// audited operations always name an actor, never leave the field
// blank.
func actorLabel(actor uuid.UUID, ok bool) string {
	if !ok || actor == uuid.Nil {
		return "system"
	}
	return actor.String()
}
