package auditlog

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// Purge removes every event for tenantID older than policy's retention
// window (task 6), measured from now. It requires the authenticated
// actor to hold identity.PermAuditRead — the same audit-access gate as
// Query/Export, since Purge is itself a sensitive, auditable operation
// on the audit trail — and returns the number of rows removed.
//
// Purge never mutates a surviving event and never removes an event
// still inside the retention window: it only ever deletes a
// contiguous prefix of the oldest events (repository.PurgeBefore is
// a pure "Time < cutoff" delete). The surviving suffix's hash chain
// remains internally consistent — VerifyChain on the post-purge
// ListAll result still succeeds — because chain links are computed
// forward from each event's immediate predecessor; removing a
// contiguous prefix does not change any surviving event's stored
// PrevHash/ChainHash. It does mean the surviving prefix's first event
// now points at a deleted predecessor, which is why full end-to-end
// verification from genesis (VerifyGenesisChain) is only meaningful
// before any purge, or against a separately retained archival export
// (task 7) taken prior to purging.
//
// Purge itself is recorded as a new KindSystem audit event (via
// Store.Append) once the deletion completes, so the purge action is
// itself part of the durable trail.
func (s *Store) Purge(ctx context.Context, tenantID uuid.UUID, policy RetentionPolicy) (int, error) {
	user, err := authorizeAuditRead(ctx)
	if err != nil {
		return 0, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return 0, err
	}
	if tenantID == uuid.Nil {
		return 0, wrapf("Purge", ErrEmptyTenantID)
	}
	if err := policy.Validate(); err != nil {
		return 0, wrapf("Purge", err)
	}

	cutoff := policy.CutoffBefore(s.now())
	removed, err := s.repo.PurgeBefore(ctx, tenantID, cutoff)
	if err != nil {
		return 0, wrapf("Purge", err)
	}

	if removed > 0 {
		purgeEvent := Event{
			TenantID: tenantID,
			Kind:     KindSystem,
			Detail:   purgeDetail(removed, cutoff),
		}
		purgeEvent.Actor = user.ID.String()
		purgeEvent.Action = "audit.purged"
		purgeEvent.Target = tenantID.String()
		purgeEvent.Outcome = "success"

		if _, appendErr := s.Append(ctx, purgeEvent); appendErr != nil {
			return removed, wrapf("Purge", appendErr)
		}
	}

	return removed, nil
}

func purgeDetail(removed int, cutoff time.Time) string {
	return "purged " + strconv.Itoa(removed) + " events before " + cutoff.UTC().Format(time.RFC3339)
}
