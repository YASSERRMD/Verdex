package auditlog

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Store is the durable, tamper-evident, queryable audit sink Phase 077
// exists to provide (see doc.go). It wraps a Repository and is
// responsible for the parts of the tamper-evidence guarantee that must
// happen exactly once, at write time, regardless of which Repository
// implementation is behind it: assigning IDs/timestamps and computing
// PrevHash/ChainHash by reading the tenant's current chain tail before
// every append.
type Store struct {
	repo  Repository
	clock func() time.Time
}

// NewStore builds a Store backed by repo. Returns ErrNilRepository if
// repo is nil.
func NewStore(repo Repository) (*Store, error) {
	if repo == nil {
		return nil, ErrNilRepository
	}
	return &Store{repo: repo, clock: time.Now}, nil
}

func (s *Store) now() time.Time {
	if s.clock != nil {
		return s.clock().UTC()
	}
	return time.Now().UTC()
}

// Append validates event, stamps ID/Time if unset, links it to the
// tenant's current chain tail (PrevHash/ChainHash), and persists it via
// the underlying Repository. Append is the only write path this
// package exposes — there is deliberately no Update or Delete method
// anywhere in Store's API.
func (s *Store) Append(ctx context.Context, event Event) (Event, error) {
	if event.TenantID == uuid.Nil {
		return Event{}, wrapf("Append", ErrEmptyTenantID)
	}
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	if event.Time.IsZero() {
		event.Time = s.now()
	} else {
		event.Time = event.Time.UTC()
	}
	if event.Kind == "" {
		event.Kind = KindSystem
	}
	if err := event.Validate(); err != nil {
		return Event{}, wrapf("Append", err)
	}

	last, err := s.repo.Last(ctx, event.TenantID)
	if err != nil {
		return Event{}, wrapf("Append", err)
	}
	prevHash := ""
	if last != nil {
		prevHash = last.ChainHash
	}
	event.PrevHash = prevHash
	event.ChainHash = ComputeChainHash(prevHash, event)

	if err := s.repo.Append(ctx, event.TenantID, &event); err != nil {
		return Event{}, wrapf("Append", err)
	}
	return event, nil
}

// VerifyTenantChain recomputes and checks the full hash chain for
// tenantID, requiring identity.PermAuditRead (task 8). It returns
// valid=false and the index/error from VerifyChain if any event's
// linkage has been tampered with.
func (s *Store) VerifyTenantChain(ctx context.Context, tenantID uuid.UUID) (valid bool, brokenAt int, err error) {
	if _, err := authorizeAuditRead(ctx); err != nil {
		return false, -1, err
	}
	events, err := s.repo.ListAll(ctx, tenantID)
	if err != nil {
		return false, -1, wrapf("VerifyTenantChain", err)
	}
	return VerifyChain(events)
}
