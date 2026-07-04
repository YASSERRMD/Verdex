package privacy

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Engine is the privacy / data-subject-rights orchestrator: it
// composes the data inventory, retention policy table, consent
// tracking, subject-access-request workflow, and right-to-erasure
// execution into one set of tenant- and permission-scoped operations,
// recording every SAR transition, erasure, and consent change via
// AuditSink (task 7). Engine mirrors
// packages/accessgovernance.Engine's shape closely: authenticate,
// check tenant match, check permission, mutate, audit regardless of
// outcome.
type Engine struct {
	inventory InventoryRepository
	consent   ConsentRepository
	sars      SARRepository
	erasures  ErasureRepository
	audit     *AuditSink
	clock     func() time.Time

	// retentionMu guards retentionPolicies.
	//
	// retentionPolicies is an in-process registry of RetentionPolicy
	// values keyed by DataCategory, set via SetRetentionPolicy. Unlike
	// the four repositories above, retention policy is small, tenant-
	// independent configuration (a fixed table of category -> window ->
	// action), not a per-tenant record set, so it is held directly on
	// Engine rather than behind its own Repository interface -- there
	// is nothing here that needs Postgres persistence or RLS the way a
	// subject's SAR/erasure/consent history does.
	retentionMu       sync.RWMutex
	retentionPolicies map[DataCategory]RetentionPolicy
}

// NewEngine builds an Engine from its dependencies. inventory,
// consent, sars, and erasures must be non-nil (ErrNilStore); audit may
// be nil (a nil audit sink means SAR/erasure/consent operations simply
// skip audit recording -- useful for lightweight unit tests of the
// decision logic itself, though production callers should always
// supply one).
func NewEngine(inventory InventoryRepository, consent ConsentRepository, sars SARRepository, erasures ErasureRepository, audit *AuditSink) (*Engine, error) {
	if inventory == nil || consent == nil || sars == nil || erasures == nil {
		return nil, ErrNilStore
	}
	return &Engine{
		inventory:         inventory,
		consent:           consent,
		sars:              sars,
		erasures:          erasures,
		audit:             audit,
		clock:             time.Now,
		retentionPolicies: make(map[DataCategory]RetentionPolicy),
	}, nil
}

func (e *Engine) now() time.Time {
	if e.clock != nil {
		return e.clock().UTC()
	}
	return time.Now().UTC()
}

// SetRetentionPolicy registers (or replaces) the RetentionPolicy for
// policy.Category (task 3), requiring managePermission. Retention
// policy is process-wide configuration shared across tenants using
// this Engine instance, mirroring how a single deployed service
// applies one retention schedule per data category regardless of
// tenant.
func (e *Engine) SetRetentionPolicy(ctx context.Context, policy RetentionPolicy) error {
	if _, err := authorizeManage(ctx); err != nil {
		return err
	}
	if err := policy.Validate(); err != nil {
		return err
	}
	e.retentionMu.Lock()
	defer e.retentionMu.Unlock()
	e.retentionPolicies[policy.Category] = policy
	return nil
}

// RetentionPolicyFor returns the currently registered RetentionPolicy
// for category, requiring viewPermission.
func (e *Engine) RetentionPolicyFor(ctx context.Context, category DataCategory) (RetentionPolicy, error) {
	if _, err := authorizeView(ctx); err != nil {
		return RetentionPolicy{}, err
	}
	e.retentionMu.RLock()
	defer e.retentionMu.RUnlock()
	policy, ok := e.retentionPolicies[category]
	if !ok {
		return RetentionPolicy{}, wrapf("RetentionPolicyFor", ErrNoRetentionPolicy)
	}
	return policy, nil
}

// EvaluateRetention resolves the registered RetentionPolicy for
// category and evaluates it against recordCreatedAt via
// EnforceRetention (task 3), requiring viewPermission. Returns
// ErrNoRetentionPolicy if no policy is registered for category.
func (e *Engine) EvaluateRetention(ctx context.Context, category DataCategory, recordCreatedAt time.Time) (DeletionAction, error) {
	policy, err := e.RetentionPolicyFor(ctx, category)
	if err != nil {
		return "", err
	}
	action, err := EnforceRetention(policy, recordCreatedAt, e.now())
	if err != nil {
		return "", wrapf("EvaluateRetention", err)
	}
	return action, nil
}

// RegisterInventoryEntry creates a DataInventoryEntry (task 1),
// requiring managePermission and tenant match. Every registration is
// recorded via AuditSink regardless of outcome.
func (e *Engine) RegisterInventoryEntry(ctx context.Context, tenantID uuid.UUID, entry DataInventoryEntry) (DataInventoryEntry, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordInventoryRegister(ctx, tenantID, actorFromCtx(ctx), entry, err)
		}
		return DataInventoryEntry{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordInventoryRegister(ctx, tenantID, user.ID, entry, err)
		}
		return DataInventoryEntry{}, err
	}

	entry.TenantID = tenantID
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}
	if entry.CreatedBy == uuid.Nil {
		entry.CreatedBy = user.ID
	}
	now := e.now()
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}
	entry.UpdatedAt = now

	if err := entry.Validate(); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordInventoryRegister(ctx, tenantID, user.ID, entry, err)
		}
		return DataInventoryEntry{}, err
	}
	if err := e.inventory.Create(ctx, tenantID, &entry); err != nil {
		wrapped := wrapf("RegisterInventoryEntry", err)
		if e.audit != nil {
			_, _ = e.audit.RecordInventoryRegister(ctx, tenantID, user.ID, entry, wrapped)
		}
		return DataInventoryEntry{}, wrapped
	}

	if e.audit != nil {
		_, _ = e.audit.RecordInventoryRegister(ctx, tenantID, user.ID, entry, nil)
	}
	return entry, nil
}

// ListInventory returns every DataInventoryEntry registered for
// tenantID, requiring viewPermission and tenant match (task 1's
// read-side).
func (e *Engine) ListInventory(ctx context.Context, tenantID uuid.UUID) ([]DataInventoryEntry, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	list, err := e.inventory.List(ctx, tenantID)
	if err != nil {
		return nil, wrapf("ListInventory", err)
	}
	return list, nil
}

// actorFromCtx resolves the actor's user ID from ctx if present,
// falling back to uuid.Nil (which actorFor renders as systemActor)
// when ctx carries no authenticated user -- used by the audit-on-
// failure paths above, which must still record an event even when
// authorizeManage itself failed (e.g. ErrUnauthenticated).
func actorFromCtx(ctx context.Context) uuid.UUID {
	user, err := authorizeActor(ctx)
	if err != nil {
		return uuid.Nil
	}
	return user.ID
}
